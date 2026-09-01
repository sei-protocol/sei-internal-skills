package driver

import "testing"

// TestWorkForWithholdsTheModelFromAWorkloadOnItsOwnAgent is the rule that keeps a
// configured model off a scout.
//
// The model is chosen for the default agent. A scout runs on a different agent, which
// means a different harness and a different provider, and a model name one provider
// answers to is rejected by another at turn start. So the cost of forwarding it is not
// a scout on the wrong model — it is a scout that produced no reading at all, and a
// review that proceeds with fewer independent views and nothing saying why.
//
// Both directions on one configuration, because a rule that withheld the model from
// everything would pass a scout-only assertion while making the whole feature dead.
func TestWorkForWithholdsTheModelFromAWorkloadOnItsOwnAgent(t *testing.T) {
	t.Parallel()

	d := &Driver{cfg: Config{Agent: "seidroid", Model: "claude-opus-4-7"}}
	work := testWork{Repo: "sei-protocol/sandbox", PR: 31}

	review := d.workFor(work)
	if review.Agent != "seidroid" {
		t.Errorf("review agent = %q, want seidroid (the configured default)", review.Agent)
	}
	if review.Model != "claude-opus-4-7" {
		t.Errorf("review model = %q, want claude-opus-4-7", review.Model)
	}

	scout := d.workFor(namedWork{testWork: work, agent: "seidroid-codex"})
	if scout.Agent != "seidroid-codex" {
		t.Errorf("scout agent = %q, want seidroid-codex (its own)", scout.Agent)
	}
	if scout.Model != "" {
		t.Errorf("scout model = %q, want empty — a model chosen for one provider fails "+
			"at turn start on another, costing the reading", scout.Model)
	}
}

// TestWorkForKeepsTheModelWhenTheWorkloadNamesNoAgent covers the AgentNamer that
// declines to name one.
//
// [Driver.workFor] reads AgentName() and ignores an empty answer, falling back to the
// configured agent. The model has to fall back with it: a workload running on the
// default agent is the case the model was configured for, and keying the withholding on
// the interface rather than on the answer would strip it.
func TestWorkForKeepsTheModelWhenTheWorkloadNamesNoAgent(t *testing.T) {
	t.Parallel()

	d := &Driver{cfg: Config{Agent: "seidroid", Model: "claude-opus-4-7"}}
	got := d.workFor(namedWork{
		testWork: testWork{Repo: "sei-protocol/sandbox", PR: 32},
		agent:    "",
	})

	if got.Agent != "seidroid" {
		t.Errorf("agent = %q, want seidroid", got.Agent)
	}
	if got.Model != "claude-opus-4-7" {
		t.Errorf("model = %q, want claude-opus-4-7 — this work runs on the very agent "+
			"the model was configured for", got.Model)
	}
}
