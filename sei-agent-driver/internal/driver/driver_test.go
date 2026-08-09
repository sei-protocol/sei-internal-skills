package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"
)

// driverCreateReq is the subset of a session-create body this file asserts
// on.
type driverCreateReq struct {
	AgentID  string            `json:"agent_id"`
	HostType string            `json:"host_type"`
	Title    string            `json:"title"`
	Labels   map[string]string `json:"labels"`
}

// driverEventReq is the subset of a POST .../events body this file asserts
// on: a plain user message or an elicitation approval both decode into this
// shape, distinguished by Type.
type driverEventReq struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// driverFakeServerConfig is what one test wants the fake Omnigent server to
// answer. Anything left zero takes the handler's default.
type driverFakeServerConfig struct {
	// AgentPages is served in order, one per call to GET /v1/agents, so a
	// test can prove resolveAgent walks more than one page.
	AgentPages []string

	// CreateResp is the SessionResponse body for POST /v1/sessions.
	CreateResp string

	// StreamFrames is served verbatim, in order, for GET .../stream.
	StreamFrames []string

	// LaterStreamFrames replace StreamFrames from the second subscription on.
	LaterStreamFrames []string

	// SandboxFrames precede StreamFrames. Left empty, a sensible launch pipeline
	// (connecting then ready) is supplied, because that is what the deployed
	// server does for a managed session and a created session's prompt waits for
	// it. Set it to exercise a launch that fails or stalls instead.
	SandboxFrames []string

	// EventResp is the acknowledgement for POST .../events. Defaults to a
	// generic accepted response.
	EventResp string

	// EventResps are served in order before EventResp takes over.
	EventResps []string

	// DeleteStatus is the status DELETE .../{id} answers with. Zero means
	// 200.
	DeleteStatus int

	// TokenResp is the body for POST /oauth/token. Defaults to a valid
	// token payload.
	TokenResp string

	// SessionListResp is the body for GET /v1/sessions, the pre-create search
	// by run-key label. Empty means an empty page, i.e. no prior session, which
	// is what every test wants except the adoption one.
	SessionListResp string

	// ItemsResp is the body for GET /v1/sessions/{id}/items, the paged read that
	// builds the pre-turn response-id set. Empty means an empty page, i.e. a
	// session with no history, which is what most tests want.
	ItemsResp string

	// SessionResps is served in order, one per GET /v1/sessions/{id}, with the
	// last body repeating. The reply read and any adoption read are the same
	// route, so a test that needs them to differ configures both.
	SessionResps []string

	// ApprovalStatus is the status POST .../events answers with for an approval
	// input. Zero means 200. Lets a test fail answering a permission prompt
	// without touching the prompt path.
	ApprovalStatus int
}

// driverFakeServer is an httptest server speaking the wire shapes the SDK
// expects for the routes the driver calls.
//
// Configuration (the driverFakeServerConfig fields, copied into the
// unexported fields below) is written once, in newDriverFakeServer, strictly
// before the serving goroutine httptest.NewServer starts — so those fields
// need no lock: the goroutine-creation inside NewServer is itself a
// synchronization point covering everything written before it. Everything a
// handler mutates per-request (the counters and captured bodies) is guarded
// by mu, because a handler goroutine's writes and this file's assertions
// after Run returns are on different goroutines with no other ordering
// between them.
type driverFakeServer struct {
	approvalStatus int
	sessionList    string
	itemsResp      string
	sessionResps   []string
	listSessHits   atomic.Int64
	getSessHits    atomic.Int64
	streamHits     atomic.Int64

	t   *testing.T
	URL string

	agentPages   []string
	createResp   string
	streamFrames []string
	// sandboxFrames precede streamFrames, mirroring the deployed server: a managed
	// session's launch pipeline announces itself before anything else. A created
	// session's prompt waits for that, so a fake without it is a server the driver
	// can never send to.
	sandboxFrames []string
	// laterStreamFrames replace streamFrames from the second subscription on, so a
	// test can end one stream mid-turn and finish the turn on the next.
	laterStreamFrames []string
	eventResp         string
	// eventResps are served in order before eventResp takes over, so a test can
	// make the server decline to queue a prompt and then accept it.
	eventResps   []string
	eventHits    atomic.Int64
	deleteStatus int
	tokenResp    string

	totalHits atomic.Int64

	mu            sync.Mutex
	agentReqAfter []string
	createReqs    []driverCreateReq
	eventReqs     []driverEventReq
	deleteHits    int
	deletedIDs    []string
	tokenHits     int
}

