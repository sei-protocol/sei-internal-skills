package omni

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// driverCreateReq is the subset of a session-create body this file asserts
// on.
type driverCreateReq struct {
	AgentID   string            `json:"agent_id"`
	HostType  string            `json:"host_type"`
	Title     string            `json:"title"`
	Workspace string            `json:"workspace"`
	Labels    map[string]string `json:"labels"`

	// A pointer so a test can tell an omitted field from an empty one. The SDK tags
	// this omitempty, and whether the default path sends "model_override":"" or
	// nothing at all is the difference between a server that validates the value
	// rejecting every create and accepting it.
	ModelOverride *string `json:"model_override"`
}

// driverPatchReq is the subset of a session-update body this file asserts on.
//
// A pointer, because the route's three states are distinct on the wire: absent
// leaves the field alone, and a value either sets or -- as a clear alias -- removes
// the override.
type driverPatchReq struct {
	ModelOverride *string `json:"model_override"`
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

	// CreateStatus fails POST /v1/sessions with this code, for the case where a
	// create commits server-side and loses its response.
	CreateStatus int

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

	// SessionListResps is served in order, last body repeating, for a test that
	// needs the search to answer differently before and after a create.
	SessionListResps []string

	// ItemsResp is the body for GET /v1/sessions/{id}/items, the paged read that
	// builds the pre-turn response-id set. Empty means an empty page, i.e. a
	// session with no history, which is what most tests want.
	ItemsResp string

	// ItemsResps serves one body per items request, so a test can exercise a
	// multi-page walk. Takes precedence over ItemsResp.
	ItemsResps []string

	// SessionListNeverEnds serves a listing that no SDK guard will stop: every page
	// carries a row, claims more, and advances its cursor. Only the driver's own
	// walk budget can end it, which is what makes it a test of that budget rather
	// than of the SDK's. Takes precedence over SessionListResp(s).
	//
	// Deliberately not the empty-page shape. The SDK stops on that one itself, so a
	// test built on it measures the SDK and passes whether or not this driver bounds
	// anything.
	SessionListNeverEnds bool

	// SessionListNeverEndsAfterFirst serves SessionListResps first, then the
	// never-ending shape. Lets a test cut a walk short with matches already found.
	SessionListNeverEndsAfterFirst bool

	// SessionResps is served in order, one per GET /v1/sessions/{id}, with the
	// last body repeating. The reply read and any adoption read are the same
	// route, so a test that needs them to differ configures both.
	SessionResps []string

	// EventStatus is the status the events route answers a prompt post with. Zero
	// answers normally. Non-zero models the ambiguous send: the server may or may
	// not have taken the prompt, and the caller cannot tell.
	EventStatus int

	// ApprovalStatus is the status the resolve route answers with. Zero means the
	// server's real 202. Lets a test fail answering a permission prompt without
	// touching the prompt path, which is a separate route.
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
	approvalStatus         int
	eventStatus            int
	sessionList            string
	sessionLists           []string
	itemsResp              string
	itemsResps             []string
	sessListNeverEnds      bool
	sessListNeverEndsAfter bool
	listItemsHits          atomic.Int64
	sessionResps           []string
	listSessHits           atomic.Int64
	getSessHits            atomic.Int64
	streamHits             atomic.Int64

	t   *testing.T
	URL string

	agentPages   []string
	createResp   string
	createStatus int
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

	// sessListQueries and itemsQueries record the raw query each listing page was
	// asked with. The cursor is the one part of a paginated walk a green suite can
	// lose silently: drop "after" and every page repeats, which still terminates.
	sessListQueries []string
	itemsQueries    []string
	createReqs      []driverCreateReq
	patchReqs       []driverPatchReq
	eventReqs       []driverEventReq
	deleteHits      int
	deletedIDs      []string
	tokenHits       int
}

