# The Review Contract

`internal/xreview` — what the agent is asked, and what counts as an answer.

The driver runs a workload to a reply and stops. This package is the workload: it
writes the instruction, and it reads the reply back into something a workflow can
act on. It reaches no server and knows nothing about sessions.

Everything in it exists because of one property of its input.

## The input is hostile

The diff, the pull request body, the file the change adds, the comments a previous
review drew, the report a scout returned — all of it is written by whoever opened
the pull request, and all of it goes into a prompt whose answer gets published under
the bot's identity.

So the material under review is **data describing what someone wants reviewed**,
never instructions, and nothing it contains may become a claim the output makes on
its own behalf. Every rule below is a consequence.

## Where things are written down

| here | what belongs in it |
|---|---|
| `README.md` (this file) | what the package is for, what to read first, the shape of the rules |
| the package doc in `xreview.go` | what the package is and where its boundary sits |
| code comments | why one statement is where it is, at the statement |

## Reading order

| file | the question it answers |
|---|---|
| `xreview.go` | what a review is asked for — the `Request`, and the `Workload` the driver drives |
| `prompt.go` | the instruction, first dispatch and every one after |
| `verdict.go` | reading a reply back into the five buckets |
| `findings.go` | which findings can be placed on a line, and which cannot |
| `check.go` | the check run, whose conclusion follows the findings |
| `publish.go` | the comment body, and what happens when it will not fit |
| `scout.go` | an independent reading, its prompt, and its own run key |
| `history.go`, `runkey.go` | prior threads, and the key a session is found by |

## How the package models it

**Embedded, not fetched.** The diff, the standards, the intent and the prior
findings are written into the prompt rather than left as steps the agent performs.
A step the agent must perform is a step it can skip; prose the author controls
should not travel through a shell to arrive; and attribution comes from this side
rather than from whatever the fetched text claims about itself.

**Standards from the base branch.** A change that edits `REVIEW.md` must not hand
itself the standards it is judged against.

**One deciding block.** A reply closes with a single fenced json block and nothing
after it. A decision quoted inside the diff makes the parse refuse rather than
forge, because two blocks are ambiguous and ambiguity resolved by picking one is
how a pull request writes its own verdict.

**Every prompt-bound string is one-lined and clipped.** A finding, a scout note or
a filename that carried a newline could otherwise open a heading and attribute
itself to someone else.

**The conclusion follows the findings.** A check's pass or fail is derived from what
the review listed, not from the word the agent used about itself, so a reply that
says `approve` while listing a blocker still fails — including a blocker that names
no line and so cannot be posted as an inline comment.

**A scout's name is its dispatch identity**, fixed when it was sent, never a value
the scout returned. And a reading that failed is named as failed — a credential
outage that read as a clean review would do so across every pull request at once.

## Two prompts, not one

A session outlives the run that opened it, so all but the first dispatch on a pull
request takes the adopted path. That makes the adopted prompt the common one, not
the exception.

The consequence shapes the file: rules are restated there rather than referred back
to. An adopted session replays only its first message, so a rule that lives only in
the first prompt is a rule that silently stops applying on re-review. `bucketRules`
is shared between them for exactly that reason — written once so the two cannot
drift.

The scout prompts follow the same split for the same reason.