func newDriverFakeServer(t *testing.T, cfg driverFakeServerConfig) *driverFakeServer {
	t.Helper()

	fs := &driverFakeServer{
		t:                 t,
		agentPages:        cfg.AgentPages,
		createResp:        cfg.CreateResp,
		streamFrames:      cfg.StreamFrames,
		sandboxFrames:     cfg.SandboxFrames,
		laterStreamFrames: cfg.LaterStreamFrames,
		sessionList:       cfg.SessionListResp,
		itemsResp:         cfg.ItemsResp,
		sessionResps:      cfg.SessionResps,
		approvalStatus:    cfg.ApprovalStatus,
		eventResp:         cfg.EventResp,
		eventResps:        cfg.EventResps,
		deleteStatus:      cfg.DeleteStatus,
		tokenResp:         cfg.TokenResp,
	}
	if fs.sandboxFrames == nil {
		fs.sandboxFrames = driverSandboxReadyFrames()
	}
	if fs.eventResp == "" {
		fs.eventResp = `{"queued":true,"item_id":"item_1"}`
	}
	if fs.tokenResp == "" {
		fs.tokenResp = `{"access_token":"minted-token","token_type":"Bearer","expires_in":1800}`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/agents", fs.handleListAgents)
	mux.HandleFunc("GET /v1/sessions", fs.handleListSessions)
	mux.HandleFunc("GET /v1/sessions/{id}", fs.handleGetSession)
	mux.HandleFunc("GET /v1/sessions/{id}/items", fs.handleListItems)
	mux.HandleFunc("POST /v1/sessions", fs.handleCreateSession)
	mux.HandleFunc("GET /v1/sessions/{id}/stream", fs.handleStream)
	mux.HandleFunc("POST /v1/sessions/{id}/events", fs.handleEvents)
	mux.HandleFunc("DELETE /v1/sessions/{id}", fs.handleDelete)
	mux.HandleFunc("POST /oauth/token", fs.handleToken)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.totalHits.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	fs.URL = srv.URL
	return fs
}

func (fs *driverFakeServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	idx := len(fs.agentReqAfter)
	fs.agentReqAfter = append(fs.agentReqAfter, r.URL.Query().Get("after"))
	fs.mu.Unlock()

	if idx >= len(fs.agentPages) {
		fs.t.Errorf("GET /v1/agents call #%d has no configured page (only %d configured)",
			idx+1, len(fs.agentPages))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, fs.agentPages[idx])
}

// handleListSessions answers the pre-create search for a session already
// carrying this run key. An empty configuration means no prior session.
func (fs *driverFakeServer) handleListSessions(w http.ResponseWriter, _ *http.Request) {
	fs.listSessHits.Add(1)
	body := fs.sessionList
	if body == "" {
		body = `{"data":[],"has_more":false}`
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

// handleGetSession answers the snapshot reads: the one adoption performs after
// the label search matches, and the one that reads the turn's reply. Both are
// this route, so a test needing them to differ configures a sequence and the last
// body repeats.
func (fs *driverFakeServer) handleGetSession(w http.ResponseWriter, r *http.Request) {
	idx := int(fs.getSessHits.Add(1)) - 1
	body := driverSessionResp(r.PathValue("id"), "ag_1")
	if len(fs.sessionResps) > 0 {
		body = fs.sessionResps[min(idx, len(fs.sessionResps)-1)]
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

// handleListItems answers the paged item read the driver uses to learn which
// response ids predate its turn. Deliberately a separate route from the session
// snapshot, which caps at the newest 100 items and says nothing about it.
func (fs *driverFakeServer) handleListItems(w http.ResponseWriter, _ *http.Request) {
	body := fs.itemsResp
	if body == "" {
		body = `{"data":[],"has_more":false}`
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

// driverItemsPage renders a page of the flat item shape that route returns,
// carrying only the response ids the driver reads off it.
func driverItemsPage(responseIDs ...string) string {
	items := make([]string, 0, len(responseIDs))
	for i, id := range responseIDs {
		items = append(items, fmt.Sprintf(`{"id":"prior_%d","response_id":%q}`, i, id))
	}
	return fmt.Sprintf(`{"data":[%s],"has_more":false}`, strings.Join(items, ","))
}

// ListSessionHits is how many times the pre-create search ran.
func (fs *driverFakeServer) ListSessionHits() int { return int(fs.listSessHits.Load()) }

func (fs *driverFakeServer) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req driverCreateReq
	if err := json.Unmarshal(body, &req); err != nil {
		fs.t.Errorf("decode create-session body: %v", err)
	}
	fs.mu.Lock()
	fs.createReqs = append(fs.createReqs, req)
	fs.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, fs.createResp)
}

// StreamHits is how many times the driver subscribed, which is what proves a
// stream lost before the prompt went in was re-established rather than abandoned.
func (fs *driverFakeServer) StreamHits() int64 { return fs.streamHits.Load() }

func (fs *driverFakeServer) handleStream(w http.ResponseWriter, r *http.Request) {
	fs.streamHits.Add(1)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	ctrl := http.NewResponseController(w)
	body := fs.streamFrames
	if fs.streamHits.Load() > 1 && fs.laterStreamFrames != nil {
		body = fs.laterStreamFrames
	}
	for _, frame := range append(fs.sandboxFrames, body...) {
		if _, err := io.WriteString(w, frame); err != nil {
			return
		}
		_ = ctrl.Flush()
	}
}

func (fs *driverFakeServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req driverEventReq
	if err := json.Unmarshal(body, &req); err != nil {
		fs.t.Errorf("decode events body: %v", err)
	}
	fs.mu.Lock()
	fs.eventReqs = append(fs.eventReqs, req)
	fs.mu.Unlock()

	if req.Type == "approval" && fs.approvalStatus != 0 {
		w.WriteHeader(fs.approvalStatus)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if n := int(fs.eventHits.Add(1)) - 1; n < len(fs.eventResps) {
		_, _ = io.WriteString(w, fs.eventResps[n])
		return
	}
	_, _ = io.WriteString(w, fs.eventResp)
}

func (fs *driverFakeServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.deleteHits++
	fs.deletedIDs = append(fs.deletedIDs, r.PathValue("id"))
	fs.mu.Unlock()

	if fs.deleteStatus != 0 && fs.deleteStatus != http.StatusOK {
		w.WriteHeader(fs.deleteStatus)
		_, _ = io.WriteString(w, `{"detail":"delete failed"}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"id":%q,"deleted":true}`, r.PathValue("id"))
}

func (fs *driverFakeServer) handleToken(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.tokenHits++
	fs.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, fs.tokenResp)
}

func (fs *driverFakeServer) AgentReqAfter() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.agentReqAfter...)
}

func (fs *driverFakeServer) CreateReqs() []driverCreateReq {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]driverCreateReq(nil), fs.createReqs...)
}

func (fs *driverFakeServer) EventReqs() []driverEventReq {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]driverEventReq(nil), fs.eventReqs...)
}

func (fs *driverFakeServer) DeleteHits() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.deleteHits
}

func (fs *driverFakeServer) DeletedIDs() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.deletedIDs...)
}

func (fs *driverFakeServer) TokenHits() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.tokenHits
}

func (fs *driverFakeServer) TotalHits() int64 { return fs.totalHits.Load() }

// driverFrame renders one SSE frame the way the server does: an event name
// line, a single-line JSON data line, then the blank line that ends the
// frame. Mirrors the SDK's own stream_test.go helper.
func driverFrame(name, payload string) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, payload)
}

func driverDoneFrame() string { return "data: [DONE]\n\n" }

// driverResponsePayload renders the "response" object every response.* event
// carries, with the fields ResponseObject requires.
func driverResponsePayload(eventType, status string) string {
	return fmt.Sprintf(`{"type":%q,"response":{"id":"resp_1","created_at":1,"model":"m","status":%q}}`,
		eventType, status)
}

func driverAckFrame() string {
	return driverFrame("session.heartbeat", `{"type":"session.heartbeat"}`)
}

func driverCreatedFrame() string {
	return driverFrame("response.created", driverResponsePayload("response.created", "in_progress"))
}

func driverCompletedFrame() string {
	return driverFrame("response.completed", driverResponsePayload("response.completed", "completed"))
}

func driverFailedFrame() string {
	return driverFrame("response.failed", driverResponsePayload("response.failed", "failed"))
}

func driverDeltaFrame(text string) string {
	encoded, _ := json.Marshal(text)
	return driverFrame("response.output_text.delta",
		fmt.Sprintf(`{"type":"response.output_text.delta","delta":%s}`, encoded))
}