func newDriverFakeServer(t *testing.T, cfg driverFakeServerConfig) *driverFakeServer {
	t.Helper()

	fs := &driverFakeServer{
		t:                      t,
		agentPages:             cfg.AgentPages,
		createResp:             cfg.CreateResp,
		createStatus:           cfg.CreateStatus,
		sessionLists:           cfg.SessionListResps,
		streamFrames:           cfg.StreamFrames,
		sandboxFrames:          cfg.SandboxFrames,
		laterStreamFrames:      cfg.LaterStreamFrames,
		sessionList:            cfg.SessionListResp,
		itemsResp:              cfg.ItemsResp,
		itemsResps:             cfg.ItemsResps,
		sessListNeverEnds:      cfg.SessionListNeverEnds,
		sessListNeverEndsAfter: cfg.SessionListNeverEndsAfterFirst,
		sessionResps:           cfg.SessionResps,
		approvalStatus:         cfg.ApprovalStatus,
		eventStatus:            cfg.EventStatus,
		eventResp:              cfg.EventResp,
		eventResps:             cfg.EventResps,
		deleteStatus:           cfg.DeleteStatus,
		tokenResp:              cfg.TokenResp,
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
	mux.HandleFunc("POST /v1/sessions/{id}/elicitations/{eid}/resolve", fs.handleResolve)
	mux.HandleFunc("POST /v1/sessions/{id}/events", fs.handleEvents)
	mux.HandleFunc("PATCH /v1/sessions/{id}", fs.handlePatchSession)
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
func (fs *driverFakeServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	hit := int(fs.listSessHits.Add(1))
	fs.mu.Lock()
	fs.sessListQueries = append(fs.sessListQueries, r.URL.RawQuery)
	fs.mu.Unlock()

	if fs.sessListNeverEnds || (fs.sessListNeverEndsAfter && hit > len(fs.sessionLists)) {
		// Costed, because an httptest server answers in microseconds: without this a
		// walk reaches the SDK's 10,000-page cap inside any sane time budget, and the
		// test would pin the SDK's bound rather than the driver's.
		time.Sleep(2 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		// A row, more promised, and a fresh cursor -- nothing here for the SDK to
		// object to. The session carries no run-key label, so a sweep walks past it.
		fmt.Fprintf(w,
			`{"data":[{"id":"conv_filler_%d","agent_id":"ag_1","labels":{}}],`+
				`"has_more":true,"last_id":"conv_filler_%d"}`, hit, hit)
		return
	}

	body := fs.sessionList
	if n := len(fs.sessionLists); n > 0 {
		body = fs.sessionLists[min(hit, n)-1]
	}
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
func (fs *driverFakeServer) handleListItems(w http.ResponseWriter, r *http.Request) {
	hit := int(fs.listItemsHits.Add(1))
	fs.mu.Lock()
	fs.itemsQueries = append(fs.itemsQueries, r.URL.RawQuery)
	fs.mu.Unlock()

	body := fs.itemsResp
	if n := len(fs.itemsResps); n > 0 {
		body = fs.itemsResps[min(hit, n)-1]
	}
	if body == driverItemsFail {
		// A listing that dies partway. The driver has to refuse rather than answer
		// from the pages it did get.
		http.Error(w, `{"error":"upstream"}`, http.StatusInternalServerError)
		return
	}
	if body == "" {
		body = `{"data":[],"has_more":false}`
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

// driverItemsFail marks a page the fake server answers with a 500 instead of a body, for
// a fixture that needs a listing to die partway.
const driverItemsFail = "\x00fail"

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

// handlePatchSession records a session update and answers with the session.
//
// It echoes rather than applying: nothing in this file reads a patched session back,
// and a fake that pretended to hold state would invite a test to trust it.
func (fs *driverFakeServer) handlePatchSession(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req driverPatchReq
	if err := json.Unmarshal(body, &req); err != nil {
		fs.t.Errorf("decode session-patch body: %v", err)
	}
	fs.mu.Lock()
	fs.patchReqs = append(fs.patchReqs, req)
	fs.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, driverSessionResp(r.PathValue("id"), "ag_1"))
}

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
	if fs.createStatus != 0 {
		w.WriteHeader(fs.createStatus)
		_, _ = io.WriteString(w, `{"detail":"create failed"}`)
		return
	}
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
	// Sandbox frames ride the first open only. The server delivers live from the
	// moment a client subscribes and replays nothing, so a re-subscribe sees the
	// launch edges only if they have not happened yet.
	if fs.streamHits.Load() > 1 {
		body = append([]string(nil), body...)
	} else {
		body = append(append([]string(nil), fs.sandboxFrames...), body...)
	}
	for _, frame := range body {
		if _, err := io.WriteString(w, frame); err != nil {
			return
		}
		_ = ctrl.Flush()
	}
}

// handleResolve answers a verdict on the dedicated resolve URL, which is where the
// SDK and upstream's own client both send one: the elicitation id travels in the
// path and the body carries only the action.
//
// The ack is what the server returns -- 202 with {"queued": false}, and no denial
// field. A verdict is resolved synchronously and persists no conversation item, so
// there is nothing to queue and nothing to read back. Kept distinct from the events
// route's ack on purpose: reusing that one here would let a driver read queued:true
// off a verdict, which no server sends.
//
// Recorded into the same list as an events POST, under type "approval", because what
// a test asks is whether the driver answered a prompt once with the right verdict --
// not which URL carried it.
func (fs *driverFakeServer) handleResolve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var result struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fs.t.Errorf("decode resolve body: %v", err)
	}
	fs.mu.Lock()
	fs.eventReqs = append(fs.eventReqs, driverEventReq{Type: "approval", Data: map[string]any{
		"elicitation_id": r.PathValue("eid"),
		"action":         result.Action,
		// Recorded so a test can catch a verdict posted to the reader rather than
		// the owner, which lands nowhere and parks the agent.
		"target_session_id": r.PathValue("id"),
	}})
	fs.mu.Unlock()

	if fs.approvalStatus != 0 {
		w.WriteHeader(fs.approvalStatus)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, `{"queued":false}`)
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

	if fs.eventStatus != 0 {
		w.WriteHeader(fs.eventStatus)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// The server acks by input class, not uniformly: {"queued":true,"item_id":...}
	// for an item-typed event, {"queued":false} for a control one. A fixture that
	// hands the queued shape to a control event lets a driver read an item id off
	// something that never persisted an item, and no test would see it.
	if req.Type != "message" && len(fs.eventResps) == 0 {
		_, _ = io.WriteString(w, `{"queued":false}`)
		return
	}
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

func (fs *driverFakeServer) SessListQueries() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.sessListQueries...)
}

func (fs *driverFakeServer) ItemsQueries() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]string(nil), fs.itemsQueries...)
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

