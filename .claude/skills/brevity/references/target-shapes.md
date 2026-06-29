# Target Shapes — Reference PRs and Comment Exemplars

Concrete examples to anchor the abstract rules in SKILL.md against real shipping code.

## PR body — word counts by change type

| Change type | Target words | Section pattern |
|---|---|---|
| Fix (single bug, single file) | 30-60 | Summary (1-2 sentences). Test plan (1-2 bullets). `Fixes #N`. |
| Small refactor (1-5 files, no semantic change) | 30-80 | Summary (1-2 sentences). Test plan (2-3 bullets). |
| Feature / behavior change | 100-250 | Summary (1-3 sentences). Test plan (2-5 bullets). Optional Follow-ups. |
| One-way door (interface, storage, event sig) | +50-100 over feature | Add a 1-paragraph "Alternatives considered" if non-obvious. |
| Multi-package coordinated change | 150-300 | Summary (the load-bearing claim + 1 sentence on the coordination). Test plan. Follow-ups for separable pieces. |

## Reference PR — the floor

[rust-lang/rust#157179](https://github.com/rust-lang/rust/pull/157179) — 28 words:

> Currently rustdoc sorts impls in a couple of places using long HTML string representations of the impls. Using plain text representations instead speeds things up. Details in individual commits.

What it does right:
- States *what* (plain text instead of HTML strings)
- States *why* (faster)
- Routes to detail (individual commits)
- No test plan because the perf-driven change is self-evident — sei-internal-skills would add a 2-bullet test plan to this floor.

## Reference PR — the upper bound for a single-cause bug fix

[kubernetes/kubernetes#139343](https://github.com/kubernetes/kubernetes/pull/139343) — ~120 words. One paragraph of root cause with two file-permalinks, one sentence of fix, `Fixes #N`. Pattern:

1. One-sentence symptom statement.
2. One paragraph naming the root cause with code permalinks.
3. One sentence describing the fix.
4. Test plan (2-3 items).
5. `Fixes #N`.

## In-code comment exemplars

### ✅ Earns its place

```go
// Both checks required: reflect.DeepEqual gives false-positives on equal-but-
// reordered maps (#241). Hash short-circuits the resulting update storm.
// Don't drop either side.
if !reflect.DeepEqual(existing.Spec, desired.Spec) && annotations[lastAppliedKey] != hashOf(desired.Spec) {
```

```go
// Cilium charges BPF maps to the calling cgroup since kernel 5.11; pinned
// caps + 2Gi memcg limit prevent the dynamic-size ENOMEM that motivated this.
extraConfig:
```

```python
# 5s = p99 image-pull + 2x jitter, see platform#719
WARMUP_TIMEOUT_SECONDS = 5
```

### ❌ Does NOT earn its place

```go
// Hash returns a hash of the spec.
func (s Spec) Hash() string {
```

```go
// We loop over the items in the list.
for _, item := range items {
```

```go
// TODO: refactor this later
```

```python
# Initialize the database connection
db = connect(...)
```

## Section pattern for a sei-internal-skills PR body

```markdown
## Summary

<1-3 sentences. Lead with the load-bearing noun + what changed + why.>

## Test plan

- [ ] <concrete verification 1>
- [ ] <concrete verification 2>
- [ ] (optional) <manual check>

## Follow-ups (optional)

- <deferred slice, linked issue>

<`Fixes #N` or design doc link>
```

For a one-way door, add between Summary and Test plan:

```markdown
## Alternatives considered

- **<option>**: <why not, in one sentence>.
- **<option>**: <why not, in one sentence>.
```

Limit alternatives to 2-3 — beyond that, the rejection signal weakens.
