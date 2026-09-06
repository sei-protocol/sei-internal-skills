package review

import "strings"

// Bounds on the thread ids one reply may name.
const (
	// maxThreadID bounds one id. A node id GitHub mints is short; this is generous
	// against the classic form and far under what a runaway reply produces.
	maxThreadID = 200

	// maxThreadIDs bounds how many ids one key carries. A caller reads at most a
	// hundred prior threads, so a reply naming more than that names ids no allowlist
	// can hold. The bound is load-bearing because every refused id is reported: an
	// unbounded refusal list is model output written to a file and to a log.
	maxThreadIDs = 100
)

// ThreadPlan is what a caller does to the threads this tool left before, once it has
// published this review.
//
// Resolving a thread is a mutation on someone else's pull request, and the ids that
// decide it come out of a reply the agent wrote after reading a diff its author
// controls. So the ids here are the admitted ones only: [BuildThreadPlan] keeps an id
// that names a thread the caller listed as this tool's own, and refuses every other.
//
// The two lists are separate because a caller gates them differently. Addressed is
// spent whenever the review publishes: its finding is gone from the diff, so the thread
// closes whether or not anything new reached the code. Superseded is spent only when the
// replacement comments actually posted, because a thread closed with nothing put in its
// place takes a live finding off the pull request.
type ThreadPlan struct {
	// Addressed are the threads whose finding the change fixed. Nothing replaces them.
	Addressed []string `json:"addressed"`

	// Superseded are the threads a new inline comment restates. They close once that
	// comment is on the code and not before.
	Superseded []string `json:"superseded"`

	// Refused are the ids the reply named that no thread of this tool's carries, one
	// line each and clipped, so a caller can print them. They are the reply's own bytes
	// and they reach a warning a human reads, which is why nothing is published raw.
	Refused []string `json:"refused"`
}

// BuildThreadPlan reads the thread ids out of a reply and admits the ones a caller may
// act on.
//
// One pass over one allowlist, so the two lists and the refusals cannot disagree about
// what "this tool's own thread" means. An id is admitted once: naming it in both keys,
// or twice in one, resolves one thread and reports one refusal.
//
// The superseding ids are read from the findings a caller can place, under the same nit
// and placeability rules [PlaceableFindings] applies. A comment that never posts
// supersedes nothing, and a thread closed behind an unposted comment leaves the author a
// finding that reaches them nowhere.
func BuildThreadPlan(v Verdict, includeNits bool, prior []PriorThread) ThreadPlan {
	own := ownThreadIDs(prior)
	plan := ThreadPlan{
		Addressed:  []string{},
		Superseded: []string{},
		Refused:    []string{},
	}
	seen := make(map[string]bool)
	admit := func(ids []string, into *[]string) {
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			if own[id] {
				*into = append(*into, id)
				continue
			}
			// One-lined and clipped, because this is the reply's own bytes on their
			// way to a warning a caller prints. An id that failed the shape check is
			// exactly the one that can carry a newline.
			plan.Refused = append(plan.Refused, clip(oneLine(id), maxThreadID))
		}
	}

	admit(namedThreadIDs(v.Structured, "resolved_thread_ids"), &plan.Addressed)
	for _, f := range PlaceableFindings(v, includeNits) {
		admit(f.Supersedes, &plan.Superseded)
	}
	return plan
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
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-', r == '=', r == '+', r == '/':
		default:
			return false
		}
	}
	return true
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