func (fs *driverFakeServer) PatchReqs() []driverPatchReq {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return append([]driverPatchReq(nil), fs.patchReqs...)
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

// driverElicitationToolFrame carries tool_name, which the server sends as an
// undeclared field on the params object. It reaches the driver only through the
// SDK's catch-all map, so a frame that declares nothing but policy_name cannot
// exercise the tool allowlist at all.
func driverElicitationToolFrame(id, toolName string) string {
	payload := fmt.Sprintf(
		`{"type":"response.elicitation_request","elicitation_id":%q,`+
			`"params":{"message":"approve?","tool_name":%q}}`,
		id, toolName)
	return driverFrame("response.elicitation_request", payload)
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
	// The prompt item leads: recovery proves a reply sits after it, so a fixture
	// without it models a session that cannot exist.
	all := append([]string{driverPromptItem(driverAnchorItemID)}, items...)
	return fmt.Sprintf(`{"id":%q,"agent_id":%q,"created_at":1,"status":"idle","items":[%s]}`,
		id, agentID, strings.Join(all, ","))
}

// driverAnchorItemID is the item id the fake events route echoes back, and so the
// anchor every turn in these tests is bounded by.
const driverAnchorItemID = "item_1"

// driverPromptItem is the turn's own prompt, carrying no response id: it is input,
// not a reply.
func driverPromptItem(itemID string) string {
	return fmt.Sprintf(`{"id":%q,"type":"message","role":"user",`+
		`"content":[{"type":"input_text","text":"prompt"}]}`, itemID)
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

// driverAgentPageWithHarness is driverAgentPage plus the field that decides which
// signal ends a turn. Kept separate so the existing pages keep exercising the
// harness-absent case, which resolves to the stricter terminal-backed rule.
func driverAgentPageWithHarness(id, name, harness string) string {
	return fmt.Sprintf(
		`{"data":[{"id":%q,"name":%q,"created_at":1,"harness":%q}],"last_id":%q,"has_more":false}`,
		id, name, harness, id)
}

func driverTestConfig(t *testing.T, baseURL string) driver.Config {
	t.Helper()
	return driver.Config{
		BaseURL:           baseURL,
		Origin:            "test-origin",
		Agent:             "seidroid",
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
// the session is left running and Close reclaims it when the work ends.
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
			driverAgentPage("ag_1", "seidroid", "ag_1", false),
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK (%d)", result.ExitCode, driver.ExitOK)
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
		t.Errorf("driver.Reply.Text = %q, want %q", result.Reply.Text, reply)
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("reply does not carry decision approve: %q", result.Reply.Text)
	}
	if result.Reply.TurnID != "resp_claude_a" {
		t.Errorf("driver.Reply.TurnID = %q, want resp_claude_a: the comment names its own provenance",
			result.Reply.TurnID)
	}
	if result.Reply.ItemID != "item_reply" {
		t.Errorf("driver.Reply.ItemID = %q, want item_reply", result.Reply.ItemID)
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
	const unanswered = false
	if got := driverPromptText(t, events[0].Data); got != req.Prompt(unanswered) {
		t.Errorf("prompt sent = %q, want the workload's prompt %q", got, req.Prompt(unanswered))
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
// finished. It arrives before the prompt has even been persisted —
// 1.7, 3.1 and 6.0 seconds against boundaries at 7.2, 6.9 and 8.3 — so a driver
// that treats it as a turn end reads no reply and reports no verdict on a review
// that was about to succeed. Both a completed and a failed lifecycle event sit
// before the boundary here and neither may have any effect.
func TestDriverIgnoresTheInjectionAcknowledgement(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Read the diff.", "comment")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK: a response lifecycle event must not end a turn",
			result.ExitCode)
	}
	if !carriesDecision(result.Reply, "comment") {
		t.Fatalf("Verdict = %+v, want the decision the turn actually produced", result.Reply)
	}
}

// TestDriverIgnoresBareIdleEdges checks that an idle edge carrying no response id
// does not end a turn.
//
// Those edges are terminal churn rather than progress. A session carries
// five of them, one arriving 24 seconds into work that ran for 38, so "the first
// idle edge ends the turn" would cut that review off mid-tool-call.
func TestDriverIgnoresBareIdleEdges(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Two findings.", "request_changes")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if !carriesDecision(result.Reply, "request_changes") {
		t.Fatalf("Verdict = %+v, want request_changes: a bare idle edge must not end the turn",
			result.Reply)
	}
}

// TestDriverIgnoresATurnThatEndedBeforeItsOwnPrompt checks the boundary.
//
// The stream opens with a prologue replaying earlier work, so an id-bearing idle
// edge can arrive before the server has confirmed our own prompt. Taking it would
// attribute the reply of a previous invocation, which the prologue replays ahead of
// our prompt being persisted.
func TestDriverIgnoresATurnThatEndedBeforeItsOwnPrompt(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if result.Reply.TurnID != "resp_claude_a" {
		t.Errorf("driver.Reply.TurnID = %q, want resp_claude_a: an idle edge before the boundary "+
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if !carriesDecision(result.Reply, "approve") {
		t.Errorf("reply does not carry decision approve: attribution must read the "+
			"turn id, not the position: %q", result.Reply.Text)
	}
	if result.Reply.ItemID != "item_ours" {
		t.Errorf("driver.Reply.ItemID = %q, want item_ours", result.Reply.ItemID)
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitNoVerdict {
		t.Errorf("ExitCode = %d, want driver.ExitNoVerdict (%d): two reply groups must refuse",
			result.ExitCode, driver.ExitNoVerdict)
	}
	// Ambiguous attribution yields no reply at all, not an unusable one: the
	// driver refuses to name a turn's answer rather than guessing between two.
	if result.Reply != nil && result.Reply.Text != "" {
		t.Errorf("driver.Reply = %+v, want none when attribution is ambiguous", result.Reply)
	}
}

// TestDriverFailsWhenAPermissionPromptCannotBeAnswered is the regression test for
// the most expensive fault this driver can produce.
//
// The permission hook blocks the agent synchronously while it waits for an answer,
// so a prompt the driver fails to resolve stalls the review for the rest of the
// run: an unanswered prompt holds the agent for the rest of the budget
// while the transport stayed healthy the whole time. The previous version logged
// the failure and carried on reading a stream that would never produce anything.
func TestDriverFailsWhenAPermissionPromptCannotBeAnswered(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.NewPolicy("approve_shell", ""), driverTestLogger())

	result := d.Run(t.Context(), req)
	// driver.ExitTransport, not driver.ExitTurnFailed: the turn did not fail, we failed to answer
	// it. Reporting the agent's outcome for our own transport fault sends an
	// operator looking in the wrong place.
	if result.ExitCode != driver.ExitTransport {
		t.Errorf("ExitCode = %d, want driver.ExitTransport (%d): failing to answer a prompt is the "+
			"driver's fault, not the agent's", result.ExitCode, driver.ExitTransport)
	}
	if !result.TeardownOK {
		t.Error("TeardownOK = false, want true: teardown must still run")
	}
}

// TestDriverTurnFailedLeavesTheSessionRunning checks a failed status edge yields
// driver.ExitTurnFailed, carries no verdict, and — the point of the test — still releases
// the session.
func TestDriverTurnFailedLeavesTheSessionRunning(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitTurnFailed {
		t.Errorf("ExitCode = %d, want driver.ExitTurnFailed (%d)", result.ExitCode, driver.ExitTurnFailed)
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.NewPolicy("approve_shell", ""), driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitOK || !result.TeardownOK {
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
// ahead of the read reported driver.ExitTimeout on a completed, paid-for review.
func TestReplyForReadsAFinishedTurnEvenAfterTheClockExpires(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		SessionResps: []string{
			driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Finished just before the clock ran out.", "approve"))),
		},
	})

	h := New(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger())
	client, err := h.newClient(t.Context())
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	c := &conversation{host: h, client: client, sessionID: "conv_1"}

	// Already done, standing in for the deadline or the signal landing here.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	verdict, err := c.replyFor(ctx,
		&turn{id: "resp_claude_a", crossed: true, prior: map[string]bool{}})
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
			wantExit:   driver.ExitOK,
			wantReview: true,
		},
		{
			name:      "an unnamed failure cannot be attributed, so it stays a failure",
			failFrame: driverSessionFailedFrame("server_error", "transport lost"),
			reply:     driverVerdict("Committed, but nothing ties it to this turn.", "approve"),
			wantExit:  driver.ExitTurnFailed,
		},
		{
			name:      "a named turn whose reply has no verdict stays a failure",
			failFrame: driverSessionFailedFrameFor("resp_claude_a", "server_error", "transport lost"),
			reply:     "I was still reading the diff when the connection dropped.",
			wantExit:  driver.ExitTurnFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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

			result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
				Run(t.Context(), testWork{
					Repo: "sei-protocol/sandbox", PR: 20, Trigger: "trigger-salvage"})
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{
			Repo: "sei-protocol/sandbox", PR: 30, Trigger: "trigger-stale"})
	if result.Reply == nil {
		t.Fatal("Verdict = nil")
	}
	if result.Reply.TurnID != "resp_claude_ours" {
		t.Errorf("driver.Reply.TurnID = %q, want resp_claude_ours: an id already on the session "+
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
			wantExit:   driver.ExitOK,
			wantReview: true,
			wantLogged: "Finished just before the drop.",
		},
		{
			name: "two new replies cannot be told apart, so the drop stands",
			items: []string{
				driverReplyItem("item_a", "resp_claude_a", driverVerdict("Ours?", "approve")),
				driverReplyItem("item_b", "resp_claude_b", driverVerdict("Theirs?", "comment")),
			},
			wantExit: driver.ExitTransport,
		},
		{
			name: "a reply with no verdict is not worth recovering",
			items: []string{driverReplyItem("item_reply", "resp_claude_a",
				"I was still reading the diff when the connection dropped.")},
			wantExit:   driver.ExitTransport,
			wantLogged: "I was still reading the diff when the connection dropped.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
			result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, log).
				Run(t.Context(), testWork{
					Repo: "sei-protocol/sandbox", PR: 31, Trigger: "trigger-drop"})
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	d.Run(t.Context(), req)
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_failed", "ag_1"),
		SandboxFrames: []string{
			driverSandboxFrame("provisioning", ""),
			driverSandboxFrame("failed", reason),
		},
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 12, Trigger: "trigger-failed"}
	d := newTestDriver(cfg, driver.Policy{}, driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode != driver.ExitTurnFailed {
		t.Errorf("ExitCode = %d, want driver.ExitTurnFailed (%d)", result.ExitCode, driver.ExitTurnFailed)
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_cold", "ag_1"),
		// The sandbox never gets past connecting, and the stream ends each time.
		SandboxFrames: []string{driverSandboxFrame("connecting", "")},
		StreamFrames:  []string{driverAckFrame(), driverDoneFrame()},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 31, Trigger: "trigger-cold"}

	newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Run(t.Context(), req)

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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 41, Trigger: "t-pending"})
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
// The connection has a lifetime of its own, around three minutes,
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
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_idle", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			// The connection expired mid-turn: no done sentinel.
		},
		LaterStreamFrames: []string{
			driverConsumedFrame("item_2"),
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

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 52, Trigger: "t-idle"})
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
	if result.ExitCode != driver.ExitOK {
		t.Errorf("ExitCode = %d, want driver.ExitOK: the turn finished on the second stream",
			result.ExitCode)
	}
	if got := len(driverPrompts(fs.EventReqs())); got != 1 {
		t.Errorf("prompt posts = %d, want 1: rejoining must not re-send a queued prompt", got)
	}
}

