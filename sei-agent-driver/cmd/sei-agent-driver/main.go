// Command sei-agent-driver drives one Omnigent agent session to a
// machine-readable answer, and writes that answer out for a caller to publish.
//
// One subcommand per workload. Everything a workload does not decide — resolving
// the agent, creating or adopting the session, following a turn across the
// streams it outlives, answering the permission prompts it raises, telling a
// finished turn from an agent still working — is the driver's, and is the same
// whatever the workload asks for. `xreview` is the first; a workload is a prompt,
// a way to read the reply, and somewhere to put it.
//
// Configured entirely by environment so no credential lands in an argument; the
// arguments identify only the unit of work. Exit codes are the contract with the
// calling workflow — see the Exit constants in the driver package.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/omni"
	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/xreview"
	"github.com/urfave/cli/v3"
)

// version is stamped at build time; see the driver-build target. "dev" means a
// build that did not go through it, which a consumer pinning an artifact needs
// to be able to tell.
var version = "dev"

// resolvedVersion prefers the stamped value and falls back to the module version
// the toolchain records.
//
// A `go install` applies no ldflags, so a binary fetched that way would otherwise
// report "dev" and a consumer could not tell which release it was running. The
// toolchain does record the module version for a proxy-fetched build, which is a
// different thing from the VCS stamping a nested module cannot do.
func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		// "(devel)" is a local build off a working tree, which is what "dev" means.
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

// logLevel reads LOG_LEVEL, defaulting to info.
//
// Unprefixed, unlike this tool's own knobs: verbosity is the runner's business
// rather than this review's, and a workflow setting it once for every step should
// not have to know which binary reads it.
//
// Debug carries a line per HTTP request with the connection it used and how long it
// had been idle. That is noise on a healthy run and the whole diagnosis on a run
// whose connection died, so it is a knob rather than a rebuild.
func logLevel() slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		return slog.LevelInfo
	}
	return level
}

func main() {
	// Logs go to stderr so stdout carries only the verdict payload and a caller
	// can consume one without parsing around the other.
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel()}))

	cmd := &cli.Command{
		Name:    "sei-agent-driver",
		Usage:   "drive an Omnigent agent session to a machine-readable answer",
		Version: resolvedVersion(),
		Commands: []*cli.Command{
			xreviewCommand(log),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// Log first, then exit. Every failure says why, including the ones that
		// carry their own code — an operator meeting a bare exit 2 with no reason
		// has to go read this source to find out what was missing.
		log.Error("the driver failed", "error", err)

		// An exitError carries the code the driver decided; anything else never
		// got as far as a run and is a usage or configuration fault.
		var exit *exitError
		if errors.As(err, &exit) {
			os.Exit(exit.code)
		}
		os.Exit(driver.ExitConfig)
	}
}

// xreviewCommand is the pull-request review workload: it hands the agent a diff
// to read and expects a verdict back, then leaves the caller to post it.
//
// The flags are all about where the answer goes, because that is the only part
// of a workload the caller has to wire. What identifies the work stays in the
// arguments.
func xreviewCommand(log *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:      "xreview",
		Usage:     "review a pull request with the seidroid agent",
		ArgsUsage: "<owner/name> <pr-number>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "out",
				Usage: "write the verdict text here for the caller to post",
			},
			&cli.StringFlag{
				Name:  "findings-out",
				Usage: "write the placeable findings here as json for the caller to post inline",
			},
			&cli.StringFlag{
				Name: "guidelines-file",
				Usage: "repository standards file to read from the base branch " +
					"(default " + xreview.DefaultGuidelinesFile + ")",
			},
			&cli.StringFlag{
				Name:  "extra-instructions",
				Usage: "guidance this repository adds to every review",
			},
			&cli.StringFlag{
				Name:  "conversation-context",
				Usage: "read this tool's earlier findings and their replies from here as json",
			},
			&cli.StringFlag{
				Name:  "check-out",
				Usage: "write the check run here as json for the caller to publish",
			},
			&cli.BoolFlag{
				Name: "close",
				Usage: "delete this pull request's session instead of reviewing; " +
					"the only thing that reclaims its sandbox",
			},
			&cli.StringFlag{
				Name: "trigger-id",
				Usage: "identifies this dispatch in the logs, e.g. the triggering " +
					"comment id; defaults to the workflow run and attempt. Not part " +
					"of the session key -- that is the pull request",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return run(ctx, cmd, log)
		},
	}
}

