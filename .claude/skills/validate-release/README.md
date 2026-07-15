# Release Validation

> Chaos and metrics signals, deciphered, drive a release go/no-go verdict.

![Release Validation architecture diagram](assets/validate-release.png)

This skill turns a real nightly chaos run into a single executive-quality release validation report on Notion. It consumes what the live harness actually emits: the raw harbor Prometheus metrics (via the federated `prometheus-prod` datasource, time-scoped to the run window) for each scenario's story, and the harness Job — its spec env for the release image under test (`SEID_IMAGE_CHAOS`) and its pod-log for the authoritative per-scenario PASS/FAIL verdict. It leads with a clear **liveness** go/no-go — `LIVENESS GO` / `LIVENESS NO-GO`, never a bare "GO", with the tx-correctness caveat inline. It is read-only: it queries Prometheus and reads the harbor Job (`kubectl get`/`logs`), writes one Notion page (plus panel PNGs to S3 for embedding), and never mutates a cluster.

| | |
|---|---|
| **Diagram archetype** | layered-cake (signal) |
| **Visual grammar** | Design 14 · Grammar-version 14.1.0 |
| **Live diagram** | [Open in Lucid](https://lucid.app/lucidchart/542f3aba-186d-44f3-b29d-bcb856db6351/edit) |
| **Skill** | [`SKILL.md`](./SKILL.md) |

## What it does

- Discovers chaos runs from Prometheus (`chaos-<token>-<scenario>` chains, tokens ordered by base36 value), then queries the raw harbor chaos release signals per scenario — restart-aware halt detection on the validator set and bucket-bounded p95 block interval — time-scoped and forced to raw resolution. (TPS + mempool are ~0 by design — chaos runs no load generator — carried transparency-only, not release signals; throughput is the deferred phase-2 benchmark report.)
- Reads the harbor harness Job (joined to the run token by the decoded UnixNano) for the authoritative per-scenario PASS/FAIL and the release image; reconciles against the known 10-scenario set so a missing scenario shows DID NOT RUN and an empty series shows NO DATA — never a green 0.
- Assembles one shareable Notion page per invocation — run-identity header, the liveness headline with caveat, per-scenario sections with provenance markers and window-scoped panel embeds, and a raw-metric appendix.
- The guarantees that matter most: it is read-only and anti-fabricating. When the verdict log is GC'd but metrics survive (the 7d/15d band) it renders VERDICT UNAVAILABLE and suppresses the headline rather than synthesize a pass; a run past the 15d raw bound degrades gracefully to "raw expired."

## Reading the diagram

The layered-cake (signal) archetype stacks the upstream signal sources at the base and composes them upward into the verdict at the top. Here the bottom layers are the live evidence — raw harbor Prometheus metrics and the harness Job pod-log — feeding into the join layer where the authoritative Job-log verdict is annotated with the metric summary, which in turn feeds the narrated per-scenario outcomes and the single Notion report at the crown. Arrows flow upward only: signals compose into judgment, and nothing writes back down into a cluster, reflecting the skill's read-only discipline.