func TestDriverFollowsATurnAcrossStreams(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
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

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 51, Trigger: "t-long"})
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

// TestCloseReportsAnUnreachableServerAsTransport pins the close path's exit code
// to the same rule the run path uses.
//
// The mint deliberately leaves a failure-to-reach unwrapped so it classifies as
// transport, because the exit code is the caller's contract: it decides whether a
// workflow retries an outage or tells an operator to go fix a secret. Reading
// every client failure as configuration throws that distinction away.
func TestCloseReportsAnUnreachableServerAsTransport(t *testing.T) {
	t.Parallel()

	// A port with nothing behind it: the exchange fails in transit rather than
	// being rejected, which is the case the two codes separate.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	cfg := driverTestConfig(t, "http://"+addr)
	cfg.Token = ""
	cfg.MachineClientID = "client"
	cfg.MachineClientSecret = "secret"

	result := newTestDriver(cfg, driver.Policy{}, driverTestLogger()).
		Close(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})
	if result.ExitCode != driver.ExitTransport {
		t.Errorf("ExitCode = %d, want driver.ExitTransport (%d): an outage is retryable, a bad secret is not",
			result.ExitCode, driver.ExitTransport)
	}
}

// TestHealthCheckedClientPingsAnIdleConnection pins the settings that let a dead
// connection be noticed.
//
// A flow dropped without a reset leaves the socket ESTABLISHED and the connection a
// reuse candidate, so requests are written into a socket nothing reads and recovery
// waits on the kernel's retransmit ceiling. The pings are what turn that into a
// re-dial.
func TestHealthCheckedClientPingsAnIdleConnection(t *testing.T) {
	t.Parallel()

	t.Run("the pings are configured", func(t *testing.T) {
		t.Parallel()
		// Configured here rather than read back off a finished client: http2 refuses
		// a transport it has already enabled, so asking twice yields an error and no
		// settings to inspect.
		h2, err := configureHealthChecks(http.DefaultTransport.(*http.Transport).Clone())
		if err != nil {
			t.Fatalf("configureHealthChecks: %v", err)
		}
		if h2.ReadIdleTimeout != http2ReadIdleTimeout {
			t.Errorf("ReadIdleTimeout = %v, want %v", h2.ReadIdleTimeout, http2ReadIdleTimeout)
		}
		if h2.PingTimeout != http2PingTimeout {
			t.Errorf("PingTimeout = %v, want %v", h2.PingTimeout, http2PingTimeout)
		}
	})

	t.Run("the idle bound clears the server's heartbeat", func(t *testing.T) {
		t.Parallel()
		// Below it, a healthy stream is pinged for being quiet between heartbeats.
		if http2ReadIdleTimeout <= 15*time.Second {
			t.Errorf("http2ReadIdleTimeout = %v, must exceed the server's 15s heartbeat",
				http2ReadIdleTimeout)
		}
	})

	t.Run("the client bounds headers and not the body", func(t *testing.T) {
		t.Parallel()
		client, err := healthCheckedClient(driverTestLogger())
		if err != nil {
			t.Fatalf("healthCheckedClient: %v", err)
		}
		traced, ok := client.Transport.(*tracingTransport)
		if !ok {
			t.Fatalf("Transport = %T, want *tracingTransport", client.Transport)
		}
		transport, ok := traced.base.(*http.Transport)
		if !ok {
			t.Fatalf("base = %T, want *http.Transport", traced.base)
		}
		if transport.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
			t.Errorf("ResponseHeaderTimeout = %v, want %v",
				transport.ResponseHeaderTimeout, defaultResponseHeaderTimeout)
		}
		// A stream's body is unbounded by design; a whole-request timeout would cut
		// a healthy long turn.
		if client.Timeout != 0 {
			t.Errorf("Timeout = %v, want 0", client.Timeout)
		}
	})
}