// exitError carries a driver exit code up through cli's error return.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func run(ctx context.Context, cmd *cli.Command, log *slog.Logger) error {
	// First, before anything can return. A caller publishes on a file being present,
	// so an output left by an earlier run on a reused workspace is that run's verdict
	// posted under this one's name. Clearing inside report is too late: a bad target,
	// a missing credential and a malformed scout list all exit ahead of it.
	if err := clearOutputs(cmd.String("out"), cmd.String("findings-out"),
		cmd.String("check-out")); err != nil {
		return &exitError{code: driver.ExitConfig, err: err}
	}

	repo, pr, err := parseTarget(cmd.Args().Slice())
	if err != nil {
		return err
	}

	cfg, err := driver.LoadConfig()
	if err != nil {
		return &exitError{code: driver.ExitConfig, err: err}
	}
	if err := cfg.RequireAuth(); err != nil {
		return &exitError{code: driver.ExitConfig, err: err}
	}

	// A terminate signal cancels the context rather than killing the process, so the
	// run unwinds and reports instead of dying mid-write. It reclaims nothing: a
	// review deletes no session by design, and only --close does. A second signal is
	// not caught, which is the operator's override when the unwind is what is
	// hanging.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	policy := driver.NewPolicy(
		os.Getenv("XREVIEW_ALLOW_POLICIES"),
		os.Getenv("XREVIEW_ALLOW_TOOLS"),
	)
	if len(policy.AllowPolicies) == 0 && len(policy.AllowTools) == 0 {
		log.Warn("no permission allowlist configured; every prompt will be declined",
			"hint", "set XREVIEW_ALLOW_TOOLS to the tool_name values this run logs")
	}

	req := xreview.Request{
		Repo: repo,
		PR:   pr,
		Trigger: xreview.TriggerID(
			cmd.String("trigger-id"),
			os.Getenv("GITHUB_RUN_ID"),
			os.Getenv("GITHUB_RUN_ATTEMPT"),
			repo, pr,
		),
	}

	// Logged, which is the whole of what a trigger id is for. It is deliberately not
	// part of the session key -- every dispatch for a pull request adopts that pull
	// request's one session -- so the only way it earns its place is by being here
	// when someone reads back why a particular run happened.
	log.Info("dispatch", "repo", repo, "pr", pr, "trigger_id", req.Trigger)

	req.GuidelinesFile = cmd.String("guidelines-file")
	req.ExtraInstructions = cmd.String("extra-instructions")

	// A history that cannot be read is not a reason to refuse the review. The
	// review is still correct without it — it just repeats itself — where refusing
	// leaves the pull request with no review at all.
	if path := cmd.String("conversation-context"); path != "" {
		threads, err := readPriorThreads(path)
		if err != nil {
			log.Warn("could not read the earlier findings; reviewing without them",
				"path", path, "error", err)
		}
		req.PriorThreads = threads
	}

	// Parsed leniently here and enforced below, so a malformed scout list cannot
	// refuse --close: that is the only path that reclaims a sandbox, and it must
	// not depend on an unrelated variable being well-formed.
	specs, scoutErr := parseScouts(os.Getenv("XREVIEW_SCOUTS"), cfg.Agent)

	// The driver states what a run needs from a server; omni is the implementation
	// that reaches this one. Wiring them is this binary's job -- it is the only
	// place that knows both.
	d := driver.New(cfg, omni.New(cfg, policy, log), log)

	// --close ends the unit of work. Reviewing keeps the conversation so the next
	// invocation builds on it; this is what finally destroys it, and with it the
	// sandbox the Kubernetes launcher cannot otherwise reclaim.
	if cmd.Bool("close") {
		// The scouts hold sessions of their own, and nothing else reclaims them.
		// Deleted first and best-effort: the review's session is the one whose
		// failure the exit code must report, and a scout that cannot be found is
		// already gone.
		if scoutErr != nil {
			log.Warn("the scout list is malformed, so only the review's session is deleted",
				"error", scoutErr)
		}
		for _, spec := range specs {
			if scout := d.Close(ctx, xreview.NewScout(req, spec.name, spec.agent)); !scout.TeardownOK {
				log.Warn("could not reclaim a scout's session; its sandbox may be left running",
					"scout", spec.name, "session_id", scout.SessionID, "exit_code", scout.ExitCode)
			}
		}
		result := d.Close(ctx, xreview.New(req))
		if err := report("", "", "", result); err != nil {
			return &exitError{code: driver.ExitConfig, err: err}
		}
		if result.ExitCode != driver.ExitOK {
			return &exitError{code: result.ExitCode,
				err: fmt.Errorf("close finished with exit code %d", result.ExitCode)}
		}
		return nil
	}

	if scoutErr != nil {
		return &exitError{code: driver.ExitConfig, err: scoutErr}
	}

	// The readings are gathered before the review so they can ride in its prompt.
	// Their budget is carved out of the run deadline; see [scoutShareNum].
	req.Scouts = gatherScouts(ctx, d, req, specs,
		cfg.RunDeadline*scoutShareNum/scoutShareDenom, log)

	result := d.Run(ctx, xreview.New(req))

	if err := report(cmd.String("out"), cmd.String("findings-out"),
		cmd.String("check-out"), result); err != nil {
		// The run's own outcome wins: ExitConfig here would relabel a review that
		// timed out as one rejected before it started.
		code := result.ExitCode
		if code == driver.ExitOK {
			code = driver.ExitConfig
		}
		return &exitError{code: code, err: err}
	}
	if result.ExitCode != driver.ExitOK {
		return &exitError{
			code: result.ExitCode,
			err:  fmt.Errorf("review finished with exit code %d", result.ExitCode),
		}
	}
	return nil
}

