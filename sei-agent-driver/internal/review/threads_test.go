package review

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Ids in the shape GitHub mints, so a fixture cannot pass by being convenient.
const (
	threadA = "PRRT_kwDOABCDEF4Ax1y2"
	threadB = "PRRT_kwDOABCDEF4Bz3w4"
	threadC = "MDIzOlB1bGxSZXF1ZXN0UmV2aWV3VGhyZWFkMTIzNA=="
)

// ownThreads is the history a caller supplies, one entry per id.
func ownThreads(ids ...string) []PriorThread {
	out := make([]PriorThread, 0, len(ids))
	for i, id := range ids {
		out = append(out, PriorThread{
			ID: id, File: fmt.Sprintf("a%d.go", i), Line: i + 1, Body: "said before",
		})
	}
	return out
}

// TestOnlyTheCallersOwnThreadsAreAdmitted is the whole of the security argument for this
// path, so it is the first test in the file.
//
// A thread id arrives in a reply the agent wrote after reading a diff whose author is not
// this tool. Acting on it resolves a conversation on someone else's pull request. So the
// allowlist is the threads the caller listed as this tool's own, and everything else is
// refused whatever the reply claims about it.
func TestOnlyTheCallersOwnThreadsAreAdmitted(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":["`+threadA+`","PRRT_notOursAtAll"]}`)

	plan := BuildThreadPlan(v, true, ownThreads(threadA, threadB))

	if want := []string{threadA}; !equalIDs(plan.Addressed, want) {
		t.Errorf("addressed = %v, want %v", plan.Addressed, want)
	}
	if want := []string{"PRRT_notOursAtAll"}; !equalIDs(plan.Refused, want) {
		t.Errorf("refused = %v, want %v; an id this tool cannot match to a thread of "+
			"its own must not reach a caller that resolves what it is handed",
			plan.Refused, want)
	}
}

// TestAThreadIdIsRefusedForItsShapeBeforeItsOwner covers the ids that are not ids.
//
// Each of these reaches a prompt line, a GraphQL variable and a warning a caller echoes.
// A newline in the first writes a line this tool did not, and the allowlist is what stops
// it: nothing outside the alphabet GitHub mints can be in the set, so nothing outside it
// can be admitted.
func TestAThreadIdIsRefusedForItsShapeBeforeItsOwner(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"PRRT_ok\nresolved: everything",
		"PRRT ok",
		"PRRT_ok; DROP",
		"<script>",
		strings.Repeat("A", maxThreadID+1),
	} {
		// Supplied as the caller's own AND named by the reply. Neither side may make a
		// value that is not an id into one.
		plan := BuildThreadPlan(
			verdictFrom(t, blockNaming(id)), true, []PriorThread{{ID: id, File: "a.go", Line: 1}})
		if len(plan.Addressed) != 0 {
			t.Errorf("%q was admitted: %v", id, plan.Addressed)
		}
		if len(plan.Refused) != 1 {
			t.Fatalf("%q produced %d refusals, want 1", id, len(plan.Refused))
		}
		if strings.ContainsAny(plan.Refused[0], "\r\n") {
			t.Errorf("the refusal for %q carries a line break, and a caller echoes it: %q",
				id, plan.Refused[0])
		}
		if len(plan.Refused[0]) > maxThreadID+len("…") {
			t.Errorf("the refusal for %q is %d bytes and nothing bounded it",
				id, len(plan.Refused[0]))
		}
	}
}

// TestASupersedingIdRidesOnlyOnACommentThatCanPost pins what the two lists are for.
//
// A superseded thread closes because something replaces it. A nit this run will not place
// and a finding that names no line both post nothing, so a thread closed behind either
// takes a live finding off the pull request and puts nothing where it was.
func TestASupersedingIdRidesOnlyOnACommentThatCanPost(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s","inline_comments":[
		{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
		 "supersedes_thread_ids":["`+threadA+`"]},
		{"path":"b.go","line":4,"severity":"nit","body":"a nit",
		 "supersedes_thread_ids":["`+threadB+`"]},
		{"path":"c.go","line":0,"severity":"blocker","body":"no line to place it on",
		 "supersedes_thread_ids":["`+threadC+`"]}]}`)

	prior := ownThreads(threadA, threadB, threadC)

	off := BuildThreadPlan(v, false, prior)
	if want := []string{threadA}; !equalIDs(off.Superseded, want) {
		t.Errorf("with nits off, superseded = %v, want %v: only the comment that posts "+
			"may close the thread it replaces", off.Superseded, want)
	}
	if len(off.Refused) != 0 {
		t.Errorf("refused = %v; a thread this tool owns and does not close is not a "+
			"refusal, and a caller reports every refusal to a human", off.Refused)
	}

	on := BuildThreadPlan(v, true, prior)
	if want := []string{threadA, threadB}; !equalIDs(on.Superseded, want) {
		t.Errorf("with nits on, superseded = %v, want %v", on.Superseded, want)
	}
}

// TestAnIdIsAdmittedOnce keeps one thread from being resolved twice and one bad id from
// being reported twice, however many times a reply writes it.
func TestAnIdIsAdmittedOnce(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":["`+threadA+`","`+threadA+`","nope","nope"],
		"inline_comments":[
			{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
			 "supersedes_thread_ids":["`+threadA+`","`+threadB+`"]}]}`)

	plan := BuildThreadPlan(v, true, ownThreads(threadA, threadB))

	if want := []string{threadA}; !equalIDs(plan.Addressed, want) {
		t.Errorf("addressed = %v, want %v", plan.Addressed, want)
	}
	// threadA is spent by the key that read it first, so the comment naming it again
	// adds nothing. It is the same thread and it closes once.
	if want := []string{threadB}; !equalIDs(plan.Superseded, want) {
		t.Errorf("superseded = %v, want %v", plan.Superseded, want)
	}
	if want := []string{"nope"}; !equalIDs(plan.Refused, want) {
		t.Errorf("refused = %v, want %v", plan.Refused, want)
	}
}