// TestDriverInProcessTurnEndsOnResponseCompleted is the codex scout's failure,
// reduced. The harness is in-process, so no status edge ever carries a response
// id — the server documents that field as absent there. Before this, the wait ran
// to its deadline and discarded a complete report; the recorded run produced a
// valid verdict at 18:17:18 and gave up at 18:23:22.
func TestDriverInProcessTurnEndsOnResponseCompleted(t *testing.T) {
	t.Parallel()

	reply := driverVerdict("Two findings.", "comment")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPageWithHarness("ag_1", "seidroid", "codex")},
		CreateResp: driverSessionResp("conv_1", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverCreatedFrame(),
			driverDeltaFrame("Two find"),
			driverCompletedFrame(),
			// No id-bearing idle: the whole point is that one never arrives.
			driverBareIdleFrame(),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_1", reply)),
		},
	})

	d := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger())
	result := d.Run(t.Context(),
		testWork{Repo: "sei-protocol/sandbox", PR: 42, Trigger: "in-process"})
	if result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK (%d)", result.ExitCode, driver.ExitOK)
	}
	if result.Reply == nil {
		t.Fatal("driver.Reply = nil: the turn never ended, which is the bug this covers")
	}
	if result.Reply.Text != reply {
		t.Errorf("driver.Reply.Text = %q, want %q", result.Reply.Text, reply)
	}
	if result.Reply.TurnID != "resp_1" {
		t.Errorf("driver.Reply.TurnID = %q, want resp_1 (the completed response)", result.Reply.TurnID)
	}
}

// TestDriverTerminalBackedIgnoresResponseCompleted is the regression guard for the
// harness we did not break. There the same event only acknowledges that the prompt
// reached the terminal, so it arrives before the answer exists; ending on it
// publishes a review the agent has not written yet.
//
// Both candidate ends are on the stream, each with its own reply attributed to it.
// Ending on the wrong one is therefore visible in the published text, not just in
// a timing.
func TestDriverTerminalBackedIgnoresResponseCompleted(t *testing.T) {
	t.Parallel()

	finished := driverVerdict("Two findings.", "comment")
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPageWithHarness("ag_1", "seidroid", "claude-native")},
		CreateResp: driverSessionResp("conv_1", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverCreatedFrame(),
			// The injection acknowledgement. Ending here is the failure.
			driverCompletedFrame(),
			driverBareIdleFrame(),
			driverRunningFrame("resp_claude_a"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			// Only the finished reply is on the session. Ending on the
			// acknowledgement would look for resp_1's reply, which does not exist,
			// so the wrong end cannot quietly produce the right text.
			driverSessionWithItems("conv_1", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a", finished)),
		},
	})

	d := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger())
	result := d.Run(t.Context(),
		testWork{Repo: "sei-protocol/sandbox", PR: 42, Trigger: "terminal-backed"})
	if result.Reply == nil {
		t.Fatal("driver.Reply = nil, want the finished reply")
	}
	if result.Reply.TurnID != "resp_claude_a" {
		t.Fatalf("driver.Reply.TurnID = %q, want resp_claude_a: the turn ended on the "+
			"acknowledgement instead of the Stop-hook edge", result.Reply.TurnID)
	}
	if result.Reply.Text != finished {
		t.Errorf("published %q, want %q", result.Reply.Text, finished)
	}
}