// report parses the driver's reply into a verdict, prints the machine-readable
// result, and when asked writes the verdict text for the caller to post.
//
// Parsing lives here rather than in the driver because what counts as an answer
// is the workload's. The driver attributes a reply to a turn and stops.
//
// The file is written only on a *structured* verdict, and the emphasis is the
// whole point. Its absence is how the caller tells "ready to post" from "nothing
// to post" — the calling workflow decides on the file existing, not on the exit
// code, because a non-zero exit can still carry a good review (a teardown leak,
// say). So the gate has to be the same thing the driver itself calls a verdict.
//
// Gating on non-empty text instead would post the prose from a run that reported
// [driver.ExitNoVerdict], upserting unparsed prose over a previous good review.
// That prose is deliberately not published; stdout carries the decision, the
// structured block and, when there is no verdict, the reason there is none.
func report(outPath, findingsPath, checkPath string, result driver.Result) error {
	payload := map[string]any{
		"session_id":  result.SessionID,
		"exit_code":   result.ExitCode,
		"teardown_ok": result.TeardownOK,
	}
	var verdict xreview.Verdict
	if result.Reply != nil {
		verdict = xreview.ParseVerdict(result.Reply.Text)
		verdict.TurnID = result.Reply.TurnID
		verdict.ItemID = result.Reply.ItemID
		payload["decision"] = verdict.Decision()
		payload["structured"] = verdict.Structured
		// Why there is no verdict is the actionable half of a no-verdict run, so it
		// travels in the payload rather than only in the logs. The driver's own
		// reason wins when it had one: it names a failure the text cannot show,
		// like a reply refused for carrying a credential.
		if !verdict.HasVerdict() {
			payload["reason"] = firstNonEmpty(result.Reply.Reason, verdict.Reason)
		}
	}
	blob, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering the result: %w", err)
	}
	fmt.Println(string(blob))

	// Before anything is written, on every path: a workspace can be reused, and a
	// caller posts on presence.
	if err := clearOutputs(outPath, findingsPath, checkPath); err != nil {
		return err
	}

	if !verdict.HasVerdict() {
		return nil
	}

	// Each output answers to its own flag. An absent check run reads as a review
	// that did not run rather than one that passed, so gating it on --out made the
	// fail-closed signal the one that failed open.
	if outPath != "" {
		body := xreview.RenderComment(verdict, result.SessionID)
		if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
			return fmt.Errorf("writing the verdict to %s: %w", outPath, err)
		}
	}
	if err := writeFindings(findingsPath, verdict); err != nil {
		return err
	}
	return writeCheckRun(checkPath, verdict)
}