func driverElicitationFrame(id, policyName string) string {
	payload := fmt.Sprintf(
		`{"type":"response.elicitation_request","elicitation_id":%q,`+
			`"params":{"message":"approve?","policy_name":%q}}`,
		id, policyName)
	return driverFrame("response.elicitation_request", payload)
}

// driverSandboxReadyFrames is the launch pipeline a managed session announces
// before it can accept a prompt, as recorded from the deployed server.
func driverSandboxReadyFrames() []string {
	return []string{
		driverSandboxFrame("connecting", ""),
		driverSandboxFrame("ready", ""),
	}
}

// driverSandboxFrame is one launch-stage edge. A failed stage carries the reason.
func driverSandboxFrame(stage, errText string) string {
	payload := fmt.Sprintf(`{"type":"session.sandbox_status","stage":%q`, stage)
	if errText != "" {
		payload += fmt.Sprintf(`,"error":%q`, errText)
	}
	return driverFrame("session.sandbox_status", payload+"}")
}

// driverConsumedFrame echoes a queued input back. This is the boundary: the
// server confirming it persisted our own prompt, after which events can be this
// turn's and before which nothing can.
func driverConsumedFrame(itemID string) string {
	return driverFrame("session.input.consumed", fmt.Sprintf(
		`{"type":"session.input.consumed","data":{"item_id":%q,"type":"message","data":{}}}`,
		itemID))
}

// driverPendingConsumedFrame echoes a prompt that was parked as a pending input:
// the item is created now, and the event names the pending entry it drained.
func driverPendingConsumedFrame(itemID, pendingID string) string {
	return driverFrame("session.input.consumed", fmt.Sprintf(
		`{"type":"session.input.consumed","data":{"item_id":%q,"cleared_pending_id":%q,`+
			`"type":"message","data":{}}}`, itemID, pendingID))
}

// driverIdleFrame is a status edge reporting idle and carrying a response id, the
// one signal that ends a turn.
func driverIdleFrame(responseID string) string {
	return driverStatusFrame("idle", fmt.Sprintf(`,"response_id":%q`, responseID))
}

// driverBareIdleFrame is an idle edge with no response id: the pane churn that
// must never end a turn.
func driverBareIdleFrame() string { return driverStatusFrame("idle", "") }

// driverRunningFrame is a mid-turn edge that carries a response id without ending
// anything.
func driverRunningFrame(responseID string) string {
	return driverStatusFrame("running", fmt.Sprintf(`,"response_id":%q`, responseID))
}

// driverSessionFailedFrame is the server reporting a failed turn. This event, not
// the response lifecycle, is what carries a reason.
func driverSessionFailedFrame(code, message string) string {
	return driverStatusFrame("failed",
		fmt.Sprintf(`,"error":{"code":%q,"message":%q}`, code, message))
}

func driverStatusFrame(status, extra string) string {
	return driverFrame("session.status", fmt.Sprintf(
		`{"type":"session.status","conversation_id":"conv_1","status":%q%s}`, status, extra))
}

// driverReplyItem renders an assistant message stamped with a turn's response id,
// which is what attribution reads.
func driverReplyItem(itemID, responseID, text string) string {
	encoded, _ := json.Marshal(text)
	return fmt.Sprintf(
		`{"id":%q,"response_id":%q,"type":"message","status":"completed","created_at":2,`+
			`"data":{"role":"assistant","content":[{"type":"output_text","text":%s}]}}`,
		itemID, responseID, encoded)
}

// driverSessionWithItems renders a snapshot carrying the given items, in order,
// oldest first.
func driverSessionWithItems(id, agentID string, items ...string) string {
	return fmt.Sprintf(`{"id":%q,"agent_id":%q,"created_at":1,"status":"idle","items":[%s]}`,
		id, agentID, strings.Join(items, ","))
}

// driverVerdict is a reply body carrying the closing block the prompts ask for.
func driverVerdict(prose, decision string) string {
	return fmt.Sprintf("%s\n\n```json\n{\"decision\": %q}\n```", prose, decision)
}

func driverSessionResp(id, agentID string) string {
	return fmt.Sprintf(`{"id":%q,"agent_id":%q,"created_at":1,"status":"idle","items":[]}`, id, agentID)
}

// driverLivenessResp is a session snapshot carrying the two fields adoption reads.
// The list item cannot answer this on its own: it has runner_online but no
// host_resumable, so the full read is what decides.
func driverLivenessResp(id, agentID string, runnerOnline, hostResumable bool) string {
	return fmt.Sprintf(
		`{"id":%q,"agent_id":%q,"created_at":1,"status":"idle","items":[],`+
			`"runner_online":%t,"host_resumable":%t}`,
		id, agentID, runnerOnline, hostResumable)
}

// driverRunningSessionResp names a turn in flight, which is how a reconnecting
// client is meant to tell "still working" from "finished while you were away".
func driverRunningSessionResp(id, agentID, responseID string, items ...string) string {
	return fmt.Sprintf(
		`{"id":%q,"agent_id":%q,"created_at":1,"status":"running","items":[%s],`+
			`"active_response_id":%q}`, id, agentID, strings.Join(items, ","), responseID)
}

func driverAgentPage(id, name, lastID string, hasMore bool) string {
	return fmt.Sprintf(`{"data":[{"id":%q,"name":%q,"created_at":1}],"last_id":%q,"has_more":%t}`,
		id, name, lastID, hasMore)
}

func driverTestConfig(t *testing.T, baseURL string) Config {
	t.Helper()
	return Config{
		BaseURL:           baseURL,
		Origin:            "test-origin",
		Agent:             "sei-droid",
		Token:             "test-token",
		RunDeadline:       10 * time.Second,
		RequestTimeout:    5 * time.Second,
		StreamIdleTimeout: 5 * time.Second,
	}
}

// driverPrompts drops stop_session so a test can count prompts and mean it. A
// review posts none any more, so this filters nothing today; it stays because a
// test asserting "how many prompts" should not silently start counting one.
func driverPrompts(reqs []driverEventReq) []driverEventReq {
	out := make([]driverEventReq, 0, len(reqs))
	for _, r := range reqs {
		if r.Type != omnigent.InputTypeStopSession {
			out = append(out, r)
		}
	}
	return out
}

// driverStops counts stop_session inputs. A review must post none: stopping a
// session frees no compute and costs the next invocation a fresh runner, so
// the session is left running and DeleteSessionForPR reclaims it on close.
func driverStops(reqs []driverEventReq) int {
	n := 0
	for _, r := range reqs {
		if r.Type == omnigent.InputTypeStopSession {
			n++
		}
	}
	return n
}

func driverTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// driverLogSink collects what the driver logged. Guarded because the driver logs
// from the stream's goroutine as well as the caller's.
type driverLogSink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *driverLogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *driverLogSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// driverCapturingLogger returns a logger and the sink holding what it wrote, for
// the assertions about what a failed run leaves behind for an operator.
func driverCapturingLogger() (*slog.Logger, *driverLogSink) {
	sink := &driverLogSink{}
	return slog.New(slog.NewTextHandler(sink, nil)), sink
}

// driverPromptText pulls the text back out of a UserMessage's wire shape:
// data.content[0].text.
func driverPromptText(t *testing.T, data map[string]any) string {
	t.Helper()
	content, ok := data["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("event data has no content: %#v", data)
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] is not an object: %#v", content[0])
	}
	text, _ := part["text"].(string)
	return text
}

// TestDriverRunHappyPath drives a whole successful review turn against a fake
// server and checks every stop along the way: the agent is resolved by name
// across two pages, the session is created carrying the run-key label, the prompt
// is sent once from the subscription hook, the status edge carrying a response id
// ends the turn, the reply is read off the session and attributed to that id, and
// the session is released rather than deleted.
func TestDriverRunHappyPath(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Looks good.", "approve")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{
			driverAgentPage("ag_other", "other-agent", "ag_other", true),
			driverAgentPage("ag_1", "sei-droid", "ag_1", false),
		},
		CreateResp: driverSessionResp("conv_1", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverRunningFrame("resp_claude_a"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 42, Trigger: "trigger-happy"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK (%d)", result.ExitCode, ExitOK)
	}
	if result.SessionID != "conv_1" {
		t.Errorf("SessionID = %q, want conv_1", result.SessionID)
	}
	if !result.TeardownOK {
		t.Error("TeardownOK = false, want true")
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil, want a verdict")
	}
	if result.Reply.Text != reply {
		t.Errorf("Reply.Text = %q, want %q", result.Reply.Text, reply)
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("reply does not carry decision approve: %q", result.Reply.Text)
	}
	if result.Reply.TurnID != "resp_claude_a" {
		t.Errorf("Reply.TurnID = %q, want resp_claude_a: the comment names its own provenance",
			result.Reply.TurnID)
	}
	if result.Reply.ItemID != "item_reply" {
		t.Errorf("Reply.ItemID = %q, want item_reply", result.Reply.ItemID)
	}

	if got := fs.AgentReqAfter(); len(got) != 2 || got[0] != "" || got[1] != "ag_other" {
		t.Errorf("agent list calls' after= %v, want [\"\" \"ag_other\"]: resolveAgent must page", got)
	}

	creates := fs.CreateReqs()
	if len(creates) != 1 {
		t.Fatalf("create-session calls = %d, want 1", len(creates))
	}
	if creates[0].AgentID != "ag_1" {
		t.Errorf("create AgentID = %q, want ag_1", creates[0].AgentID)
	}
	wantRunKey := testRunKey(req.Repo, req.PR)
	if got := creates[0].Labels[RunKeyLabel]; got != wantRunKey {
		t.Errorf("create Labels[%s] = %q, want %q", RunKeyLabel, got, wantRunKey)
	}

	events := driverPrompts(fs.EventReqs())
	if len(events) != 1 {
		t.Fatalf("prompt posts = %d, want exactly 1 (the prompt, sent once)", len(events))
	}
	if events[0].Type != "message" {
		t.Errorf("event[0].Type = %q, want message", events[0].Type)
	}
	// The driver sends what the workload asked for, verbatim. What that text
	// should say is the workload's to test.
	if got := driverPromptText(t, events[0].Data); got != req.Prompt() {
		t.Errorf("prompt sent = %q, want the workload's prompt %q", got, req.Prompt())
	}

	// The session is no longer deleted on a normal run: the conversation is the
	// agent's memory of this pull request and the next invocation builds on it.
	// It is released with stop_session and destroyed only when the PR closes.
	if got := fs.DeleteHits(); got != 0 {
		t.Errorf("DELETE calls = %d, want 0 — a clean run must keep the conversation", got)
	}
	if got := driverStops(fs.EventReqs()); got != 0 {
		t.Errorf("stop_session posts = %d, want 0: a review leaves the session running", got)
	}
}

// TestDriverIgnoresTheInjectionAcknowledgement is the regression test for the
// defect that made every recorded review end before the agent had done anything.
//
// On this harness the response lifecycle's completed event is an acknowledgement
// that the prompt was injected into the terminal, not a report that the turn
// finished. It arrives before the prompt has even been persisted — measured at
// 1.7, 3.1 and 6.0 seconds against boundaries at 7.2, 6.9 and 8.3 — so a driver
// that treats it as a turn end reads no reply and reports no verdict on a review
// that was about to succeed. Both a completed and a failed lifecycle event sit
// before the boundary here and neither may have any effect.
func TestDriverIgnoresTheInjectionAcknowledgement(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Read the diff.", "comment")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_ack", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverCreatedFrame(),
			driverCompletedFrame(),
			driverFailedFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_ack", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 8, Trigger: "trigger-ack"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitOK {
		t.Fatalf("ExitCode = %d, want ExitOK: a response lifecycle event must not end a turn",
			result.ExitCode)
	}
	if !carriesDecision(result.Reply, "comment") {
		t.Fatalf("Verdict = %+v, want the decision the turn actually produced", result.Reply)
	}
}

// TestDriverIgnoresBareIdleEdges checks that an idle edge carrying no response id
// does not end a turn.
//
// Those edges are terminal churn rather than progress. One recorded trace carries
// five of them, one arriving 24 seconds into work that ran for 38, so "the first
// idle edge ends the turn" would cut that review off mid-tool-call.
func TestDriverIgnoresBareIdleEdges(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Two findings.", "request_changes")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_idle", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverBareIdleFrame(),
			driverConsumedFrame("item_1"),
			driverBareIdleFrame(),
			driverRunningFrame("resp_claude_a"),
			driverBareIdleFrame(),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_idle", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", reply)),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 9, Trigger: "trigger-idle"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !carriesDecision(result.Reply, "request_changes") {
		t.Fatalf("Verdict = %+v, want request_changes: a bare idle edge must not end the turn",
			result.Reply)
	}
}