// TestTerminalBacked pins the classification, including the fail-safe: an
// unrecognised or absent harness keeps the stricter rule, because publishing a
// half-written review costs more than waiting out a deadline.
func TestTerminalBacked(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		harness string
		want    bool
	}{
		{"codex", false},
		{"claude_sdk", false},
		{"openai-agents", false},
		{"pi", false},
		{"claude-native", true},
		{"codex-native", true},
		{"native-claude", true},
		{"CLAUDE-NATIVE", true},
		{"  codex-native  ", true},
		{"", true},
		{"something-we-have-not-seen", true},
	} {
		if got := terminalBacked(tc.harness); got != tc.want {
			t.Errorf("terminalBacked(%q) = %t, want %t", tc.harness, got, tc.want)
		}
	}
}

// TestAReplyCarryingACredentialIsNeverPublished is the wiring test for the scan.
//
// driver.ScanSecrets is unit-tested in its own package, but nothing asserted it was
// still called on the path a reply actually takes. That is the failure that matters:
// the scan can be dropped from fetchReply and every existing test stays green, while
// a credential the agent quoted reaches a public pull request.
func TestAReplyCarryingACredentialIsNeverPublished(t *testing.T) {
	t.Parallel()

	// A shape the patterns catch, embedded in an otherwise complete answer, so the
	// only thing standing between it and the caller is the scan.
	leaked := "Here is the token I found: ghp_" + strings.Repeat("A", 36) +
		"\n```json\n{\"decision\": \"approve\"}\n```"

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_1", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame(driverAnchorItemID),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_1", "ag_1",
				driverPromptItem(driverAnchorItemID),
				driverReplyItem("item_reply", "resp_claude_a", leaked)),
		},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 77})

	if result.Reply != nil && strings.Contains(result.Reply.Text, "ghp_") {
		t.Fatal("the reply reached the caller with a credential in it: the scan is " +
			"no longer on the path a reply takes")
	}
	if result.ExitCode != driver.ExitNoVerdict {
		t.Errorf("ExitCode = %d, want ExitNoVerdict (%d): a refused reply is not an answer",
			result.ExitCode, driver.ExitNoVerdict)
	}
	if result.Reply == nil || !strings.Contains(result.Reply.Reason, "credential") {
		t.Errorf("Reason = %+v, want it to name why nothing was published", result.Reply)
	}
}

// TestAnAmbiguousSendReportsTheTurnNotTheTransport covers what the caller is told
// when a prompt's fate cannot be established.
//
// The server queues the prompt but names neither an item nor a pending input, so
// this run has no anchor and cannot attribute a reply. If the session is meanwhile
// answering something, the prompt must not be sent again -- and the outcome the
// caller branches on has to be the turn's failure, not the stream error that
// exposed it, or a workflow reads a transport fault and the diagnostic is dropped.
func TestAnAmbiguousSendReportsTheTurnNotTheTransport(t *testing.T) {
	t.Parallel()

	runKey := testRunKey("sei-protocol/sandbox", 22)
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		SessionListResp: `{"data":[{"id":"conv_1","labels":` +
			`{"` + RunKeyLabel + `":"` + runKey + `"}}],"has_more":false}`,
		SessionResps: []string{
			// Adopted live, then answering: the send landed after all.
			driverSessionResp("conv_1", "ag_1"),
			driverRunningSessionResp("conv_1", "ag_1", "resp_claude_a"),
		},
		// The send fails as transport: the server may have taken the prompt and lost
		// the answer, which is the case a caller cannot tell apart and must not
		// repeat.
		EventStatus:  http.StatusBadGateway,
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if got := driverPrompts(fs.EventReqs()); len(got) != 1 {
		t.Errorf("prompt posts = %d, want 1: a prompt whose fate is unknown must not "+
			"be repeated to a session already answering", len(got))
	}
	if result.ExitCode != driver.ExitTurnFailed {
		t.Errorf("ExitCode = %d, want ExitTurnFailed (%d): the turn's own failure is "+
			"what the caller branches on, not the stream error that surfaced it",
			result.ExitCode, driver.ExitTurnFailed)
	}
}

