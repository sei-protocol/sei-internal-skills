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
//
// Which list an id lands in when a reply names it under both keys is the part worth
// pinning, and Superseded is the safe answer rather than the arbitrary one. Addressed is
// spent whenever the review publishes; Superseded is spent only once a comment replaced
// the finding. A reply that contradicts itself has to resolve somewhere, and under
// Addressed it would close a thread on publication alone -- so if placement then failed,
// a live finding would come off the pull request with nothing where it was. Held to
// Superseded, the same contradiction costs a thread left open, which is what this
// workflow does today.
//
// Nothing in the prompt forbids naming an id twice and the reply is untrusted, so this
// is a shape that arrives rather than one a well-behaved model avoids.
func TestAnIdNamedTwiceIsHeldToTheStricterGate(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":["`+threadA+`","`+threadA+`","nope","nope"],
		"inline_comments":[
			{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
			 "supersedes_thread_ids":["`+threadA+`","`+threadB+`"]}]}`)

	plan := BuildThreadPlan(v, true, ownThreads(threadA, threadB))

	if want := []string{threadA, threadB}; !equalIDs(plan.Superseded, want) {
		t.Errorf("superseded = %v, want %v: an id under both keys closes only once its "+
			"replacement is on the code", plan.Superseded, want)
	}
	if len(plan.Addressed) != 0 {
		t.Errorf("addressed = %v; threadA is named under both keys, and closing it on "+
			"publication alone is what the split exists to prevent", plan.Addressed)
	}
	if want := []string{"nope"}; !equalIDs(plan.Refused, want) {
		t.Errorf("refused = %v, want %v", plan.Refused, want)
	}
}

// TestAnIdNamedTwiceOnAnUnplaceableCommentClosesNothing is the other half of the rule.
//
// The stricter gate is only worth reaching if it actually holds. A comment this run will
// not place supersedes nothing, so an id named there and in resolved_thread_ids has to
// close under neither -- falling back to Addressed would restore exactly the behaviour
// the test above refuses.
func TestAnIdNamedTwiceOnAnUnplaceableCommentClosesNothing(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":["`+threadA+`"],
		"inline_comments":[
			{"path":"a.go","line":12,"severity":"nit","body":"a nit",
			 "supersedes_thread_ids":["`+threadA+`"]}]}`)

	// Nits off, so the comment naming threadA is never placed.
	plan := BuildThreadPlan(v, false, ownThreads(threadA))

	if len(plan.Addressed) != 0 || len(plan.Superseded) != 0 {
		t.Errorf("plan = %+v; the only comment naming this thread will not be placed, "+
			"so nothing replaces it and it stays open", plan)
	}
}

