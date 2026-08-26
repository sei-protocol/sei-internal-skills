package omni

import (
	omnigent "github.com/sei-protocol/omnigent-go-sdk"

	"github.com/sei-protocol/sei-internal-skills/sei-agent-driver/internal/driver"
)

// The two ways a permission prompt reaches this driver, reduced to the one shape
// a decision is made on.
//
// Two, because the stream replays nothing. A prompt raised while nobody was
// subscribed is readable only from the session snapshot, and a run that adopts an
// existing session is exactly the one that finds prompts waiting there.

// elicitationFromEvent reduces a streamed request event to a [driver.Elicitation].
func elicitationFromEvent(ev omnigent.ElicitationRequestEvent) driver.Elicitation {
	p := ev.Params
	return driver.Elicitation{
		ID:              ev.ElicitationID,
		Phase:           deref(p.Phase),
		PolicyName:      deref(p.PolicyName),
		Mode:            modeString(p.Mode),
		ToolName:        stringAt(p.AdditionalProperties, "tool_name"),
		Message:         p.Message,
		ContentPreview:  deref(p.ContentPreview),
		TargetSessionID: deref(p.TargetSessionID),
	}
}

// elicitationFromSnapshot reduces one entry of
// [omnigent.SessionResponse.PendingElicitations] to a [driver.Elicitation].
//
// The snapshot carries prompts raised while nobody was subscribed, which the
// stream does not replay, so a run that adopts an existing session has to read
// them from here. They are the original event dicts and arrive untyped, and
// older ones flatten the params rather than nesting them — so each field is
// looked up in params first and then at the top level, and either shape yields
// the same value.
func elicitationFromSnapshot(raw map[string]any) driver.Elicitation {
	params, _ := raw["params"].(map[string]any)
	pick := func(key string) string {
		if v := stringAt(params, key); v != "" {
			return v
		}
		return stringAt(raw, key)
	}
	id := stringAt(raw, "elicitation_id")
	if id == "" {
		id = stringAt(raw, "id")
	}
	if id == "" {
		id = pick("elicitation_id")
	}
	return driver.Elicitation{
		ID:              id,
		Phase:           pick("phase"),
		PolicyName:      pick("policy_name"),
		Mode:            pick("mode"),
		ToolName:        pick("tool_name"),
		Message:         pick("message"),
		ContentPreview:  pick("content_preview"),
		TargetSessionID: pick("target_session_id"),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func modeString(m *string) string {
	if m == nil {
		return ""
	}
	return *m
}

// stringAt reads a string out of an untyped map, returning "" for a missing key
// or a value of any other type. A non-string where a string belongs is treated
// as absent rather than coerced, so a number or object in a classification field
// declines instead of stringifying into something that might match.
func stringAt(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}
