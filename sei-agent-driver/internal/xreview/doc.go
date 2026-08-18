// Package xreview is the pull-request review workload: what the agent is asked, how
// to tell it has answered, and what to publish.
//
// The driver package owns everything about running the session. It resolves the
// agent, adopts one an earlier dispatch created, follows a turn across the streams it
// outlives, and answers permission prompts. None of that changes with what the agent
// is asked for.
//
// This package is the other half. It writes the instruction, and it reads the reply
// back into something a workflow can act on. It reaches no server and knows nothing
// about sessions.
//
// One property of its input shapes everything here. This file states that property
// and the rules it forces. A code comment explains one statement, beside that
// statement.
//
// # The input is hostile
//
// Everything reaching a prompt here comes from the pull request: the diff, the body,
// the file the change adds, the comments a previous review drew, the report a scout
// returned. Whoever opened the pull request wrote all of it. The answer to that prompt
// gets published under the bot's identity.
//
// So the material under review is data describing what someone wants reviewed, never
// instructions. Nothing it contains may become a claim the output makes on its own
// behalf. Every rule below is a consequence.
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
// Embedded, not fetched. The diff, the standards, the intent and the prior findings are
// written into the prompt rather than left as steps the agent performs. Three reasons.
// A step the agent must perform is a step it can skip. Prose the author controls should
// not travel through a shell to arrive. And attribution comes from this side, not from
// whatever the fetched text claims about itself.
//
// Standards from the base branch. A change that edits REVIEW.md must not hand itself
// the standards it is judged against.
//
// One deciding block. A reply closes with a single fenced json block and nothing after
// it. A decision quoted inside the diff makes the parse refuse rather than forge. Two
// blocks are ambiguous, and ambiguity resolved by picking one is how a pull request
// writes its own verdict.
//
// Every prompt-bound string is one-lined and clipped. A finding, a scout note or a
// filename that carried a newline could otherwise open a heading and attribute
// itself to someone else.
//
// The check summary follows the same rule from the other end. This package writes
// that summary under headings of its own, so every field it draws in is one-lined.
// The review's own summary is the exception, because it is published as prose. Its
// heading markers are defused instead: it keeps its paragraphs and loses only the
// ability to open a section.
//
// The conclusion follows the findings. A check passes or fails on what the review
// listed, never on the word the agent used about itself. So a reply that says approve
// while it lists a blocker still fails. That holds for a blocker which names no line
// and so cannot become an inline comment.
//
// A scout's name is its dispatch identity. This side fixes it when the scout goes out,
// and never takes it from what the scout returned. A reading that failed is reported as
// failed. A credential outage that read as a clean review would read that way on every
// pull request at once.
//
// # Two prompts, not one
//
// A session outlives the run that opened it, so all but the first dispatch on a pull
// request takes the adopted path. That makes the adopted prompt the common one, not
// the exception.
//
// The consequence shapes prompt.go: rules are restated there rather than referred
// back to. An adopted session replays only its first message, so a rule that lives
// only in the first prompt quietly stops applying on re-review. Both prompts render
// bucketRules for that reason. It is written once, so the two cannot drift.
//
// Shared text carries no positional reference for the same reason. Step numbers exist
// only in the first prompt. So a "Step 3" written into text both of them render points
// at nothing on the path almost every review takes.
//
// The scout prompts follow the same split for the same reason.
package xreview
