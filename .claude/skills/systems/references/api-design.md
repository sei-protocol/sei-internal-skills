# API & interface design

Checklist for interfaces that outlive their first caller — once published, an API's behavior is a contract you can't quietly change. Our-own-words items; cite the authority (see `sources.md`). Google AIP / Zalando / Microsoft are CC-BY, adapted with attribution.

Sections: `## checklist` · `## severity model`. Anchors: Google AIP, Zalando RESTful Guidelines, Microsoft REST Guidelines. Companion: `safety-quality.md` (validate every input), `reliability.md` (idempotency keys, bounded retries).

## checklist

| id | principle | rule | cue | authority |
|----|-----------|------|-----|-----------|
| API1 | Backward compat is one-way | Treat a shipped API as a contract you can add to but not break — removing a field, tightening validation, or changing a default breaks callers you can't see. Decide the shape before the first release. | A breaking change to a published response shape/default; tightening required fields post-release. | Google AIP |
| API2 | Idempotency, even for POST | Let clients safely retry by supporting an idempotency key on non-idempotent operations (create/charge), so a retried request returns the original result instead of acting twice. | A create/charge endpoint with no idempotency key, retried on timeout → duplicates. | Microsoft REST Guidelines |
| API3 | Cursor pagination | Page with an opaque cursor whose stability guarantees are documented, not offset/limit — offsets skip or repeat rows when the underlying set changes mid-scan. | `?offset=&limit=` over a mutating collection; a cursor with no documented ordering/stability. | Google AIP / Zalando |
| API4 | Structured, diagnosable errors | Return machine-parseable errors with a stable type/code, a human message, and enough detail to act on — a standard envelope (Problem+JSON) so clients branch without scraping strings. | A bare 500 + free-text string; client code matching on message substrings. | Zalando / Microsoft REST |
| API5 | Explicit versioning | Version explicitly and consistently (path or media-type) with a documented deprecation path — implicit "we'll just change it" versioning breaks callers without warning. | An unversioned public API; mixed versioning schemes; no deprecation policy. | Microsoft REST / Zalando |
| API6 | Resource-oriented naming | Name and structure around resources consistently (predictable plurals, standard methods, uniform casing) so one endpoint predicts the next. *(Advisory — cut-first for an internal/pre-1.0 API.)* | Verb-in-path RPC endpoints mixed with REST; inconsistent casing/plurality across resources. | Google AIP |
| API7 | Minimal public surface | Expose the smallest interface that serves the use case; don't leak internal types/enums/implementation detail into the contract — every exposed field is something you must keep stable. | An internal struct serialized directly to the wire; an enum exposing internal states; speculative fields. | Google AIP |
| API8 | Document the contract, not just the schema | State the guarantees behind each field — nullability, ordering, units, ranges, stability — so clients don't infer behavior from observed responses (Hyrum's Law: observed behavior becomes the de-facto contract). *(Advisory — cut-first until the API is public or has a second consumer.)* | A spec with field types but no nullability/ordering/stability notes; behavior discoverable only by probing. | Zalando; SWE-at-Google (Hyrum's Law) |

## severity model

- **correctness/safety** — API1 (a breaking change is a defect for every existing caller), API2 (missing idempotency → duplicate side effects like double charges). Cause real downstream failures; lead with them.
- **consequence-under-load** — API3 (offset pagination corrupts results and degrades as the dataset grows), API4 (unparseable errors force fragile client logic that fails at scale).
- **advisory** — API5 (versioning), API6 (naming), API7 (surface minimization), API8 (contract docs). Cheap before release, expensive after; flag at design time. Cut-first: API6, API8 for internal/pre-1.0 APIs. Keep API1/API2/API4 as the irreversible floor.