// TestDriverIgnoresATurnThatEndedBeforeItsOwnPrompt checks the boundary.
//
// The stream opens with a prologue replaying earlier work, so an id-bearing idle
// edge can arrive before the server has confirmed our own prompt. Taking it would
// attribute the reply of a previous invocation — on a recorded trace, one that
// arrived 31 milliseconds before our prompt was persisted.
func TestDriverIgnoresATurnThatEndedBeforeItsOwnPrompt(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionWithItems("conv_bound", "ag_1",
			driverReplyItem("item_old", "resp_claude_old",
				driverVerdict("An earlier invocation said this.", "request_changes"))),
		// The pre-turn response ids now come from the paged item route, which is
		// the only read that sees past the snapshot's newest-100 window.
		ItemsResp: driverItemsPage("resp_claude_old"),
		StreamFrames: []string{
			driverAckFrame(),
			driverIdleFrame("resp_claude_old"),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_bound", "ag_1",
				driverReplyItem("item_old", "resp_claude_old",
					driverVerdict("An earlier invocation said this.", "request_changes")),
				driverReplyItem("item_new", "resp_claude_a",
					driverVerdict("This invocation says this.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 10, Trigger: "trigger-boundary"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if result.Reply.TurnID != "resp_claude_a" {
		t.Errorf("Reply.TurnID = %q, want resp_claude_a: an idle edge before the boundary "+
			"belongs to another turn", result.Reply.TurnID)
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("reply does not carry decision approve; the earlier invocation's "+
			"verdict was published as this one's: %q", result.Reply.Text)
	}
}

// TestDriverAttributesByTurnIDRatherThanRecency puts a foreign reply last in the
// snapshot, where "newest assistant message wins" would find it.
//
// Recency was the rule this replaced, filtered by a set of item ids captured
// before the turn. That filter is negative, so it fails open: an empty or stale
// set publishes whatever is newest, which is how another invocation's verdict once
// went out at exit 0.
func TestDriverAttributesByTurnIDRatherThanRecency(t *testing.T) {
	t.Parallel()

	foreign := driverReplyItem("item_foreign", "resp_claude_old",
		driverVerdict("Not this turn's answer.", "request_changes"))
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		// The foreign group is already on the session before the turn, so it is
		// history rather than a second live turn.
		CreateResp: driverSessionWithItems("conv_recency", "ag_1", foreign),
		ItemsResp:  driverItemsPage("resp_claude_old"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_recency", "ag_1",
				driverReplyItem("item_ours", "resp_claude_a",
					driverVerdict("This turn's answer.", "approve")),
				foreign),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 11, Trigger: "trigger-recency"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("reply does not carry decision approve: attribution must read the "+
			"turn id, not the position: %q", result.Reply.Text)
	}
	if result.Reply.ItemID != "item_ours" {
		t.Errorf("Reply.ItemID = %q, want item_ours", result.Reply.ItemID)
	}
}

// TestDriverRefusesWhenTwoTurnsRepliedIntoTheSession checks that an ambiguous
// session is refused rather than resolved by guessing.
//
// One turn commits one group of replies. Two means something else replied into
// this session while ours ran, the likeliest cause being a run cancelled mid-turn
// whose stop lost the race. Nothing on the wire says which group is ours, and
// choosing the newest is exactly the failure this driver has already shipped once.
func TestDriverRefusesWhenTwoTurnsRepliedIntoTheSession(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_two", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_two", "ag_1",
				driverReplyItem("item_a", "resp_claude_a", driverVerdict("Ours.", "approve")),
				driverReplyItem("item_b", "resp_claude_b", driverVerdict("Theirs.", "request_changes"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 12, Trigger: "trigger-two"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitNoVerdict {
		t.Errorf("ExitCode = %d, want ExitNoVerdict (%d): two reply groups must refuse",
			result.ExitCode, ExitNoVerdict)
	}
	// Ambiguous attribution yields no reply at all, not an unusable one: the
	// driver refuses to name a turn's answer rather than guessing between two.
	if result.Reply != nil && result.Reply.Text != "" {
		t.Errorf("Reply = %+v, want none when attribution is ambiguous", result.Reply)
	}
}

// TestDriverFailsWhenAPermissionPromptCannotBeAnswered is the regression test for
// the most expensive fault this driver has produced.
//
// The permission hook blocks the agent synchronously while it waits for an answer,
// so a prompt the driver fails to resolve stalls the review for the rest of the
// run: one recorded trace sat on an unanswered prompt for 9 minutes 39 seconds
// while the transport stayed healthy the whole time. The previous version logged
// the failure and carried on reading a stream that would never produce anything.
func TestDriverFailsWhenAPermissionPromptCannotBeAnswered(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_stuck", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverElicitationFrame("elicit_stuck", "approve_shell"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		ApprovalStatus: http.StatusInternalServerError,
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 13, Trigger: "trigger-stuck"}
	driver := NewDriver(cfg, NewPolicy("approve_shell", ""), driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// ExitTransport, not ExitTurnFailed: the turn did not fail, we failed to answer
	// it. Reporting the agent's outcome for our own transport fault sends an
	// operator looking in the wrong place.
	if result.ExitCode != ExitTransport {
		t.Errorf("ExitCode = %d, want ExitTransport (%d): failing to answer a prompt is the "+
			"driver's fault, not the agent's", result.ExitCode, ExitTransport)
	}
	if !result.TeardownOK {
		t.Error("TeardownOK = false, want true: teardown must still run")
	}
}

// TestDriverTurnFailedLeavesTheSessionRunning checks a failed status edge yields
// ExitTurnFailed, carries no verdict, and — the point of the test — still releases
// the session.
func TestDriverTurnFailedLeavesTheSessionRunning(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_2", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverSessionFailedFrame("server_error", "the harness died"),
			driverDoneFrame(),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 7, Trigger: "trigger-fail"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitTurnFailed {
		t.Errorf("ExitCode = %d, want ExitTurnFailed (%d)", result.ExitCode, ExitTurnFailed)
	}
	if result.Reply != nil {
		t.Errorf("Verdict = %+v, want nil on a failed turn", result.Reply)
	}
	if !result.TeardownOK {
		t.Error("TeardownOK = false, want true: teardown must still run on a failed turn")
	}
	if got := fs.DeleteHits(); got != 0 {
		t.Errorf("DELETE calls = %d, want 0: a failed turn keeps its conversation too", got)
	}
	if got := driverStops(fs.EventReqs()); got != 0 {
		t.Errorf("stop_session posts = %d, want 0 even on a failed turn", got)
	}
}

// TestDriverAnswersElicitationsOncePerIDAndPostsTheApprovalShape checks that a
// duplicated elicitation id is answered only once, and that the approval travels
// with type "approval" and data.elicitation_id / data.action.
func TestDriverAnswersElicitationsOncePerIDAndPostsTheApprovalShape(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_5", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverElicitationFrame("elicit_1", "approve_shell"),
			driverElicitationFrame("elicit_1", "approve_shell"), // duplicate id
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_5", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Ran a command.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 14, Trigger: "trigger-elicit"}
	driver := NewDriver(cfg, NewPolicy("approve_shell", ""), driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitOK {
		t.Fatalf("ExitCode = %d, want ExitOK", result.ExitCode)
	}

	var approvals []driverEventReq
	for _, e := range fs.EventReqs() {
		if e.Type == "approval" {
			approvals = append(approvals, e)
		}
	}
	if len(approvals) != 1 {
		t.Fatalf("approval POSTs = %d, want exactly 1 despite the duplicated elicitation id",
			len(approvals))
	}
	if got := approvals[0].Data["elicitation_id"]; got != "elicit_1" {
		t.Errorf("data.elicitation_id = %v, want elicit_1", got)
	}
	if got := approvals[0].Data["action"]; got != "accept" {
		t.Errorf("data.action = %v, want accept", got)
	}
}

// TestDriverTeardownReusesTheMintedTokenInsteadOfMintingAgain configures machine
// credentials rather than a static token, so the driver mints its own, and checks
// the whole run — including teardown — costs exactly one token exchange.
func TestDriverTeardownReusesTheMintedTokenInsteadOfMintingAgain(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_8", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_8", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", driverVerdict("Fine.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.Token = ""
	cfg.MachineClientID = "machine-id"
	cfg.MachineClientSecret = "machine-secret"

	req := testWork{Repo: "sei-protocol/sandbox", PR: 16, Trigger: "trigger-mint"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitOK || !result.TeardownOK {
		t.Fatalf("result = %+v, want a clean OK run", result)
	}
	if got := fs.TokenHits(); got != 1 {
		t.Errorf("POST /oauth/token calls across the whole run (including teardown) = %d, "+
			"want exactly 1", got)
	}
}

// driverSessionFailedFrameFor is a failed status edge that names the turn it
// describes. The server sends one without a response id for ordinary
// session-level faults, so both shapes are real and they resolve differently.
func driverSessionFailedFrameFor(responseID, code, message string) string {
	return driverStatusFrame("failed", fmt.Sprintf(
		`,"response_id":%q,"error":{"code":%q,"message":%q}`, responseID, code, message))
}

// TestReplyForReadsAFinishedTurnEvenAfterTheClockExpires pins the precedence a
// review of the previous design caught this driver getting wrong.
//
// The reply is committed before the edge that ends the turn, and fetchReply reads
// on a detached context, so a deadline or a SIGTERM landing in the window between
// those two moments must not discard a review that finished. Checking the clock
// ahead of the read reported ExitTimeout on a completed, paid-for review.
func TestReplyForReadsAFinishedTurnEvenAfterTheClockExpires(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		SessionResps: []string{
			driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Finished just before the clock ran out.", "approve"))),
		},
	})

	driver := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger())
	client, err := driver.newClient(context.Background())
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}

	// Already done, standing in for the deadline or the signal landing here.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	verdict, err := driver.replyFor(ctx, client, "conv_1",
		&turn{id: "resp_claude_a", crossed: true}, map[string]bool{})
	if err != nil {
		t.Fatalf("replyFor returned %v, want the verdict: a finished turn outranks the clock", err)
	}
	if !carriesDecision(&verdict, "approve") {
		t.Errorf("reply = %q, want the finished turn's answer: a finished turn "+
			"outranks the clock", verdict.Text)
	}
}

