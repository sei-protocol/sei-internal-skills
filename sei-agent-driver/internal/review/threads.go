package review

import "strings"

// Bounds on the thread ids one reply may name, and on what a plan reports back.
const (
	// maxThreadID bounds one id. A node id GitHub mints is short; this is generous
	// against the classic form and far under what a runaway reply produces.
	maxThreadID = 200

	// maxThreadIDs bounds how many ids one key carries. A caller reads at most a
	// hundred prior threads, so a reply naming more than that names ids no allowlist
	// can hold.
	maxThreadIDs = 100

	// maxPlanThreadIDs bounds how many ids one plan decides at all. Every id costs a map
	// entry and a pass, and the number of them is a reply's to choose. Past it nothing
	// further is admitted, which closes no thread — the safe direction, since a thread
	// left open is what this workflow does today.
	maxPlanThreadIDs = maxThreadIDs * maxPlaceableFindings

	// maxRefusedThreadIDs bounds how many refusals a plan carries, over the whole plan
	// rather than over one key.
	//
	// Per key is not a bound on this file. supersedes_thread_ids is read once per
	// placeable finding, so maxThreadIDs alone admits maxThreadIDs × maxPlaceableFindings
	// refusals -- about a megabyte of model output written into the check file and
	// echoed line by line by a caller that warns on each. The reason to report a refusal
	// is that the review is naming threads which do not exist, and twenty examples carry
	// that as well as five thousand. RefusedTotal is what says how many there were.
	maxRefusedThreadIDs = 20
)

// ThreadPlan is what a caller does to the threads this tool left before, once it has
// published this review.
//
// Resolving a thread is a mutation on someone else's pull request, and the ids that
// decide it come out of a reply the agent wrote after reading a diff its author
// controls. So the ids here are the admitted ones only: [BuildThreadPlan] keeps an id
// that names a thread the caller listed as this tool's own, and refuses every other.
//
// The two act-on lists are separate because a caller gates them differently. Addressed
// is spent whenever the review publishes: its finding is gone from the diff, so the
// thread closes whether or not anything new reached the code. Superseded is spent only
// when the replacement comments actually posted, because a thread closed with nothing
// put in its place takes a live finding off the pull request.
//
// That difference is also which list wins when a reply names one id under both keys.
// See [BuildThreadPlan].
type ThreadPlan struct {
	// Addressed are the threads whose finding the change fixed. Nothing replaces them.
	Addressed []string `json:"addressed"`

	// Superseded are the threads a new inline comment restates. They close once that
	// comment is on the code and not before.
	Superseded []string `json:"superseded"`

	// Refused are the ids the reply named that no thread of this tool's carries, in a
	// form a caller can echo. At most maxRefusedThreadIDs of them; RefusedTotal is how
	// many there were.
	Refused []string `json:"refused"`

	// RefusedTotal is how many ids were refused, counting the ones Refused does not
	// list. A caller reading len(Refused) alone would report twenty over a reply that
	// named five thousand, which is the difference between a model that slipped and a
	// model that is inventing ids wholesale.
	RefusedTotal int `json:"refused_total"`
}

// BuildThreadPlan reads the thread ids out of a reply and admits the ones a caller may
// act on.
//
// One pass over one allowlist, so the two lists and the refusals cannot disagree about
// what "this tool's own thread" means. An id is admitted once, and the three passes below
// are what decide where.
//
// A reply may name one id under both keys, and which list wins is a safety question
// rather than a tidiness one. Addressed closes a thread on publication alone; Superseded
// closes it only once a comment replaced the finding. So an id an inline comment claims
// can never reach Addressed, and a self-contradicting reply costs a thread left open
// rather than a live finding taken off the pull request with nothing where it was.
// Nothing in the prompt forbids naming an id twice and the reply is untrusted, so this is
// a shape that arrives.
//
// Ordering the passes is not enough on its own, which is why there are three. The first
// takes the findings a caller can place, under the same nit and placeability rules
// [PlaceableFindings] applies, because a comment that never posts supersedes nothing. The
// second takes the claims left on findings this run will NOT place: they close nothing,
// and spending them here is what stops the third pass from picking one up under the
// weaker gate. [PlaceableFindings] stays the single definition of what gets placed — the
// second pass is a superset the first has already filtered.
func BuildThreadPlan(v Verdict, includeNits bool, prior []PriorThread) ThreadPlan {
	own := ownThreadIDs(prior)
	plan := ThreadPlan{
		Addressed:  []string{},
		Superseded: []string{},
		Refused:    []string{},
	}
	seen := make(map[string]bool)
	// into may be nil, which decides an id without closing anything. An invented id is
	// still reported there: a review naming threads that do not exist is worth an
	// operator's attention wherever in the reply it wrote them.
	admit := func(ids []string, into *[]string) {
		for _, id := range ids {
			if seen[id] || len(seen) >= maxPlanThreadIDs {
				continue
			}
			seen[id] = true
			if own[id] {
				if into != nil {
					*into = append(*into, id)
				}
				continue
			}
			plan.RefusedTotal++
			if len(plan.Refused) < maxRefusedThreadIDs {
				plan.Refused = append(plan.Refused, safeThreadID(id))
			}
		}
	}

	admit(supersedingIDs(PlaceableFindings(v, includeNits)), &plan.Superseded)
	admit(supersedingIDs(reportedPlacements(v)), nil)
	admit(namedThreadIDs(v.Structured, "resolved_thread_ids"), &plan.Addressed)
	return plan
}

