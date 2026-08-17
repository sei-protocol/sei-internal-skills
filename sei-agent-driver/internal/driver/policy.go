package driver

import "strings"

// Action is a verdict on one permission prompt.
type Action string

const (
	// Accept authorises the parked operation. It lets the agent run something it
	// otherwise could not, with the session owner's execution identity, so it is
	// only ever returned on an exact match against operator-supplied
	// configuration.
	Accept Action = "accept"

	// Decline refuses. This is what every unrecognised prompt gets.
	Decline Action = "decline"
)

// Elicitation is one permission prompt, reduced to the fields a decision may
// legitimately rest on.
//
// The split that matters is attested versus model-influenced. Phase, PolicyName
// and Mode are stamped by the server's policy engine. Message and
// ContentPreview originate with the model and are truncated by the server, so
// they are carried for logging and are never classified on — deciding from a
// model-chosen string hands the decision to the thing being gated.
type Elicitation struct {
	// ID is the correlation key the verdict is sent back with. An empty ID
	// cannot be answered at all.
	ID string

	// Phase is the policy phase that fired, e.g. a tool-call phase. Attested.
	Phase string

	// PolicyName is the policy that parked the turn. Attested, and one of the two
	// fields this package classifies on — see [Policy.AllowTools] for why
	// tool_name is the one to prefer.
	PolicyName string

	// Mode is the prompt's rendering mode, "url" or "form".
	Mode string

	// ToolName is the gated tool's registered name, when a harness supplies one.
	//
	// This deployment does send it, e.g. "Bash" on a shell prompt. The
	// SDK does not model the field, so it is read out of the elicitation params'
	// passthrough extras rather than a named struct member, and stays empty on a
	// harness that omits it.
	//
	// It is the finest-grained attested field available here, which makes it the
	// allowlist to prefer: see [Policy.AllowPolicies] for why policy_name is not.
	ToolName string

	// Message is the prompt text shown to a human. Model-influenced. Logged,
	// never classified on.
	Message string

	// ContentPreview is a truncated preview of the gated content.
	// Model-influenced. Logged, never classified on.
	ContentPreview string

	// TargetSessionID is set when the prompt is mirrored from another session,
	// in which case the verdict belongs to that session rather than the one the
	// stream belongs to.
	TargetSessionID string
}

// ResolveSession returns the session the verdict must be sent to, which is the
// mirroring target when there is one and otherwise the session in hand.
func (e Elicitation) ResolveSession(streamSession string) string {
	if e.TargetSessionID != "" {
		return e.TargetSessionID
	}
	return streamSession
}

// Policy decides each permission prompt.
//
// It denies by default and accepts only on an exact match against an allowlist
// an operator supplied. There is no substring, prefix or pattern matching
// anywhere in it, deliberately: a prompt raised while the agent reads a pull
// request is raised over content the agent was asked to read, so any matcher
// that a request's own text can influence is a matcher the reviewed content can
// steer.
//
// The zero Policy declines everything, and an all-declining policy is why a
// review turn cannot run the read operations it needs — the agent reports that it
// could not read the diff, which is a poor review rather than a loud failure.
//
// Unblocking it means allowing on an attested field, and the two available fields
// are not equivalent. See [Policy.AllowTools] and [Policy.AllowPolicies].
type Policy struct {
	// AllowPolicies are exact policy_name values to accept.
	//
	// Coarser than it looks on this deployment, and the difference matters: every
	// native permission prompt carries the same policy_name,
	// "claude_native_permission", because the field names the policy that fired
	// rather than the action it gated. Allowlisting that one value therefore
	// accepts every tool call the agent makes. Prefer [Policy.AllowTools], which
	// discriminates per tool.
	AllowPolicies map[string]bool

	// AllowTools are exact tool_name values to accept, e.g. "Bash". This is the
	// allowlist to reach for first: the field is per-tool, so it is the narrowest
	// attested discriminator this policy has.
	//
	// It does not amount to a read-only guarantee. "Bash" is arbitrary shell, so
	// allowing it allows a push as readily as a diff read. The read-only posture
	// rests on the prompt's instruction and the server-side gate, not on this.
	AllowTools map[string]bool
}

// NewPolicy builds a [Policy] from comma-separated allowlists, as the
// environment supplies them. Blank entries and surrounding spaces are dropped;
// an empty string yields an empty allowlist, which declines everything.
func NewPolicy(policies, tools string) Policy {
	return Policy{
		AllowPolicies: parseSet(policies),
		AllowTools:    parseSet(tools),
	}
}

// Decide returns the verdict for one prompt, and the reason, which is logged so
// an operator can see which rule fired rather than only what happened.
//
// A prompt with no ID is declined: without a correlation key there is nothing to
// answer, and accepting an unidentifiable prompt is the one case where a
// mistake cannot even be traced afterwards.
func (p Policy) Decide(e Elicitation) (Action, string) {
	switch {
	case e.ID == "":
		return Decline, "no elicitation id"
	case e.PolicyName != "" && p.AllowPolicies[e.PolicyName]:
		return Accept, "policy_name allowlisted: " + e.PolicyName
	case e.ToolName != "" && p.AllowTools[e.ToolName]:
		return Accept, "tool_name allowlisted: " + e.ToolName

	// Nothing attested to classify on: a harness stamping neither field, rather
	// than anything this driver can act on.
	case e.PolicyName == "" && e.ToolName == "":
		return Decline, "no attested policy_name or tool_name to match"
	default:
		return Decline, "not allowlisted"
	}
}

func parseSet(csv string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(csv, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out[v] = true
		}
	}
	return out
}