// TestTheRefusalListIsBoundedOverTheWholePlan holds the reported refusals to a size a
// file and a log can take, counting every key rather than each one on its own.
//
// Per key is not a bound here. supersedes_thread_ids is read once per placeable finding,
// so a per-key bound alone admits maxThreadIDs × maxPlaceableFindings refusals — about a
// megabyte of model output written into the check file and echoed line by line by a
// caller that warns on each. This fixture is that shape: every finding names a full key
// of invented ids, and none of them is a thread anybody owns.
func TestTheRefusalListIsBoundedOverTheWholePlan(t *testing.T) {
	t.Parallel()

	comments := make([]string, 0, maxPlaceableFindings)
	for f := 0; f < maxPlaceableFindings; f++ {
		ids := make([]string, 0, maxThreadIDs)
		for i := 0; i < maxThreadIDs; i++ {
			ids = append(ids, fmt.Sprintf(`"invented%d-%d"`, f, i))
		}
		comments = append(comments, fmt.Sprintf(
			`{"path":"a%d.go","line":%d,"severity":"blocker","body":"b%d",`+
				`"supersedes_thread_ids":[%s]}`,
			f, f+1, f, strings.Join(ids, ",")))
	}
	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"inline_comments":[`+strings.Join(comments, ",")+`]}`)

	plan := BuildThreadPlan(v, true, ownThreads(threadA))

	if len(plan.Refused) != maxRefusedThreadIDs {
		t.Errorf("refused list holds %d ids, want the bound of %d",
			len(plan.Refused), maxRefusedThreadIDs)
	}
	// Bounded is not the same as silent. A caller reading the length alone would report
	// twenty over a reply that named thousands, and those are different problems.
	if want := maxThreadIDs * maxPlaceableFindings; plan.RefusedTotal != want {
		t.Errorf("refused_total = %d, want %d; what the bound leaves out is counted "+
			"rather than dropped", plan.RefusedTotal, want)
	}
	bytes := 0
	for _, id := range plan.Refused {
		bytes += len(id)
	}
	if bytes > maxRefusedThreadIDs*(maxThreadID+8) {
		t.Errorf("the refusal list is %d bytes, which is not a bound a log can take", bytes)
	}
}

// TestARefusedIdCarriesNothingATerminalObeys covers the bytes a refusal is echoed with.
//
// A refused id reaches a workflow annotation and a log, and a terminal reads more than
// text: ESC opens an OSC 8 hyperlink, and BEL and DEL rewrite what a reader sees. So a
// refusal that renders as "this id was not resolved" must not also render as a link
// somebody else wrote. oneLine does not stop any of these — it splits on unicode space,
// and a C0 control is not one — which is why the filter is the id alphabet instead.
func TestARefusedIdCarriesNothingATerminalObeys(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"PRRT_\x1b]8;;http://evil.example\x07click here\x1b]8;;\x07",
		"PRRT_\x00nul",
		"PRRT_\x07bel",
		"PRRT_\x7fdel",
		"PRRT_\rcarriage",
		"PRRT_ok not really",
	} {
		plan := BuildThreadPlan(verdictFrom(t, blockNaming(id)), true, ownThreads(threadA))
		if len(plan.Refused) != 1 {
			t.Fatalf("%q produced %d refusals, want 1", id, len(plan.Refused))
		}
		got := plan.Refused[0]
		for _, r := range got {
			if r == '…' {
				continue // clip's own marker, and printable
			}
			if r == '?' {
				continue // what this replaces a byte outside the alphabet with
			}
			if !threadIDRune(r) {
				t.Errorf("the refusal for %q carries %q, which is not a character an "+
					"id is made of: %q", id, r, got)
			}
		}
		// The length still says how much was written, so a reader can tell a mangled id
		// from a short one.
		if len([]rune(got)) != len([]rune(id)) {
			t.Errorf("the refusal for %q is %d runes against %d written; a dropped byte "+
				"reads as a shorter id rather than as a mangled one",
				id, len([]rune(got)), len([]rune(id)))
		}
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

// TestEachPlacedCommentCarriesOnlyItsOwnSupersededThreads is the per-thread half of the
// gate, and the reason the linkage is published at all.
//
// A caller closes a superseded thread when the comment replacing it reached the code. A
// review superseding three threads places three comments, and any one of them can fail on
// its own. So each comment has to name the thread it replaces and no other: handed the
// review's whole set, a caller closes all three on the strength of whichever one posted,
// and the other two authors read a resolved thread with nothing on the diff where it was.
func TestEachPlacedCommentCarriesOnlyItsOwnSupersededThreads(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s","inline_comments":[
		{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
		 "supersedes_thread_ids":["`+threadA+`"]},
		{"path":"b.go","line":4,"severity":"blocker","body":"also still wrong",
		 "supersedes_thread_ids":["`+threadB+`"]},
		{"path":"c.go","line":8,"severity":"suggestion","body":"and this",
		 "supersedes_thread_ids":["`+threadC+`"]}]}`)

	got := PlaceableFindings(v, true, ownThreads(threadA, threadB, threadC))

	if len(got) != 3 {
		t.Fatalf("placeable findings = %d, want 3: %+v", len(got), got)
	}
	for i, want := range [][]string{{threadA}, {threadB}, {threadC}} {
		if !equalIDs(got[i].Supersedes, want) {
			t.Errorf("%s:%d supersedes %v, want %v: a comment carrying another comment's "+
				"thread closes that thread on this one posting",
				got[i].File, got[i].Line, got[i].Supersedes, want)
		}
	}
}