// supersedingIDs is every thread these findings say they replace, in order.
func supersedingIDs(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Supersedes...)
	}
	return out
}

// reportedPlacements decodes every line-tied finding a reply offered, placeable or not.
//
// Read for the claims on findings that will not be placed. Those close nothing, and the
// reason to decode them at all is that an id left undecided is one resolved_thread_ids
// can still take under the weaker gate.
func reportedPlacements(v Verdict) []Finding {
	entries := reportedFindings(v)
	out := make([]Finding, 0, len(entries))
	for _, entry := range entries {
		fields, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, findingFrom(fields))
	}
	return out
}

// ownThreadIDs is the set of threads a reply is allowed to name.
//
// The caller supplies it, in the same file it supplies the history from: those are the
// threads it has already established are this tool's own on this pull request. A reply
// naming anything else is naming a thread this tool did not write, and the id came from
// model output, so the claim carries no weight on its own.
//
// A resolved thread stays in the set. It belongs to this tool, so naming it is not a
// refusal worth showing anyone, and resolving one that is already resolved changes
// nothing. A caller that skips it saves a call, and this is not the place that decides.
//
// [wellFormedThreadID] runs here rather than at every use. An id that cannot be in this
// set cannot be admitted, so the shape check and the membership check are one gate.
func ownThreadIDs(prior []PriorThread) map[string]bool {
	own := make(map[string]bool, len(prior))
	for _, t := range prior {
		if wellFormedThreadID(t.ID) {
			own[t.ID] = true
		}
	}
	return own
}

// namedThreadIDs reads the ids a reply wrote under one key, trimmed, without the empties
// and bounded in count.
func namedThreadIDs(fields map[string]any, key string) []string {
	raw := listField(fields, key)
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		id, ok := entry.(string)
		if !ok {
			continue
		}
		if id = strings.TrimSpace(id); id == "" {
			continue
		}
		out = append(out, id)
		if len(out) >= maxThreadIDs {
			break
		}
	}
	return out
}

// wellFormedThreadID reports whether s has the shape of a node id GitHub mints.
//
// The shape, not a set of known values: [ownThreadIDs] is what decides an id may be
// acted on, and this is what decides it may be rendered. An id reaches a prompt line, a
// GraphQL variable and a warning a caller echoes, and each of those is a place where a
// newline or a bracket writes something this tool did not.
//
// GitHub mints two forms and both are base64 alphabets — the classic
// "MDIzOlB1bGxSZXF1ZXN0UmV2aWV3VGhyZWFk…" and the current "PRRT_kwDO…". Nothing outside
// those alphabets is admitted, so a caller's file cannot put arbitrary bytes in a prompt
// by calling them a thread id.
func wellFormedThreadID(s string) bool {
	if s == "" || len(s) > maxThreadID {
		return false
	}
	for _, r := range s {
		if !threadIDRune(r) {
			return false
		}
	}
	return true
}

// threadIDRune reports whether r is one of the characters a node id is built from.
//
// One definition, read by the check that admits an id and by the filter that renders a
// refused one. Two would drift, and the direction they would drift in is a byte this
// package refuses to act on and prints anyway.
func threadIDRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_', r == '-', r == '=', r == '+', r == '/':
		return true
	}
	return false
}

// safeThreadID renders a refused id in a form a caller can echo.
//
// A refusal is model output on its way to a log line and to a workflow annotation, and a
// terminal reads more than text: an id carrying ESC can open an OSC 8 hyperlink, and one
// carrying BEL or DEL can rewrite what a reader sees. [oneLine] does not stop any of
// them — it splits on unicode space, and a C0 control is not one.
//
// So the filter is the alphabet an id is allowed to be made of, and every byte outside it
// becomes a question mark. Replaced rather than dropped: the length still says how much
// was written, and a reader sees that the value was not an id rather than seeing a
// shorter string that looks like one. Invalid UTF-8 arrives as U+FFFD and is replaced
// with the rest.
func safeThreadID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if threadIDRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('?')
	}
	return clip(b.String(), maxThreadID)
}

// threadHandle renders the id a reply names to close a thread, and nothing when the
// caller supplied none.
//
// The leading space belongs to the token: the callers append it to a location, and a
// thread with no id has to render as the location alone.
func threadHandle(t PriorThread) string {
	if !wellFormedThreadID(t.ID) {
		return ""
	}
	return " [thread_id: " + t.ID + "]"
}
