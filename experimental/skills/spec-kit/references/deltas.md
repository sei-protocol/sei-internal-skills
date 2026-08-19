# Deltas from the vendored template

Spec Kit's `spec-template.md` is the shape. Keep its section order and its field
names. These are the only changes, and each one has a reason.

## 1. Add the anchor block

Not in Spec Kit. It goes directly under the metadata, before User Scenarios.

Without it, a spec explains its own method, and the explanation is the bulk of
the document. With it, the method is named once and the body carries only what
is specific to this feature.

## 2. Every success criterion names a verifier

Spec Kit asks for measurable outcomes. Measurable is not the same as checked.

Before, from the vendored template:

> **SC-002**: System handles 1000 concurrent users without degradation

After:

> **SC-002** The exported API stays backward compatible across the release.
> *Verifier:* `gorelease` and `gocompat` in CI, against the previous tag.

When nothing checks it, say so:

> **SC-005** A new engineer can follow the reconcile path without a narrator.
> *Verifier:* judgement — `/brandon-code` review, no machine check.

`judgement` is an honest verdict. An unmarked criterion that nothing checks is not.

## 3. Write engineering criteria, not product metrics

The vendored examples are consumer-product flavoured: onboarding time, support
ticket volume, user satisfaction. For platform and SDK work they produce mush.

Anchor a criterion to something the repository can observe: an exit code, a
benchmark number, a coverage delta, an API-compatibility result, a line count, a
lint verdict.

## 4. Requirements take EARS form

The template writes `System MUST [capability]`. Keep the RFC 2119 keyword and add
the EARS template, so the trigger and the condition sit in their own slots.

Before:

> **FR-003**: System MUST validate email addresses

After:

> **FR-003** WHEN a caller submits a session request, the client MUST reject an
> address that fails RFC 5322 parsing before any network call.

One requirement per statement. Split rather than joining with "and".

## 5. Delete the placeholder comments

The template ships instructions as HTML comments — `ACTION REQUIRED`,
`IMPORTANT`, and bracketed examples. They are authoring scaffolding. A shipped
spec that still carries them tells the reader nobody finished the section.

## 6. The writing contract applies

`vale` runs on the finished spec. Descriptive sentences stay under 25 words,
active voice, one instruction per sentence. A term list or a verbatim quotation
is wrapped in `<!-- vale off -->` and `<!-- vale on -->`, not reworded.

## 7. Keep these exactly as Spec Kit has them

Do not redesign what already works:

- `[NEEDS CLARIFICATION: <question>]` — the honest marker for an unstated detail.
- `(Priority: P1)` on every user story.
- **Why this priority** and **Independent Test** on every user story.
- **Acceptance Scenarios** in Given / When / Then.
- **Key Entities**, only when the feature involves data.
- **Assumptions**, stating the defaults chosen where the input was silent.

## Why the four story fields are not optional

`/tasks-to-linear` files one Linear issue per user story. It files what the
tasks phase decided and does not re-plan. A story missing its Independent Test
becomes a ticket whose author has to invent the acceptance bar, which is where
the specification stops binding the work.
