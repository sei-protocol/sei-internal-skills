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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
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

func main() {
	// Logs go to stderr so stdout carries only the verdict payload and a caller
	// can consume one without parsing around the other.
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

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
		Usage:     "review a pull request with the sei-droid agent",
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
			&cli.BoolFlag{
				Name: "close",
				Usage: "delete this pull request's session instead of reviewing; " +
					"the only thing that reclaims its sandbox",
			},
			&cli.StringFlag{
				Name: "trigger-id",
				Usage: "idempotency key for this dispatch, e.g. the triggering " +
					"comment id; defaults to the workflow run and attempt",
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

	// A terminate signal cancels the context rather than killing the process, so
	// the driver's teardown still runs and the sandbox is deleted. A second
	// signal is not caught, which is the operator's override when teardown itself
	// is what is hanging.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	policy := driver.NewPolicy(
		os.Getenv("XREVIEW_ALLOW_POLICIES"),
		os.Getenv("XREVIEW_ALLOW_TOOLS"),
	)
	if len(policy.AllowPolicies) == 0 && len(policy.AllowTools) == 0 {
		log.Warn("no permission allowlist configured; every prompt will be declined",
			"hint", "set XREVIEW_ALLOW_POLICIES to the policy_name values this run logs")
	}

	work := xreview.New(xreview.Request{
		Repo: repo,
		PR:   pr,
		Trigger: xreview.TriggerID(
			cmd.String("trigger-id"),
			os.Getenv("GITHUB_RUN_ID"),
			os.Getenv("GITHUB_RUN_ATTEMPT"),
			repo, pr,
		),
	})

	d := driver.NewDriver(cfg, policy, log)

	// --close ends the unit of work. Reviewing keeps the conversation so the next
	// invocation builds on it; this is what finally destroys it, and with it the
	// sandbox the Kubernetes launcher cannot otherwise reclaim.
	if cmd.Bool("close") {
		result, err := d.DeleteSession(ctx, work)
		if err != nil {
			return &exitError{code: result.ExitCode, err: err}
		}
		if err := report("", "", result); err != nil {
			return &exitError{code: driver.ExitConfig, err: err}
		}
		if result.ExitCode != driver.ExitOK {
			return &exitError{code: result.ExitCode,
				err: fmt.Errorf("close finished with exit code %d", result.ExitCode)}
		}
		return nil
	}

	result, err := d.Run(ctx, work)
	if err != nil {
		return &exitError{code: result.ExitCode, err: err}
	}

	if err := report(cmd.String("out"), cmd.String("findings-out"), result); err != nil {
		return &exitError{code: driver.ExitConfig, err: err}
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
func report(outPath, findingsPath string, result driver.Result) error {
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

	if outPath == "" || !verdict.HasVerdict() {
		return nil
	}
	body := xreview.RenderComment(verdict, result.SessionID)
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("writing the verdict to %s: %w", outPath, err)
	}
	return writeFindings(findingsPath, verdict)
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