// readPriorThreads loads what this tool said on this pull request before.
//
// An absent file is not an error: a pull request reviewed for the first time has
// no history, and that is the common case rather than a fault.
func readPriorThreads(path string) ([]xreview.PriorThread, error) {
	blob, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(bytes.TrimSpace(blob)) == 0 {
		return nil, nil
	}
	var threads []xreview.PriorThread
	if err := json.Unmarshal(blob, &threads); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return threads, nil
}

// writeCheckRun renders the check run the caller publishes.
//
// Written whenever there is a verdict, unlike the findings: a review that found
// nothing still concludes, and a checks list with no xreview entry reads as a
// review that did not run rather than one that passed.
func writeCheckRun(path string, verdict xreview.Verdict) error {
	if path == "" {
		return nil
	}
	check, ok := xreview.BuildCheckRun(verdict)
	if !ok {
		return nil
	}
	blob, err := json.MarshalIndent(check, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering the check run: %w", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		return fmt.Errorf("writing the check run to %s: %w", path, err)
	}
	return nil
}

// firstNonEmpty returns the first of its arguments that says something.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// clearOutputs removes any output left by an earlier run.
//
// The caller publishes on a file being present, so a stale one is a previous
// review published under this run's name. An absent file is not an error; a file
// that could not be removed is, because leaving it is the exact outcome this
// exists to prevent -- a read-only mount or a file owned by another uid on a
// reused workspace would otherwise hand the caller an earlier verdict.
//
// Every path is attempted and the failures are joined. The obligation is per file, so
// returning on the first one would leave the rest of an earlier run's outputs in place
// -- and the caller decides on each file's presence separately, so that is two stale
// publishes bought by reporting one error early.
func clearOutputs(paths ...string) error {
	var failed error
	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failed = errors.Join(failed,
				fmt.Errorf("clearing an earlier run's output at %s: %w", p, err))
		}
	}
	return failed
}

// writeFindings hands the caller the observations it can post against a line.
//
// Separate from the verdict text on purpose. That file's presence means "post
// this review"; this one's means "and place these on the code", and a review with
// nothing placeable is normal rather than a failure — so an empty list writes no
// file and the caller posts a summary alone.
func writeFindings(path string, verdict xreview.Verdict) error {
	if path == "" {
		return nil
	}
	findings := xreview.PlaceableFindings(verdict)
	if len(findings) == 0 {
		return nil
	}
	blob, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return fmt.Errorf("rendering the findings: %w", err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		return fmt.Errorf("writing the findings to %s: %w", path, err)
	}
	return nil
}