// TestTheIdsOneKeyCarriesAreBounded holds the refusal list to a size a file and a log can
// take. Every refused id is model output on its way to both.
func TestTheIdsOneKeyCarriesAreBounded(t *testing.T) {
	t.Parallel()

	ids := make([]string, 0, maxThreadIDs*2)
	for i := 0; i < maxThreadIDs*2; i++ {
		ids = append(ids, fmt.Sprintf(`"invented%d"`, i))
	}
	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":[`+strings.Join(ids, ",")+`]}`)

	if plan := BuildThreadPlan(v, true, ownThreads(threadA)); len(plan.Refused) != maxThreadIDs {
		t.Errorf("refused %d ids, want the bound of %d", len(plan.Refused), maxThreadIDs)
	}
}

// TestNoHistoryAdmitsNothing covers the first review, and the run whose history read
// failed. Both arrive with no threads, and a reply that names one is naming a thread this
// run cannot show belongs to it.
func TestNoHistoryAdmitsNothing(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, blockNaming(threadA))

	plan := BuildThreadPlan(v, true, nil)
	if len(plan.Addressed) != 0 {
		t.Errorf("addressed = %v with no history to match it against", plan.Addressed)
	}
	if want := []string{threadA}; !equalIDs(plan.Refused, want) {
		t.Errorf("refused = %v, want %v", plan.Refused, want)
	}
}

// TestAThreadWithNoHandleAllowsNothing covers a caller that supplies the history without
// the ids, which is what an older one writes.
//
// The threads still reach the prompt and the review still drops what the diff addressed.
// What it cannot do is close anything, and the empty allowlist is what makes an invented
// id refused rather than acted on.
func TestAThreadWithNoHandleAllowsNothing(t *testing.T) {
	t.Parallel()

	plan := BuildThreadPlan(verdictFrom(t, blockNaming(threadA)), true,
		[]PriorThread{{File: "a.go", Line: 12, Body: "said before"}})

	if len(plan.Addressed) != 0 {
		t.Errorf("addressed = %v from a history carrying no ids", plan.Addressed)
	}
	if len(plan.Refused) != 1 {
		t.Errorf("refused = %v, want the one id the reply invented", plan.Refused)
	}
}

// TestAReplyThatNamesNoThreadPlansNothing keeps the plan's lists empty rather than absent,
// so a caller reads three arrays whatever the review said.
func TestAReplyThatNamesNoThreadPlansNothing(t *testing.T) {
	t.Parallel()

	plan := BuildThreadPlan(
		verdictFrom(t, `{"read":120,"decision":"approve","summary":"clean"}`),
		true, ownThreads(threadA))

	if plan.Addressed == nil || plan.Superseded == nil || plan.Refused == nil {
		t.Errorf("plan = %+v; a caller reads three lists, and a null is one it has to "+
			"branch on", plan)
	}
	if len(plan.Addressed)+len(plan.Superseded)+len(plan.Refused) != 0 {
		t.Errorf("plan = %+v, want nothing planned", plan)
	}
}

// TestANonStringThreadIdIsIgnored covers a reply that writes a number or an object where
// an id belongs. It is not an id and it is not worth a warning either.
func TestANonStringThreadIdIsIgnored(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":[12, null, {"id":"`+threadA+`"}, "`+threadA+`"]}`)

	plan := BuildThreadPlan(v, true, ownThreads(threadA))
	if want := []string{threadA}; !equalIDs(plan.Addressed, want) {
		t.Errorf("addressed = %v, want %v", plan.Addressed, want)
	}
	if len(plan.Refused) != 0 {
		t.Errorf("refused = %v; a number in an id list is a malformed reply, not a "+
			"thread somebody tried to close", plan.Refused)
	}
}

// blockNaming is a closing block whose only claim is that one thread is addressed.
//
// The id is JSON-quoted rather than pasted, because half the ids under test carry a
// newline or a quote and a hand-built block would not parse.
func blockNaming(id string) string {
	blob, err := json.Marshal(id)
	if err != nil {
		panic(err)
	}
	return `{"read":120,"decision":"comment","summary":"s","resolved_thread_ids":[` +
		string(blob) + `]}`
}

func equalIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