// TestAPromptParkedWhileDisconnectedIsAnswered covers a prompt raised off-stream.
//
// The stream replays nothing, so a permission prompt raised while no stream was
// attached is never delivered. The hook blocks the agent synchronously while it
// waits, so one this run never answers holds the turn for the rest of its budget
// with the transport looking perfectly healthy. A reconnect is when such a prompt is
// sitting there, so the snapshot is swept on every one.
func TestAPromptParkedWhileDisconnectedIsAnswered(t *testing.T) {
	t.Parallel()

	parked := `{"id":"parked_1","params":{"policy_name":"approve_shell",` +
		`"phase":"pre_tool_use","tool_name":"Bash"}}`
	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_1", "ag_1"),
		SessionListResp: `{"data":[{"id":"conv_1","labels":` +
			`{"` + RunKeyLabel + `":"` + testRunKey("sei-protocol/sandbox", 22) + `"}}],` +
			`"has_more":false}`,
		SessionResps: []string{
			// Adopted, then holding a prompt raised while nothing was listening.
			`{"id":"conv_1","agent_id":"ag_1","created_at":1,"status":"idle","items":[],` +
				`"pending_elicitations":[` + parked + `]}`,
		},
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	newTestDriver(driverTestConfig(t, fs.URL), driver.NewPolicy("approve_shell", ""),
		driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	approvals := 0
	for _, r := range fs.EventReqs() {
		if r.Type == "approval" {
			approvals++
		}
	}
	if approvals == 0 {
		t.Error("a prompt parked on the session was never answered, so the agent would " +
			"stay blocked until the run deadline")
	}
}

// TestAnAttemptedSendIsNeverRepeated covers what a session read cannot settle.
//
// A prompt queued and not yet active leaves no active response, so the session looks
// idle while holding it — and reading the session was what a resend keyed on.
// Posting an input carries no idempotency key, so the attempt is remembered locally.
func TestAnAttemptedSendIsNeverRepeated(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		SessionListResp: `{"data":[{"id":"conv_1","labels":` +
			`{"` + RunKeyLabel + `":"` + testRunKey("sei-protocol/sandbox", 22) + `"}}],` +
			`"has_more":false}`,
		// Idle on every read: no active response, which is what a queued prompt looks
		// like from here.
		SessionResps: []string{driverSessionResp("conv_1", "ag_1")},
		EventStatus:  http.StatusBadGateway,
		StreamFrames: []string{driverAckFrame(), driverDoneFrame()},
	})

	result := newTestDriver(driverTestConfig(t, fs.URL), driver.Policy{}, driverTestLogger()).
		Run(t.Context(), testWork{Repo: "sei-protocol/sandbox", PR: 22})

	if got := driverPrompts(fs.EventReqs()); len(got) != 1 {
		t.Errorf("prompt posts = %d, want 1: a send whose answer never arrived must not "+
			"be repeated", len(got))
	}
	if result.ExitCode != driver.ExitTurnFailed {
		t.Errorf("ExitCode = %d, want ExitTurnFailed (%d)", result.ExitCode, driver.ExitTurnFailed)
	}
}

// TestDriverDeclinesWhatThePolicyDoesNotAllow follows the verdict to the wire.
//
// Policy.Decide has thorough unit coverage, and none of it reaches the one line
// that calls it. Both of these mutations of that call site left the whole suite
// green before this test existed:
//
//	action, reason := driver.Accept, "ignored the policy"
//	action, reason := c.host.policy.Decide(e); if action == driver.Decline { action = driver.Accept }
//
// Which is the same gap the reply path already has a test for, and for the same
// stated reason: a guard nothing follows to the wire can be dropped from the path
// without a single test noticing. Here that means approving every tool call an
// agent makes while it reads an attacker-authored diff, and policy.go says what
// that is worth -- "Bash" is arbitrary shell, so allowing it allows a push as
// readily as a diff read.
//
// The zero Policy is the case that matters most: it is what an operator with no
// allowlist configured gets, and it must decline everything.
func TestDriverDeclinesWhatThePolicyDoesNotAllow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		policy driver.Policy
	}{
		{"the zero policy allows nothing", driver.Policy{}},
		{"an allowlist that does not name this tool", driver.NewPolicy("", "Read")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := newDriverFakeServer(t, driverFakeServerConfig{
				AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
				CreateResp: driverSessionResp("conv_deny", "ag_1"),
				StreamFrames: []string{
					driverAckFrame(),
					driverConsumedFrame("item_1"),
					driverElicitationToolFrame("elicit_1", "Bash"),
					driverIdleFrame("resp_claude_a"),
					driverDoneFrame(),
				},
				SessionResps: []string{
					driverSessionWithItems("conv_deny", "ag_1",
						driverReplyItem("item_reply", "resp_claude_a",
							driverVerdict("Could not run it.", "approve"))),
				},
			})

			cfg := driverTestConfig(t, fs.URL)
			req := testWork{Repo: "sei-protocol/sandbox", PR: 31, Trigger: "trigger-deny"}
			d := newTestDriver(cfg, tc.policy, driverTestLogger())

			if result := d.Run(t.Context(), req); result.ExitCode != driver.ExitOK {
				t.Fatalf("ExitCode = %d, want driver.ExitOK: a declined prompt is the "+
					"agent's problem to work around, not a failed run", result.ExitCode)
			}

			var approvals []driverEventReq
			for _, e := range fs.EventReqs() {
				if e.Type == "approval" {
					approvals = append(approvals, e)
				}
			}
			if len(approvals) != 1 {
				t.Fatalf("approval POSTs = %d, want exactly 1", len(approvals))
			}
			if got := approvals[0].Data["action"]; got != "decline" {
				t.Errorf("data.action = %v, want decline: the policy allows no such "+
					"tool, and the verdict on the wire is the only thing that enforces it", got)
			}
		})
	}
}

// TestDriverAllowsAToolNamedOnlyOnTheWire guards a regression that shipped once.
//
// tool_name is not a declared field on the elicitation params, so it survives only
// as long as the generated type keeps its catch-all map. A spec transform that drops
// that map still compiles, still passes every other test, and silently declines every
// prompt the tool allowlist was supposed to accept. This is the only test where the
// name has to travel the whole way -- frame, catch-all, policy -- to pass.
func TestDriverAllowsAToolNamedOnlyOnTheWire(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_tool", "ag_1"),
		StreamFrames: []string{
			driverAckFrame(),
			driverConsumedFrame("item_1"),
			driverElicitationToolFrame("elicit_tool", "Bash"),
			driverIdleFrame("resp_claude_a"),
			driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_tool", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Ran a command.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 21, Trigger: "trigger-tool"}
	// No allowed policy names -- the tool allowlist is the only thing that can accept.
	d := newTestDriver(cfg, driver.NewPolicy("", "Bash"), driverTestLogger())

	if result := d.Run(t.Context(), req); result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
	}

	var approvals []driverEventReq
	for _, e := range fs.EventReqs() {
		if e.Type == "approval" {
			approvals = append(approvals, e)
		}
	}
	if len(approvals) != 1 {
		t.Fatalf("approval POSTs = %d, want exactly 1", len(approvals))
	}
	if got := approvals[0].Data["action"]; got != "accept" {
		t.Errorf("data.action = %v, want accept -- the tool name did not reach the policy", got)
	}
}