// parseTarget reads the repository and pull request from the positional
// arguments, rejecting shapes that would otherwise reach the API as a request for
// something else entirely.
func parseTarget(args []string) (string, int, error) {
	if len(args) != 2 {
		return "", 0, fmt.Errorf(
			"%w: expected <owner/name> <pr-number>, got %d argument(s)",
			driver.ErrConfig, len(args))
	}
	repo := args[0]
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", 0, fmt.Errorf("%w: repository must be \"owner/name\", got %q",
			driver.ErrConfig, repo)
	}
	// Checked for what it contains, not only its shape: this is written into the
	// shell commands the prompt hands the agent, so a quote or a substitution in it
	// would end the argument and start something else.
	if !isPlainRepoName(repo) {
		return "", 0, fmt.Errorf(
			"%w: repository has characters GitHub does not allow in an owner or name: %q",
			driver.ErrConfig, repo)
	}
	// Atoi, not Sscanf: Sscanf("%d") stops at the first non-digit and reports
	// success, so "4.9" would parse as 4 and this would review a different pull
	// request than the one asked for, silently. Atoi rejects the whole string.
	pr, err := strconv.Atoi(args[1])
	if err != nil || pr <= 0 {
		return "", 0, fmt.Errorf("%w: pull request must be a positive number, got %q",
			driver.ErrConfig, args[1])
	}
	return repo, pr, nil
}

// scoutShareNum over scoutShareDenom is how much of the run's deadline the scouts
// may spend between them.
//
// They run in parallel, so this is wall-clock for all of them together rather than
// each. It is not carved out of the review's budget: [driver.Driver.Run] applies
// the whole deadline to every session it drives, so a run costs at worst
// RunDeadline x (1 + this share) end to end, and the calling job's timeout has to
// cover that. Bounding the gather is still what keeps the review reachable — a
// review that heard from nobody still publishes, and a run that never reaches the
// review publishes nothing at all.
const (
	scoutShareNum   = 2
	scoutShareDenom = 5
)

// scoutSpec is one configured scout: the name findings are attributed to, and the
// agent bundle that fixes its harness.
type scoutSpec struct {
	name  string
	agent string
}

// isPlainRepoName reports whether s is an owner/name pair and nothing more.
//
// GitHub allows only letters, digits, hyphen, underscore and dot in an owner or a
// name, so anything else cannot be a repository this could review.
func isPlainRepoName(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-', r == '/':
		default:
			return false
		}
	}
	return true
}

// parseScouts reads the scout list, formatted "name=agent,name=agent".
//
// An empty value configures none, which is the solo review. A malformed entry is
// an error rather than a skip: a typo that silently drops a reader would publish a
// review that looks like it weighed opinions it never heard.
func parseScouts(raw, reviewAgent string) ([]scoutSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []scoutSpec
	seen := map[string]bool{}
	agents := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		name, agent, ok := strings.Cut(strings.TrimSpace(entry), "=")
		name, agent = strings.TrimSpace(name), strings.TrimSpace(agent)
		if !ok || name == "" || agent == "" {
			return nil, fmt.Errorf("%w: scout %q is not name=agent", driver.ErrConfig, entry)
		}
		if seen[name] {
			// Two scouts under one name would share a run key, so the second would
			// adopt the first's session and report its findings back as its own.
			return nil, fmt.Errorf("%w: scout %q is configured twice", driver.ErrConfig, name)
		}
		if agent == reviewAgent {
			// The bundle fixes the harness, so this is the review reading its own
			// diff and being shown the result as an independent opinion.
			return nil, fmt.Errorf("%w: scout %q runs the review's own agent %q, so its "+
				"reading would not be independent", driver.ErrConfig, name, agent)
		}
		if dup, ok := agents[agent]; ok {
			return nil, fmt.Errorf("%w: scouts %q and %q both run agent %q, so one opinion "+
				"would be counted twice", driver.ErrConfig, dup, name, agent)
		}
		seen[name], agents[agent] = true, name
		out = append(out, scoutSpec{name: name, agent: agent})
	}
	return out, nil
}

