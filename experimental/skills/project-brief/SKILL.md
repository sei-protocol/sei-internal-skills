---
name: project-brief
description: "Use when authoring or rewriting a Linear Project's summary and description so the project stands on its own for a human reader — 'write the project brief', 'draft this project', 'fill in the project description', 'make this project self-contained', 'this project reads like a manifest', '/project-brief'. Fires when a project exists but its description is empty, is a pointer to a spec, or has become a list of its own issues. Anti-triggers: NOT for writing a project UPDATE — status, health, blockers, date changes and weekly progress belong there, not in the description (Linear separates the two deliberately); NOT for authoring the deep technical spec itself (that is a document this description links to); NOT for filing or grooming issues (/issue); NOT for capturing a design decision (/design); NOT for code or PR prose (/idiomatic). Authors the artifact; it never sets status, never writes issues, and never publishes without confirmation."
category: project-management
---

# Project Brief

Authors the two text fields on a Linear Project — the one-line **summary** and the
**description** — so that a person who opens the project understands it without
opening anything else.

**Shape:** procedural, discipline-primary. The procedure is short. The refusals are
the substance, because the failure modes here survive the author knowing better.

## The bar

From the manager this exists to serve:

> "It has got to be self contained, polished with link to external resources as
> needed, written for human consumption first and foremost. Everything else: specs,
> issues etc. stems from that. It's important that project itself is of super high
> quality with little to no cognitive cycle."

## The one principle

**Self-containment is a property of dependencies, not of volume.** A project is
self-contained when the reader can understand and decide without opening anything
else. Adding words does not make it self-contained. Removing the reader's
dependencies does.

This resolves the apparent conflict between "make it verbose" and Linear's own
guidance to "aim for brevity — short specs are more likely to be read." They agree.
Both forbid padding. Verbose means *complete*, not *long*.

The include/exclude test comes straight from cognitive-load research (split-attention
vs redundancy effects, Sweller and colleagues):

| Can the reader understand your point without the other source? | Do this |
|---|---|
| **No** — the point collapses without it | **Inline it.** Making them go fetch is split attention. |
| **Yes** — they already have it, or the page shows it | **Do not restate it.** Restating is redundancy. |

Everything that is neither is padding. Cut it.

## Guardrails

This skill writes exactly two fields on one Linear project: `summary` and
`description`. Before any write:

1. **Never write a status.** Status, health, blockers, percent-done, target-date
   changes, "what's top of mind", and weekly progress belong to **project updates**,
   which Linear ships as a separate surface with its own cadence. A description that
   opens with `**Status:** Blocked` has taken the update's job and will be wrong
   within a week. Linear's own framing: an update is "brief and to the point. Almost
   like a tweet." A description is the standing spec. **If a sentence would be false
   next month because work progressed, it belongs in an update, not here.**

2. **Never enumerate the project's own issues.** Linear renders them in the Issues
   tab, in attachable filtered views, in the progress graph, and in the milestone
   list. A hand-typed list of them is a second copy that starts drifting the moment
   it is written — and a *partial* copy is worse than none. Write the Docs put it
   exactly: "A map that displays fifty out of one hundred fire hydrants in a
   neighborhood is worse than a map that displays none."

   **You may explain the shape of the work. You may not transcribe it.**
   - Interpretation, allowed: "Build the pool, prove it under load, then cut over DNS."
   - Transcription, refused: "1. Build — PLT-812, PLT-813, PLT-817."

   Relabelling the list as "Sequencing", "Phases", "Milestones" or "Plan" does not
   change what it is. The test is whether you are naming *meaning* or copying
   *identifiers*.

   The one exception: an issue from a **separate workstream** that this project
   depends on or is blocked by. That is a genuine external link, not a child.

3. **Never restate what the page already renders.** Linear's Overview shows lead,
   team, members, status, start and target dates, initiative, milestones with their
   own descriptions, issue counts, percent complete, velocity, predicted completion,
   Resources links, and the latest update. An "Owner and dates" section duplicates
   the property table two inches above it.

   *(Linear does not publish a "do not duplicate" rule. This is derived from what
   Linear documents as auto-rendered, plus the single-source-of-truth doctrine in
   GitLab's and Google's engineering handbooks. Stated as derivation, not as vendor
   guidance.)*