// TestDriverSalvagesAVerdictFromAFailedTurn covers the other half of that
// precedence.
//
// The server publishes a failed edge on any lost transport whatever the turn was
// doing, so a review that finished and then met a network blip lands here with its
// reply already committed. Salvaging requires all three of: the edge naming a
// response id, a reply attributable to it, and that reply carrying a full verdict.
func TestDriverSalvagesAVerdictFromAFailedTurn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		failFrame  string
		reply      string
		wantExit   int
		wantReview bool
	}{
		{
			name:       "a named turn with a complete verdict is recovered",
			failFrame:  driverSessionFailedFrameFor("resp_claude_a", "server_error", "transport lost"),
			reply:      driverVerdict("The review finished before the blip.", "request_changes"),
			wantExit:   ExitOK,
			wantReview: true,
		},
		{
			name:      "an unnamed failure cannot be attributed, so it stays a failure",
			failFrame: driverSessionFailedFrame("server_error", "transport lost"),
			reply:     driverVerdict("Committed, but nothing ties it to this turn.", "approve"),
			wantExit:  ExitTurnFailed,
		},
		{
			name:      "a named turn whose reply has no verdict stays a failure",
			failFrame: driverSessionFailedFrameFor("resp_claude_a", "server_error", "transport lost"),
			reply:     "I was still reading the diff when the connection dropped.",
			wantExit:  ExitTurnFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
				CreateResp: driverSessionResp("conv_salvage", "ag_1"),
				StreamFrames: []string{
					driverAckFrame(),
					driverConsumedFrame("item_1"),
					tc.failFrame,
					driverDoneFrame(),
				},
				SessionResps: []string{
					driverSessionWithItems("conv_salvage", "ag_1",
						driverReplyItem("item_reply", "resp_claude_a", tc.reply)),
				},
			})

			result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
				Run(context.Background(), testWork{
					Repo: "sei-protocol/sandbox", PR: 20, Trigger: "trigger-salvage"})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ExitCode != tc.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tc.wantExit)
			}
			// The driver carries a reply or it does not; whether that reply is a
			// review is the workload's reading, tested there.
			if got := result.Reply != nil && result.Reply.Text != ""; got != tc.wantReview {
				t.Errorf("carried a reply = %v, want %v", got, tc.wantReview)
			}
			// Whatever the outcome, the session is released rather than leaked.
			if got := driverStops(fs.EventReqs()); got != 0 {
				t.Errorf("stop_session posts = %d, want 0", got)
			}
		})
	}
}