// gatherScouts runs the scouts and returns what each contributed, in dispatch
// order.
//
// A scout never fails the review. Every way one can go wrong — no such agent, no
// credential for its harness, a turn that never ends — becomes a note on that
// scout's result, and the review proceeds having heard from fewer readers. The
// note matters as much as the findings: a reading that failed must not arrive
// looking like a reading that found nothing, or an outage reads as a clean bill of
// health on every pull request at once.
//
// Results are written to a slot each, so dispatch order survives without a lock
// and the review sees the same order every run.
func gatherScouts(
	ctx context.Context,
	d *driver.Driver,
	req xreview.Request,
	specs []scoutSpec,
	budget time.Duration,
	log *slog.Logger,
) []xreview.ScoutResult {
	if len(specs) == 0 {
		return nil
	}
	log.Info("gathering independent readings", "scouts", len(specs), "budget", budget)

	// Bounded here rather than left to the driver, which applies the whole run
	// deadline per session.
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	out := make([]xreview.ScoutResult, len(specs))
	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = runScout(ctx, d, req, spec, log)
		}()
	}
	wg.Wait()

	for _, r := range out {
		if r.Failed() {
			log.Warn("scout contributed nothing", "scout", r.Name, "note", r.Note)
		} else {
			log.Info("scout reported", "scout", r.Name, "findings", len(r.Findings))
		}
	}
	return out
}

// scoutNote renders why a scout contributed nothing, from the driver's own
// classification.
//
// A fixed vocabulary rather than the error text. A driver error can carry a
// workspace URL, which [driver.Cloner] documents as credential-bearing, and this
// string reaches both a log line and a prompt whose answer is published verbatim
// to a public thread. The exit code already says what went wrong precisely enough
// for an operator to act on.
func scoutNote(result driver.Result) string {
	switch result.ExitCode {
	case driver.ExitConfig:
		return "its agent or credential is not configured on this server"
	case driver.ExitTimeout:
		return "it did not answer within its share of the deadline"
	case driver.ExitTurnFailed:
		return "its session reported the turn failed"
	case driver.ExitNoVerdict:
		return "it ended the turn without a readable report"
	case driver.ExitTransport:
		return "the connection to its session failed"
	default:
		return fmt.Sprintf("it ended with exit code %d", result.ExitCode)
	}
}

// runScout drives one scout and turns whatever happened into a result.
//
// The panic guard is what makes "a scout never fails the review" true rather than
// intended: a bare goroutine panicking takes the process with it, so one scout's
// bug would kill a review that had already gathered the others.
func runScout(
	ctx context.Context,
	d *driver.Driver,
	req xreview.Request,
	spec scoutSpec,
	log *slog.Logger,
) (res xreview.ScoutResult) {
	res = xreview.ScoutResult{Name: spec.name}
	defer func() {
		if p := recover(); p != nil {
			log.Error("scout panicked", "scout", spec.name, "panic", p)
			res = xreview.ScoutResult{Name: spec.name, Note: "it failed unexpectedly"}
		}
	}()

	result := d.Run(ctx, xreview.NewScout(req, spec.name, spec.agent))
	// The session id is logged even on failure: it names the sandbox, and nothing
	// else reclaims one, so this is the operator's only handle on a leak.
	log.Info("scout finished", "scout", spec.name, "agent", spec.agent,
		"session_id", result.SessionID, "exit_code", result.ExitCode)

	switch {
	case result.ExitCode != driver.ExitOK:
		// Every failure arrives as an exit code, so there is no second arm here that
		// a reader could forget to write.
		res.Note = scoutNote(result)
		return res
	case result.Reply == nil:
		res.Note = "it ended the turn without answering"
		return res
	case result.Reply.Reason != "":
		res.Note = "its reply was not usable"
		return res
	}

	parsed := xreview.ParseScoutReport(result.Reply.Text)
	switch {
	case !parsed.HasReport():
		res.Note = parsed.Reason
		return res
	case !parsed.Read():
		// It answered, and what it answered is that it never got the diff. That is
		// not a clean reading however empty its findings list.
		res.Note = "it reported reading no diff"
		return res
	}
	res.Findings = parsed.Findings
	return res
}
