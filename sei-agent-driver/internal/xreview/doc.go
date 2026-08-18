// Package xreview is the pull-request review workload: what the agent is asked, how
// to tell it has answered, and what to publish.
//
// Everything about running the session — resolving the agent, adopting one an earlier
// dispatch created, following a turn across the streams it outlives, answering
// permission prompts — belongs to the driver package and is the same whatever the
// agent is asked for. This package is the other half: it writes the instruction, and
// it reads the reply back into something a workflow can act on. It reaches no server
// and knows nothing about sessions.
//
// Everything in it exists because of one property of its input. This file holds that
// property and the rules that follow from it; a code comment says why one statement
// is where it is, at the statement.
//
// # The input is hostile
//
// The diff, the pull request body, the file the change adds, the comments a previous
// review drew, the report a scout returned — all of it is written by whoever opened
// the pull request, and all of it goes into a prompt whose answer gets published
// under the bot's identity.
//
// So the material under review is data describing what someone wants reviewed, never
// instructions, and nothing it contains may become a claim the output makes on its
// own behalf. Every rule below is a consequence.
//
// # Reading order
//
//   - xreview.go — what a review is asked for: the Request, and the workload the
//     driver drives.
//   - prompt.go — the instruction, first dispatch and every one after.
//   - verdict.go — reading a reply back into the five buckets.
//   - findings.go — which findings can be placed on a line, and which cannot.
//   - check.go — the check run, whose conclusion follows the findings.
//   - publish.go — the comment body, and what happens when it will not fit.
//   - scout.go — an independent reading, its prompt, and its own run key.
//   - history.go, runkey.go — prior threads, and the key a session is found by.
//
// # How the package models it
//
// Embedded, not fetched. The diff, the standards, the intent and the prior findings
// are written into the prompt rather than left as steps the agent performs. A step
// the agent must perform is a step it can skip; prose the author controls should not
// travel through a shell to arrive; and attribution comes from this side rather than
// from whatever the fetched text claims about itself.
//
// Standards from the base branch. A change that edits REVIEW.md must not hand itself
// the standards it is judged against.
//
// One deciding block. A reply closes with a single fenced json block and nothing
// after it. A decision quoted inside the diff makes the parse refuse rather than
// forge, because two blocks are ambiguous and ambiguity resolved by picking one is
// how a pull request writes its own verdict.
//
// Every prompt-bound string is one-lined and clipped. A finding, a scout note or a
// filename that carried a newline could otherwise open a heading and attribute
// itself to someone else. The check summary is held to the same rule from the other
// end: it is assembled under this package's own headings, so a field reaching it is
// one-lined, and the review's own summary — the one part published as prose — has
// its heading markers defused instead, so it keeps its paragraphs and loses only the
// ability to open a section.
//
// The conclusion follows the findings. A check's pass or fail is derived from what
// the review listed, not from the word the agent used about itself, so a reply that
// says approve while listing a blocker still fails — including a blocker that names
// no line and so cannot be posted as an inline comment.
//
// A scout's name is its dispatch identity, fixed when it was sent, never a value the
// scout returned. And a reading that failed is named as failed — a credential outage
// that read as a clean review would do so across every pull request at once.
//
// # Two prompts, not one
//
// A session outlives the run that opened it, so all but the first dispatch on a pull
// request takes the adopted path. That makes the adopted prompt the common one, not
// the exception.
//
// The consequence shapes prompt.go: rules are restated there rather than referred
// back to. An adopted session replays only its first message, so a rule that lives
// only in the first prompt is a rule that silently stops applying on re-review.
// bucketRules is shared between them for exactly that reason, written once so the two
// cannot drift.
//
// Shared text carries no positional reference for the same reason. Step numbers exist
// only in the first prompt, so a "Step 3" written into text both of them render points
// at nothing on the path almost every review takes.
//
// The scout prompts follow the same split for the same reason.
package xreview
