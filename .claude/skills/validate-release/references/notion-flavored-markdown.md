# Notion-flavored-Markdown authoring guard

**Why this is load-bearing:** a malformed line on a shared, exec-facing Notion page renders wrong
*silently*. The author cannot see it without a re-fetch. **And** a line with tangled inline formatting
**defeats a later `update_content` match**. That is the only mutation path (`write-contract.md` bans `replace_content`),
so nobody can surgically fix the line at all. The only reliable remedy is authoring it
correctly the first time. These rules keep every written line **clean on render and re-matchable on edit.**

Live corruptions on a shared exec Notion board (the retired Impact Tracker) produced them, each noted below.
The rules are surface-general: they apply to any Notion page this repo writes, which today means the release report.

## The hard rules (author to these; the failure each prevents is real)

1. **At most one inline `code` span per emphasis run — never two inside one `*italic*` (or `**bold**`) run.**
   Notion's parser mishandles `*… `a` … `b` …*`. It renders the first span **bold-code** and leaves the
   second span's backticks **literal** (visible `` `b` ``). It strands the run's closing `*` as a stray
   literal asterisk. Put code spans in *plain* (un-emphasized) text, or keep the emphasized line to a
   single span. *(This corrupted the Observability Plane weekly-log caption — four failed `update_content`
   attempts and a full-page rebuild to clear, because the mangled line was no longer matchable.)*
2. **Wrap any token containing `_`, `__`, `*`, or `**` in a `code` span.** Markdown emphasis fires on
   ordinary identifiers, not just deliberate emphasis:
   - A **single `_` pair across two snake_case tokens** italicizes the text between them. A sentence
     naming `worker_count` … `batch_size` renders as "worker*count … batch*size" (italic from the first
     interior `_` to the second). This is the **high-frequency** hazard: weekly bullets routinely name
     Prometheus metrics (`sei_block_height`), config keys, and env vars.
   - `__name__` renders as bold **name**; `**x**` as bold.
   Code-spanning the token (`` `worker_count` ``, `` `__name__` ``) renders it literally and defuses all
   of these at once. Do not rely on recognizing a token as a "path/identifier" (Rule 3) to catch this —
   wrap on the *character*, not the category.
3. **Wrap bare domains, paths, and filenames in `code` spans.** Notion auto-linkifies `api.rest`,
   `sei.io/node`, `platform.sei.io`, `seictl-harness.md` into junk links. `` `api.rest` `` renders as
   plain literal text. (A repo-qualified ref like `platform#685` inside a real `[text](url)` link is fine
   — the hazard is *bare* domain-like tokens in running prose.)
4. **Reword to keep literal asterisks out of any emphasis run.** A literal `*` inside an italic/bold run
   breaks the run's balance and re-escapes unpredictably across round-trips. Escaping it (`\*`) is
   **not** reliable inside `*…*`. Move it out: write "the clusters monitoring tree", not the glob-bearing
   path inside an italic caption. (A `\*` in plain, un-emphasized text is fine.)
   - **Do NOT** author an italic caption that embeds a `*`-glob path — that exact line is what stranded a
     trailing asterisk on the Observability caption.
5. **A line you cannot re-match is a line you cannot fix.** `update_content` matches the page's markdown
   *serialization*, which a `notion-fetch` does **not** faithfully invert for mixed-format lines. Escapes
   render ambiguously, so the `old_str` you copy from a fetch may never match. Rules 1–4 keep lines
   simple enough that a later targeted edit *can* match them. Author correctly the first time rather than
   reaching for `replace_content` (banned) or guessing escapes.
6. **Indentation under a toggle is a serialization-fragility zone.** The entry shape nests the outcome
   sentence and bullets beneath the `>` toggle title (see `write-contract.md`). Markdown can read
   4-space-indented continuation as a code block. Notion's round-trip of toggle-nested content is
   exactly the non-invertible serialization Rule 5 warns about. Express nested body as a proper nested
   list, and treat the toggle body as subject to the Verify-render-after-write check below.

## Verify-render-after-write (a check, not a vow)

`write-contract.md` mandates a target re-verify *before* the write. Add the mirror *after* it: **re-fetch
the page and read the block you just wrote.** Look for a code span rendered bold, literal
`` ` `` backticks, or an auto-linked bare domain. Also look for a mid-word italic from a snake_case `_` pair, or a stray
trailing `*`. Any of these means the text broke an authoring rule above. Fix the source text (apply the matching rule) and
re-write.

**If the re-write itself cannot match the mangled line**, do **not** loop on
escape-guessing. This is the Rule 5 deadlock: the corruption is exactly
what makes `update_content`'s `old_str` un-matchable, and `replace_content` stays banned. Anchor on the nearest *clean, re-matchable* text — the **toggle title**, which Rules 1–4
keep simple. Replace the whole toggle block through that anchor; or, if even that will not match, **halt
and surface for human repair**. Never leave the corruption "to fix later" — a later edit may match it even
less. Only a clean post-write render completes the write.

## Quick pre-write checklist

- [ ] No emphasized run (`*…*` / `**…**`) contains more than one `code` span.
- [ ] Every token bearing `_`, `__`, `*`, or `**` — and every bare domain/path/filename — is in a `code` span.
- [ ] No literal `*` sits inside an emphasized run (reword, do not escape).
- [ ] Toggle-nested body is a proper nested list, not indented-paragraph-as-code-block.
- [ ] Post-write: re-fetched and the written block renders clean (no bold-from-code, literal backticks,
      junk links, mid-word italic, or stray asterisk). On an un-matchable re-write, anchor on the toggle
      title or halt — never escape-guess in a loop.