// TestDriverRejectsATurnIDThatPredatesItsOwn closes the reachable half of the
// overlapping-run hazard.
//
// A superseded run whose stop lost the race keeps working, and its turn can end
// inside our window. Its idle edge carries a response id and is otherwise
// indistinguishable from ours — position cannot separate them, because both
// arrive after our prompt was echoed. What separates them is that the other run's
// id was already on the session before we sent anything.
func TestDriverRejectsATurnIDThatPredatesItsOwn(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_stale", "ag_1"),
		// The other run's group is already on the session.
		ItemsResp: driverItemsPage("resp_claude_other"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			// Arrives after our boundary and carries an id, so only the pre-turn
			// set can reject it.
			driverIdleFrame("resp_claude_other"),
			driverIdleFrame("resp_claude_ours"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_stale", "ag_1",
				driverReplyItem("item_theirs", "resp_claude_other",
					driverVerdict("The other run's review.", "approve")),
				driverReplyItem("item_ours", "resp_claude_ours",
					driverVerdict("Our review.", "request_changes"))),
		},
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), testWork{
			Repo: "sei-protocol/sandbox", PR: 30, Trigger: "trigger-stale"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if result.Reply.TurnID != "resp_claude_ours" {
		t.Errorf("Reply.TurnID = %q, want resp_claude_ours: an id already on the session "+
			"cannot be the turn answering our prompt", result.Reply.TurnID)
	}
	if !carriesDecision(result.Reply, "request_changes") {
		t.Errorf("reply does not carry decision request_changes: the other run's "+
			"review was published as ours: %q", result.Reply.Text)
	}
}

// TestDriverRecoversAReviewWhoseStreamDied covers the transport-drop path.
//
// The server persists an item before it publishes one, so a review that finished
// just before the connection dropped is already readable. Previously any stream
// error discarded it, and the failed-turn salvage could not help because a
// transport drop produces no server-reported failure edge to key on.
//
// The stream ends without its terminal sentinel, which is what the SDK reports as
// an interrupted stream.
func TestDriverRecoversAReviewWhoseStreamDied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		items      []string
		wantExit   int
		wantReview bool
		// wantLogged is text from the reply that has to reach the logs. A run that
		// ends without publishing leaves them as the only record of what the agent
		// said, and a short reply is the first symptom of a turn that ended early
		// -- so "the stream dropped" and "the agent gave up" have to be tellable
		// apart afterwards. Empty when the reply carried no text to log.
		wantLogged string
	}{
		{
			name: "one new reply with a full verdict is recovered",
			items: []string{driverReplyItem("item_reply", "resp_claude_a",
				driverVerdict("Finished just before the drop.", "approve"))},
			wantExit:   ExitOK,
			wantReview: true,
			wantLogged: "Finished just before the drop.",
		},
		{
			name: "two new replies cannot be told apart, so the drop stands",
			items: []string{
				driverReplyItem("item_a", "resp_claude_a", driverVerdict("Ours?", "approve")),
				driverReplyItem("item_b", "resp_claude_b", driverVerdict("Theirs?", "comment")),
			},
			wantExit: ExitTransport,
		},
		{
			name: "a reply with no verdict is not worth recovering",
			items: []string{driverReplyItem("item_reply", "resp_claude_a",
				"I was still reading the diff when the connection dropped.")},
			wantExit:   ExitTransport,
			wantLogged: "I was still reading the diff when the connection dropped.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
				CreateResp: driverSessionResp("conv_drop", "ag_1"),
				StreamFrames: []string{
					driverAckFrame(),
					driverConsumedFrame("item_1"),
					// No done frame: the stream just stops.
				},
				SessionResps: []string{
					driverSessionWithItems("conv_drop", "ag_1", tc.items...),
				},
			})

			log, sink := driverCapturingLogger()
			result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, log).
				Run(context.Background(), testWork{
					Repo: "sei-protocol/sandbox", PR: 31, Trigger: "trigger-drop"})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.ExitCode != tc.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tc.wantExit)
			}
			if tc.wantLogged != "" && !strings.Contains(sink.String(), tc.wantLogged) {
				t.Errorf("the reply never reached the logs; want %q in:\n%s",
					tc.wantLogged, sink.String())
			}
			if got := result.Reply != nil && result.Reply.Text != ""; got != tc.wantReview {
				t.Errorf("carried a review = %v, want %v", got, tc.wantReview)
			}
			// The session is released whatever happened to the stream.
			if got := driverStops(fs.EventReqs()); got != 0 {
				t.Errorf("stop_session posts = %d, want 0", got)
			}
		})
	}
}

// TestDriverWaitsForTheSandboxBeforeSendingItsPrompt pins the send timing.
//
// A managed create returns before its sandbox exists, and a prompt sent into that
// gap is accepted without being queued — leaving no anchor, so the run refuses
// rather than reviewing blind. The launch pipeline is announced on the stream, so
// the send waits for it.
func TestDriverWaitsForTheSandboxBeforeSendingItsPrompt(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_wait", "ag_1"),
		// The pipeline stalls before ready, which is the whole point: nothing here
		// says the sandbox can accept anything.
		SandboxFrames: []string{
			driverSandboxFrame("provisioning", ""),
			driverSandboxFrame("cloning", ""),
			driverSandboxFrame("connecting", ""),
		},
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 11, Trigger: "trigger-wait"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	if _, err := driver.Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := driverPrompts(fs.EventReqs()); len(got) != 0 {
		t.Errorf("prompt posts = %d, want 0: a sandbox that never reported ready "+
			"cannot have been sent a prompt", len(got))
	}
}

// TestDriverReportsASandboxThatNeverLaunched pins the reason reaching the
// operator. A failed launch carries why — a spend limit, a clone that could not
// authenticate — and that answer is worth more than a run deadline expiring.
func TestDriverReportsASandboxThatNeverLaunched(t *testing.T) {
	t.Parallel()

	const reason = "managed sandbox launch failed: spend limit reached"
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_failed", "ag_1"),
		SandboxFrames: []string{
			driverSandboxFrame("provisioning", ""),
			driverSandboxFrame("failed", reason),
		},
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 12, Trigger: "trigger-failed"}
	driver := NewDriver(cfg, Policy{}, driverTestLogger())

	result, err := driver.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != ExitTurnFailed {
		t.Errorf("ExitCode = %d, want ExitTurnFailed (%d)", result.ExitCode, ExitTurnFailed)
	}
	if got := driverPrompts(fs.EventReqs()); len(got) != 0 {
		t.Errorf("prompt posts = %d, want 0: a failed launch is not sent to", len(got))
	}
}

