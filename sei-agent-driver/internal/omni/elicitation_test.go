package omni

import (
	"testing"

	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// policyAllowed is the policy name these fixtures carry.
const policyAllowed = "release-approval"

// TestElicitationFromEventMapsAttestedAndModelFields checks the full mapping
// from a real SDK event, and separately that every optional pointer field
// being nil (the common case — most prompts carry no phase, no policy_name,
// no target session) decodes to the zero driver.Elicitation rather than panicking.
func TestElicitationFromEventMapsAttestedAndModelFields(t *testing.T) {
	t.Parallel()

	t.Run("every field populated", func(t *testing.T) {
		t.Parallel()
		phase := "pre_tool_use"
		policyName := policyAllowed
		mode := omnigent.ElicitationRequestParamsModeURL
		contentPreview := "truncated diff preview"
		targetSession := "conv_child_1"

		ev := omnigent.ElicitationRequestEvent{
			ElicitationID: "elicit_1",
			Type:          omnigent.ElicitationRequestEventTypeResponseElicitationRequest,
			Params: omnigent.ElicitationRequestParams{
				Message:              "Approve running this?",
				Phase:                &phase,
				PolicyName:           &policyName,
				Mode:                 &mode,
				ContentPreview:       &contentPreview,
				TargetSessionID:      &targetSession,
				AdditionalProperties: map[string]any{"tool_name": "Bash"},
			},
		}

		got := ElicitationFromEvent(ev)
		want := driver.Elicitation{
			ID:              "elicit_1",
			Phase:           phase,
			PolicyName:      policyName,
			Mode:            "url",
			ToolName:        "Bash",
			Message:         "Approve running this?",
			ContentPreview:  contentPreview,
			TargetSessionID: targetSession,
		}
		if got != want {
			t.Errorf("ElicitationFromEvent = %+v, want %+v", got, want)
		}
	})

	t.Run("nil optional pointers decode to zero values, not a panic", func(t *testing.T) {
		t.Parallel()
		ev := omnigent.ElicitationRequestEvent{
			ElicitationID: "elicit_2",
			Type:          omnigent.ElicitationRequestEventTypeResponseElicitationRequest,
			Params: omnigent.ElicitationRequestParams{
				Message: "hi",
				// Phase, PolicyName, Mode, ContentPreview, TargetSessionID and
				// AdditionalProperties are all left nil/unset.
			},
		}
		got := ElicitationFromEvent(ev)
		want := driver.Elicitation{ID: "elicit_2", Message: "hi"}
		if got != want {
			t.Errorf("ElicitationFromEvent = %+v, want %+v", got, want)
		}
	})
}

// TestElicitationFromSnapshotBothShapes covers the two wire shapes a
// PendingElicitations entry can take: today's nested-params event dict, and
// an older, flattened one where the same fields sit at the top level. Both
// must decode to the same driver.Elicitation, and a classification field holding a
// non-string JSON value must be read as absent rather than stringified into
// something that might accidentally match an allowlist.
func TestElicitationFromSnapshotBothShapes(t *testing.T) {
	t.Parallel()

	want := driver.Elicitation{
		ID:              "elicit_3",
		Phase:           "pre_tool_use",
		PolicyName:      policyAllowed,
		Mode:            "form",
		ToolName:        "Bash",
		Message:         "please approve",
		ContentPreview:  "prev",
		TargetSessionID: "conv_child_2",
	}

	t.Run("nested params, the current wire shape", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"elicitation_id": "elicit_3",
			"params": map[string]any{
				"phase":             "pre_tool_use",
				"policy_name":       policyAllowed,
				"mode":              "form",
				"tool_name":         "Bash",
				"message":           "please approve",
				"content_preview":   "prev",
				"target_session_id": "conv_child_2",
			},
		}
		if got := ElicitationFromSnapshot(raw); got != want {
			t.Errorf("ElicitationFromSnapshot(nested) = %+v, want %+v", got, want)
		}
	})

	t.Run("flattened, the legacy wire shape", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"elicitation_id":    "elicit_3",
			"phase":             "pre_tool_use",
			"policy_name":       policyAllowed,
			"mode":              "form",
			"tool_name":         "Bash",
			"message":           "please approve",
			"content_preview":   "prev",
			"target_session_id": "conv_child_2",
		}
		if got := ElicitationFromSnapshot(raw); got != want {
			t.Errorf("ElicitationFromSnapshot(flat) = %+v, want %+v", got, want)
		}
	})

	t.Run("id falls back to the bare id key", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{"id": "elicit_4", "policy_name": policyAllowed, "message": "hi"}
		got := ElicitationFromSnapshot(raw)
		if got.ID != "elicit_4" {
			t.Errorf("ID = %q, want %q", got.ID, "elicit_4")
		}
	})

	t.Run("nested params win over a conflicting top-level value", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"elicitation_id": "elicit_5",
			"policy_name":    "flat-value",
			"params": map[string]any{
				"policy_name": "nested-value",
			},
		}
		got := ElicitationFromSnapshot(raw)
		if got.PolicyName != "nested-value" {
			t.Errorf("PolicyName = %q, want %q (nested params must win)", got.PolicyName, "nested-value")
		}
	})

	t.Run("a non-string classification value is treated as absent, not coerced", func(t *testing.T) {
		t.Parallel()
		raw := map[string]any{
			"elicitation_id": "elicit_6",
			"params": map[string]any{
				"policy_name": 12345, // a number where the wire contract says a string
			},
		}
		got := ElicitationFromSnapshot(raw)
		if got.PolicyName != "" {
			t.Errorf("PolicyName = %q, want empty: a non-string value must not be stringified into a classifiable name", got.PolicyName)
		}

		// And it must actually decline, exactly as if policy_name (and
		// tool_name) had never been sent.
		p := driver.Policy{AllowPolicies: map[string]bool{"12345": true}}
		if action, reason := p.Decide(got); action != driver.Decline {
			t.Errorf("Decide = %q (%s), want driver.Decline: a coerced \"12345\" would wrongly match the allowlist", action, reason)
		}
	})
}