4. **Never write for a machine.** One human reader. No parsable status lines, no
   health scores, no RAG colours, no key-value headers chosen because a pipeline can
   lift them, no "Last reviewed" stamp added so an assistant knows the age. If a
   downstream system needs structure, it reads Linear's actual fields — which are
   already structured, already current, and already the source of truth. Every
   element added for a parser is redundancy charged to the human.

5. **Never leave a placeholder in the artifact.** No `[TODO]`, no `[fill this in]`,
   no bracketed slot, no note-to-self. This is the failure this skill exists to stop:
   in baseline testing it happened in two of three runs, and **both runs knew it was
   wrong while doing it** — one wrote, "three unfilled slots pasted into a live
   project would be worse than the one-liner, because it looks finished and isn't."
   A field with holes looks finished and is not. If a fact is missing, the gap goes
   to the human in conversation and the write does not happen. See step 5.

6. **Never fabricate, and never assume the inputs are real.** Verify project, issue,
   and person identifiers against Linear before using them. In baseline testing an
   agent was handed fourteen plausible issue IDs and found every one belonged to
   unrelated work. Invented numbers are worse than missing ones: a missing fact costs
   ten minutes, a fabricated one gets repeated to an exec team by someone whose
   credibility is not yours.

7. **Draft → confirm → write.** Always render both fields in full and get explicit
   confirmation before writing to Linear. Never auto-write. If Linear is unavailable,
   say so and stop.

**When the human overrides any of these.** Refuse once, in plain language, with the
specific cost named. Then it is their board and their call — write what they asked
for and record the cost in conversation, never inside the field. Two things stay
constant regardless: never comply silently, and never present the decision as yours.
This applies to every guardrail above, not only to status. *(A tester hit this gap:
the draft gave an escape valve for the status rule alone and left the others silent,
so it had to generalise the rule itself.)*

## Preconditions

- Linear MCP tools available (`get_project`, `list_projects`, `save_project`).
- A target project that already exists. This skill writes a brief; it does not
  create projects.
- Enough source material to fill the sections without invention — a spec, a design
  doc, an incident, a conversation, or the author in the room to answer questions.

## Procedure

**The order of these steps is load-bearing.** Verification comes before drafting, and
that is not stylistic. A tester put it exactly: *"If I had drafted first and verified
second, I would have had a finished page to argue for, and I would have found an
argument."* A draft in hand converts a verification failure into a negotiation. Do not
give yourself something to defend.

1. **Resolve the project and read what is already there.** Fetch it. Read the current
   `summary`, `description`, milestones, and the most recent project update. Note the
   lead, teams, and dates — you are reading them so you do not repeat them.

2. **Interview for the gaps, before writing anything.** The description needs facts
   only a human holds. Ask for what is missing rather than inferring it: what breaks
   today, why this quarter, what is deliberately not in scope, what "done" means,
   what is genuinely hard about it. One round, batched — not a question at a time.

3. **Classify every candidate fact** against the two tests before it earns a place:
   - *Does the page already render it?* → cut (guardrail 3).
   - *Would it be false next month because work progressed?* → it is an update, not a
     description (guardrail 1).
   - *Can the reader follow the point without it?* → cut if yes, inline if no.

4. **Draft both fields** using the section order in `references/structure.md`. The
   ordering rule is Linear's own, from their Head of Product: start with what is
   least likely to change, end with what is most likely to change. That is what makes
   a description survive contact with the project.

5. **Run the pre-write checks.** All of these are blocking:
   - No placeholder, bracket, or TODO anywhere in either field.
   - No issue identifier belonging to this project.
   - No status, health, or progress language.
   - No section that repeats a rendered property.
   - Every hedge is gone or made concrete — no "should", "generally",
     "significantly", "aims to". Amazon's Bar Raisers call these weasel words and
     read them as a tell: "a giveaway to your reader that you haven't got the detail."
   - Every claim is either self-evident, sourced from material you actually read, or
     confirmed by the human this run.

   If any check fails, **stop and report the gap.** Do not write a partial field and
   annotate what is missing — that is how placeholders get shipped.