// TestAPublishedLinkageNamesOnlyTheCallersOwnThreads carries the allowlist across into
// the file.
//
// A linkage is model output that decides a mutation on somebody's pull request, and the
// file it lands in is read by a workflow that resolves what it is handed. So the ids that
// reach it are the caller's own, and an invented one is still reported: a review naming
// threads that do not exist is worth an operator's attention whether or not it closed
// anything.
func TestAPublishedLinkageNamesOnlyTheCallersOwnThreads(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s","inline_comments":[
		{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
		 "supersedes_thread_ids":["`+threadA+`","PRRT_notOursAtAll"]}]}`)

	prior := ownThreads(threadA)

	got := PlaceableFindings(v, true, prior)
	if len(got) != 1 {
		t.Fatalf("placeable findings = %d, want 1: %+v", len(got), got)
	}
	if want := []string{threadA}; !equalIDs(got[0].Supersedes, want) {
		t.Errorf("supersedes = %v, want %v", got[0].Supersedes, want)
	}

	plan := BuildThreadPlan(v, true, prior)
	if want := []string{"PRRT_notOursAtAll"}; !equalIDs(plan.Refused, want) {
		t.Errorf("refused = %v, want %v: filtering the linkage must not swallow the "+
			"refusal, which is the only place a human learns of an invented id",
			plan.Refused, want)
	}
}

// TestTheLinkageSpendsExactlyThePlansSupersededSet pins the two files against each other.
//
// [ThreadPlan.Superseded] is a caller's warrant to close a thread, and the linkage is
// which comment spends it. Two derivations of one allowlist can disagree, and both
// directions are defects: a linkage naming a thread the plan does not closes one the
// driver refused, and a plan entry no comment claims is a thread nothing can ever close.
//
// The shapes that could split them are all here: a nit this run drops, a finding with no
// line, one thread claimed by two comments, and one id named under both keys.
func TestTheLinkageSpendsExactlyThePlansSupersededSet(t *testing.T) {
	t.Parallel()

	const threadD = "PRRT_kwDOABCDEF4Dq9r8"
	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s",
		"resolved_thread_ids":["`+threadD+`"],
		"inline_comments":[
			{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
			 "supersedes_thread_ids":["`+threadA+`","`+threadD+`"]},
			{"path":"b.go","line":4,"severity":"nit","body":"a nit",
			 "supersedes_thread_ids":["`+threadB+`"]},
			{"path":"c.go","line":0,"severity":"blocker","body":"no line to place it on",
			 "supersedes_thread_ids":["`+threadC+`"]},
			{"path":"d.go","line":7,"severity":"suggestion","body":"the same thread again",
			 "supersedes_thread_ids":["`+threadA+`"]}]}`)

	prior := ownThreads(threadA, threadB, threadC, threadD)

	for _, includeNits := range []bool{false, true} {
		plan := BuildThreadPlan(v, includeNits, prior)
		linked := make([]string, 0, len(plan.Superseded))
		seen := make(map[string]bool)
		for _, f := range PlaceableFindings(v, includeNits, prior) {
			for _, id := range f.Supersedes {
				if seen[id] {
					continue
				}
				seen[id] = true
				linked = append(linked, id)
			}
		}
		if !equalIDs(linked, plan.Superseded) {
			t.Errorf("include-nits %v: the linkage names %v and the plan names %v",
				includeNits, linked, plan.Superseded)
		}
		if len(plan.Addressed) != 0 {
			t.Errorf("include-nits %v: addressed = %v; an id a comment claims closes on "+
				"that comment posting, never on publication alone",
				includeNits, plan.Addressed)
		}
	}
}

// TestAFindingThisRunWillNotPlacePublishesNoLinkage is the same rule read off the file.
//
// A nit the run drops posts no comment, so it replaces nothing. It is absent from the
// findings a caller posts, and its claim has to be absent with it: a caller reading the
// linkage off that file would otherwise close a thread whose replacement was never
// written.
func TestAFindingThisRunWillNotPlacePublishesNoLinkage(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s","inline_comments":[
		{"path":"a.go","line":12,"severity":"blocker","body":"still wrong",
		 "supersedes_thread_ids":["`+threadA+`"]},
		{"path":"b.go","line":4,"severity":"nit","body":"a nit",
		 "supersedes_thread_ids":["`+threadB+`"]}]}`)

	blob, err := json.Marshal(PlaceableFindings(v, false, ownThreads(threadA, threadB)))
	if err != nil {
		t.Fatalf("marshalling the findings: %v", err)
	}
	if !strings.Contains(string(blob), threadA) {
		t.Errorf("the blocker's linkage is not in the file, so its thread can never "+
			"close:\n%s", blob)
	}
	if strings.Contains(string(blob), threadB) {
		t.Errorf("a nit this run drops named a thread in the file a caller resolves "+
			"from:\n%s", blob)
	}
}

// TestALinkageIsOmittedRatherThanWrittenEmpty is the compatibility contract, in the bytes.
//
// A caller cannot ask a file which driver wrote it. What it can read is whether any
// finding in the file carries the key at all, and that is what decides between the
// per-thread gate and the per-review one it falls back to. A finding replacing nothing
// therefore has to write no key: an empty array on every finding would read as a driver
// that published the linkage and had nothing to link, and a caller would then take the
// per-thread path against a file that names no thread -- closing nothing, ever.
func TestALinkageIsOmittedRatherThanWrittenEmpty(t *testing.T) {
	t.Parallel()

	v := verdictFrom(t, `{"read":120,"decision":"comment","summary":"s","inline_comments":[
		{"path":"a.go","line":12,"severity":"blocker","body":"replaces nothing"}]}`)

	blob, err := json.Marshal(PlaceableFindings(v, true, ownThreads(threadA)))
	if err != nil {
		t.Fatalf("marshalling the findings: %v", err)
	}
	if strings.Contains(string(blob), "supersedes") {
		t.Errorf("a finding replacing no thread wrote the key:\n%s", blob)
	}
}