// TestDriverResubscribesWhileThePromptIsStillWaiting covers the cold start.
//
// A launching sandbox emits almost nothing, and a quiet connection does not
// survive in transit, so the first stream usually ends before the sandbox is
// ready. Waiting longer cannot help — the connection is gone — so the stream is
// re-established for as long as the prompt has not gone in.
func TestDriverResubscribesWhileThePromptIsStillWaiting(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_cold", "ag_1"),
		// The sandbox never gets past connecting, and the stream ends each time.
		SandboxFrames: []string{driverSandboxFrame("connecting", "")},
		StreamFrames:  []string{driverAckFrame(), driverDoneFrame()},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 31, Trigger: "trigger-cold"}

	if _, err := NewDriver(cfg, Policy{}, driverTestLogger()).
		Run(context.Background(), req); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := fs.StreamHits(); got < 2 {
		t.Errorf("stream subscriptions = %d, want more than 1: a stream that ended "+
			"before the prompt went in must be re-established", got)
	}
	if got := driverPrompts(fs.EventReqs()); len(got) != 0 {
		t.Errorf("prompt posts = %d, want 0: the sandbox never reported ready", len(got))
	}
}

// TestDriverAnchorsOnAPendingInput covers a prompt queued before the native
// terminal is up.
//
// The server persists an item straight away when the terminal is running and
// returns its id; when it is still starting, the prompt is parked as a pending
// input and only that id comes back. Both are queued. Demanding an item id treats
// the second as a failure, and the turn it started then runs unattributed.
func TestDriverAnchorsOnAPendingInput(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_pending", "ag_1"),
		EventResp:  `{"queued":true,"pending_id":"pending_1"}`,
		StreamFrames: []string{
			driverAckFrame(),
			// The item id differs from the anchor; the pending id is what matches.
			driverPendingConsumedFrame("item_9", "pending_1"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_pending", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Reviewed after the terminal came up.", "comment"))),
		},
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), testWork{Repo: "sei-protocol/sandbox", PR: 41, Trigger: "t-pending"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil, want a review: a prompt queued as a pending input is " +
			"queued, and its consume event names the pending id it drained")
	}
	if got := len(driverPrompts(fs.EventReqs())); got != 1 {
		t.Errorf("prompt posts = %d, want exactly 1: a queued prompt must never be "+
			"sent again, whichever identifier came back", got)
	}
}

// TestDriverFollowsATurnAcrossStreams covers a connection that expires while the
// agent is still working.
//
// The connection has a lifetime of its own, measured at around three minutes,
// and a review runs longer than that. So a stream ending mid-turn is the expected
// way a long turn's connection dies, not evidence the work stopped. Salvaging
// whatever is committed at that instant takes the agent's opening narration
// instead of its review.
// TestDriverRejoinsAnIdleSessionWhoseReplyIsNotAReview covers the harder half of
// following a turn across streams: the session reports itself IDLE with no active
// response while the agent is still working.
//
// A claude-native session goes idle between tool calls, so neither its status nor
// its absent active response separates a finished turn from one mid-answer. What
// does is the reply itself: the prompt requires a closing verdict block, so a
// reply without one is the agent still writing. Reading the snapshot instead
// published an agent's opening sentence — "I have the diff (1575 lines). Let me
// read it in full." — as its review of a 1,575-line diff.
func TestDriverRejoinsAnIdleSessionWhoseReplyIsNotAReview(t *testing.T) {
	t.Parallel()

	narration := "I have the diff (1575 lines). Let me read it in full."

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_idle", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			// The connection expired mid-turn: no done sentinel.
		},
		LaterStreamFrames: []string{
			driverAckFrame(),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		// A salvage reads the session twice, once to group the replies and once to
		// fetch the one it picked, so the unfinished turn has to answer both.
		SessionResps: []string{
			// Idle, no active response, and the only thing committed is the
			// agent's opening line. The server's own signals say "finished"; the
			// missing verdict block says otherwise, and it is right.
			driverSessionWithItems("conv_idle", "ag_1",
				driverReplyItem("item_narration", "resp_claude_a", narration)),
			driverSessionWithItems("conv_idle", "ag_1",
				driverReplyItem("item_narration", "resp_claude_a", narration)),
			driverSessionWithItems("conv_idle", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("The review, once it finished reading.", "comment"))),
		},
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), testWork{Repo: "sei-protocol/sandbox", PR: 52, Trigger: "t-idle"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fs.StreamHits() < 2 {
		t.Fatalf("stream subscriptions = %d, want more than 1: an idle session whose "+
			"reply is not a review must be rejoined, not published", fs.StreamHits())
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil, want the review the turn went on to write")
	}
	if strings.Contains(result.Reply.Text, narration) {
		t.Errorf("published the agent's opening line as its review: %q", result.Reply.Text)
	}
	if result.ExitCode != ExitOK {
		t.Errorf("ExitCode = %d, want ExitOK: the turn finished on the second stream",
			result.ExitCode)
	}
	if got := len(driverPrompts(fs.EventReqs())); got != 1 {
		t.Errorf("prompt posts = %d, want 1: rejoining must not re-send a queued prompt", got)
	}
}

func TestDriverFollowsATurnAcrossStreams(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "sei-droid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_long", "ag_1"),
		// The prompt lands and the boundary is crossed, then the stream ends with
		// the turn still running.
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			// No done sentinel: the connection expired, it did not close.
		},
		// The next stream carries the turn's end.
		LaterStreamFrames: []string{
			driverAckFrame(),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			// Asked whether the turn is still going: it is, and what it has
			// committed so far is the agent's opening line, not its review.
			driverRunningSessionResp("conv_long", "ag_1", "resp_claude_a",
				driverReplyItem("item_narration", "resp_claude_a", "I'll read the diff.")),
			driverSessionWithItems("conv_long", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("The review, once it finished.", "comment"))),
		},
	})

	result, err := NewDriver(driverTestConfig(t, fs.URL), Policy{}, driverTestLogger()).
		Run(context.Background(), testWork{Repo: "sei-protocol/sandbox", PR: 51, Trigger: "t-long"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fs.StreamHits() < 2 {
		t.Fatalf("stream subscriptions = %d, want more than 1: a turn still running "+
			"must be followed onto the next stream", fs.StreamHits())
	}
	if result.Reply == nil {
		t.Fatal("Verdict = nil, want the finished review")
	}
	if got := len(driverPrompts(fs.EventReqs())); got != 1 {
		t.Errorf("prompt posts = %d, want 1: re-subscribing must not re-send a prompt "+
			"that is already queued", got)
	}
	if strings.Contains(result.Reply.Text, "I'll read the diff") {
		t.Error("published the agent's opening line: a turn the session still reports " +
			"as running must not be salvaged half-written")
	}
}