// TestDriverCarriesTheCursorAcrossEverySessionsPage pins the one part of a
// paginated walk that fails silently. Drop the cursor and every page repeats the
// first: the walk still ends, the suite still passes, and a session on page two is
// never found -- so Close never reclaims its sandbox.
func TestDriverCarriesTheCursorAcrossEverySessionsPage(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		CreateResp: driverSessionResp("conv_cursor", "ag_1"),
		SessionListResps: []string{
			`{"data":[{"id":"conv_other","agent_id":"ag_1","labels":{}}],` +
				`"has_more":true,"last_id":"conv_other"}`,
			`{"data":[],"has_more":false}`,
		},
		StreamFrames: []string{
			driverAckFrame(), driverConsumedFrame("item_1"),
			driverIdleFrame("resp_claude_a"), driverDoneFrame(),
		},
		SessionResps: []string{
			driverSessionWithItems("conv_cursor", "ag_1",
				driverReplyItem("item_reply", "resp_claude_a",
					driverVerdict("Read it.", "approve"))),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 23, Trigger: "trigger-cursor"}
	d := newTestDriver(cfg, driver.NewPolicy("", ""), driverTestLogger())

	if result := d.Run(t.Context(), req); result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
	}

	queries := fs.SessListQueries()
	if len(queries) < 2 {
		t.Fatalf("sessions listing requests = %d, want at least 2 pages", len(queries))
	}
	if strings.Contains(queries[0], "after=") {
		t.Errorf("first page query = %q, want no cursor", queries[0])
	}
	if !strings.Contains(queries[1], "after=conv_other") {
		t.Errorf("second page query = %q, want after=conv_other", queries[1])
	}
	for i, q := range queries {
		if !strings.Contains(q, "limit=1000") {
			t.Errorf("page %d query = %q, want limit=1000 on every page", i, q)
		}
	}
}

// TestDriverCollectsPriorResponseIDsFromEveryItemsPage covers the same cursor on the
// items route. A response id missed here is one the turn machine would accept as its
// own, so a lost page publishes a stale verdict rather than failing.
func TestDriverCollectsPriorResponseIDsFromEveryItemsPage(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		SessionListResp: `{"data":[{"id":"conv_pages","agent_id":"ag_1",` +
			`"labels":{"` + RunKeyLabel + `":"` + testRunKey("sei-protocol/sandbox", 24) + `"}}],"has_more":false}`,
		SessionResps: []string{
			driverSessionResp("conv_pages", "ag_1"),
			driverSessionWithItems("conv_pages", "ag_1",
				driverReplyItem("item_reply", "resp_claude_new",
					driverVerdict("Read it.", "approve"))),
		},
		ItemsResps: []string{
			`{"data":[{"id":"i1","response_id":"resp_old_a"}],"has_more":true,"last_id":"i1"}`,
			`{"data":[{"id":"i2","response_id":"resp_old_b"}],"has_more":false}`,
		},
		StreamFrames: []string{
			driverAckFrame(), driverConsumedFrame("item_1"),
			// The prior ids must both be excluded, so a reply on either is not ours.
			driverIdleFrame("resp_old_b"),
			driverIdleFrame("resp_claude_new"),
			driverDoneFrame(),
		},
	})

	cfg := driverTestConfig(t, fs.URL)
	req := testWork{Repo: "sei-protocol/sandbox", PR: 24, Trigger: "trigger-items"}
	d := newTestDriver(cfg, driver.NewPolicy("", ""), driverTestLogger())

	if result := d.Run(t.Context(), req); result.ExitCode != driver.ExitOK {
		t.Fatalf("ExitCode = %d, want driver.ExitOK", result.ExitCode)
	}

	queries := fs.ItemsQueries()
	if len(queries) < 2 {
		t.Fatalf("items listing requests = %d, want at least 2 pages", len(queries))
	}
	if !strings.Contains(queries[1], "after=i1") {
		t.Errorf("second items page query = %q, want after=i1", queries[1])
	}
}

// TestDriverStopsAListingThatNeverEnds bounds the walk, not the request.
//
// The SDK's iterator stops on the last page or on its own 10,000-page backstop. It
// does not stop on an empty page that still claims more -- the shape the hand-written
// loops this replaced did stop on. Unbounded, one listing spends the whole budget it
// was given: on Close that is the teardown window, and the run then exits by deadline
// rather than by the leak path, so nothing names the sandbox left behind.
func TestDriverStopsAListingThatNeverEnds(t *testing.T) {
	t.Parallel()

	fs := newDriverFakeServer(t, driverFakeServerConfig{
		AgentPages: []string{driverAgentPage("ag_1", "seidroid", "ag_1", false)},
		// Empty, but always more, with a cursor that advances -- so neither of the
		// SDK's own self-defence stops fires either.
		SessionListNeverEnds: true,
	})

	cfg := driverTestConfig(t, fs.URL)
	cfg.RequestTimeout = 100 * time.Millisecond
	req := testWork{Repo: "sei-protocol/sandbox", PR: 25, Trigger: "trigger-runaway"}
	d := newTestDriver(cfg, driver.NewPolicy("", ""), driverTestLogger())

	result := d.Run(t.Context(), req)
	if result.ExitCode == driver.ExitOK {
		t.Fatal("ExitCode = driver.ExitOK, want a failure -- the listing never ends")
	}
	// The bound is 2 x RequestTimeout, halved against what the caller has left.
	// Reaching the SDK's 10,000-page backstop would
	// mean the walk ran on the run deadline instead.
	if hits := fs.ListSessionHits(); hits >= 10000 {
		t.Errorf("sessions listing requests = %d, want the walk bounded well short of "+
			"the SDK's page cap", hits)
	}
}