6. **Show both fields in full, then ask.** Render the exact text for `summary` and
   `description`. State plainly what you left out and why, and what you could not
   source. Wait for explicit confirmation.

7. **Write, then verify.** Call `save_project`. Re-fetch and confirm both fields
   landed as shown.

## Rationalization table

Every row was produced by a subagent in baseline testing, in its own words. When
your reasoning matches the left column, **stop**.

| Excuse | Reality |
|---|---|
| "It isn't an issue list, it's sequencing / phases / a plan." | The baseline agent wrote *"restating it as prose is wasted space"* and then shipped four phases of issue IDs. Renaming the list does not change it. Name the meaning, not the identifiers. |
| "A brief should say who owns it and when it ships." | Linear renders lead and target date directly above your text. You are competing with the property table and you will lose to it — it updates itself. |
| "Placeholders are safer than a note nobody reads." | Both are wrong. The gap goes to the human in conversation, and the write waits. A field with holes looks finished and is not. |
| "Status up top is what a busy reader wants." | A busy reader wants it — from the update, which is built for it and refreshed weekly. Put it here and you have written something that is false by next Thursday. |
| "Both audiences want the same thing, so structure for both." | Then you did not structure for the machine; you structured for the human and stopped. The moment you add a field the human does not need, the claim is false. |
| "Adding one parsable line for the pipeline is free." | It is redundancy, and redundancy is charged to the human reader in extraneous load. Linear's own fields are already structured and already current. |
| "The spec says it better. Just link the spec." | The description is read in a list view, on a phone, by someone on-call, by a new hire in week one. None of them will open six pages to learn what this is. An abstract is not a duplicate of the paper. |
| "Two other projects do it this way." | Two projects doing a thing is evidence two people were busy, not that it worked. Check whether anyone found them useful. |
| "It's a long week and this is basically formatting." | The description is the artifact everything else stems from. Being tired is a reason to work fast; it is not a reason to assert what you have not checked. |
| "More detail signals more rigour." | Measured the other way: complexity *lowers* judged intelligence, mediated by processing difficulty (Oppenheimer 2006). Padding reads as not knowing. |
| "I'll hedge — I'm not certain yet." | Uncertainty is content. Write the open question as an open question with a name and a date. A hedge hides it. |

## Red flags — stop and reset

- You are about to type `[` in the description.
- You are copying an issue identifier that belongs to this project.
- You are writing the words "on track", "blocked", "% complete", or "last updated".
- You are adding a heading because it looks like the other projects.
- You just wrote "should", "generally", "significantly", or "aims to".
- You cannot say where a number came from.
- The description is longer than roughly a page and still growing.

## Halt conditions

Stop and report; never auto-remediate:

- **A required fact is missing and the human is not available.** Report exactly which
  section cannot be written. Do not draft around the hole.
- **An identifier does not resolve in Linear.** Report the mismatch; do not proceed
  on the assumption it will be created later.
- **The project has no clear outcome.** A project whose description cannot state what
  done means is not a project yet — say so rather than writing an eloquent
  description of ambiguity.
- **The user asks for status in the description.** Explain the update surface once,
  and offer to draft the update instead. If they repeat the request, it is their
  board — write it and note the drift cost in conversation, not in the field.
- **Linear is unavailable.** Say so and stop. Never claim a write that did not happen.

## What this skill does not do

- **Write project updates.** Different surface, different cadence, different content.
- **Author the technical spec.** That is a document. This links to it as depth, never
  as a substitute for orientation.
- **Create or groom issues.** Use `/issue`.
- **Set status, health, dates, lead, or members.** Those are properties a human owns.
- **Aggregate across projects.** Linear Views and Pulse do that natively — which is
  why the skills that used to do it here were retired.

## State

`state/run-<ISO-timestamp>/` holds `source-notes.md` (what was gathered and from
where), `draft.md` (both fields as shown to the user), and `audit.log`. Gitignored.

## Output

Both fields rendered in full, then a short plain-language note: what was deliberately
left out, what the page already renders, and anything that could not be sourced.
