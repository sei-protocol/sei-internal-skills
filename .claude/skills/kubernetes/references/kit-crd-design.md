# CRD design kit

## 1. What this concern is

CRD design here is **contract design under the one-way-door law**: a served-version spec field's shape, validation, and semantics cannot change incompatibly once a controller or user depends on it. sei-k8s-controller leans hard on **CEL `XValidation`** for invariants the API server enforces at admission (there is no webhook server), and on **discriminated-union** specs. The generic habit — "add/loosen a field freely, fix it later" — is exactly the trap. *Cited:* `api/v1alpha1/*_types.go`; `sources.md` §api_changes (the compatibility law), §crd-versioning.

## 2. The pattern (how this repo does it)

- **spec vs status.** Desired in `spec`, observed in `status`; status is a subresource; status reconstructed each reconcile (never read to decide). *Cited:* `sources.md` §api-conventions; profile §3.
- **Discriminated unions with CEL exactly-one.** `SeiNode` spec is exactly-one-of `fullNode | archive | replayer | validator`; `SeiNodeTask` is an 8-kind union — enforced by CEL `XValidation`, not Go-side only. *Cited:* `api/v1alpha1/seinode_types.go`, `seinodetask_types.go`.
- **CEL immutability for create-only fields.** `self == oldSelf` on fields that are one-way doors at the domain level: `replicas` / `genesis` / `dataVolume` (SeiNetwork), `chainId`, validator key Secret refs (rotating a live consensus key is a slashing risk). *Cited:* the `_types.go` `+kubebuilder:validation:XValidation` markers.
- **Raw passthrough is `apiextensionsv1.JSON`.** Genesis overrides and gov param-change values are `JSON` — and carry the double-encode trap (profile §7: pass a structured object, not a pre-escaped string; integer params as JSON strings). *Cited:* `seinodetask_types.go:329-347`.
- **Generated artifacts.** Edit `api/v1alpha1/`, then `make manifests generate`; never hand-edit `manifests/` or `zz_generated.deepcopy.go`. Deprecated fields are **retained**, not removed. *Cited:* profile §5.
- **The `kubectl wait` latch (the documented condition exception).** `SeiNodeTask.Status.Conditions[Ready|Failed]` latch independently on terminal state (mixed polarity) because the seitask-runner depends on `kubectl wait --for=condition=Ready=true` / `=Failed=true`. This is the *sanctioned* exception to always-present-conditions (profile §3). *Cited:* `seinodetask_types.go:84-102`.

## 3. Anti-patterns / failure modes

- **Incompatible in-version change.** Removing a served field, narrowing its validation, tightening optional→required, or changing its meaning. Cue: an edit to an existing `v1alpha1` field's type/validation/semantics. Rewrite: add a new field (optional) or a new version + storage-version/conversion; flag the one-way door.
- **Go-only invariant.** Enforcing exactly-one / immutability only in reconcile code, not CEL. Cue: a union or create-only field with no `XValidation`. Rewrite: add the CEL marker so the API server rejects bad specs at admission.
- **Hand-edited generated files.** Cue: a diff touching `zz_generated.deepcopy.go` / `manifests/` directly. Rewrite: edit types, `make manifests generate`.
- **Condition-by-removal.** Deleting a condition to mean "off" (banned, profile §3) — except the `SeiNodeTask` Ready/Failed latch, which is the documented exception, not a license to mix polarity elsewhere.
- **Double-encoded `JSON` value.** A pre-escaped string where a structured object belongs (profile §7).

## 4. Review cues

- **Dimension 2 (CRD-contract durability):** no incompatible in-version change; CEL invariants present; generated files regenerated not hand-edited; deprecated-retained. *Basis:* `sources.md` §api_changes/§crd-versioning, profile §5.
- **Dimension 5 (observability):** status subresource; conditions always-present + reason-as-API + `observedGeneration` (with the `kubectl wait` latch as the only mixed-polarity exception). *Basis:* profile §3, `sources.md` §conditions.

## 5. One-way doors in this concern

- **Any served-version field's shape/validation/semantics** once consumed — the compatibility law. Flag for human approval; route through a new version.
- **CEL immutability on a domain one-way-door field** (chainId, validator key refs) — relaxing it is a safety regression (consensus-key rotation = slashing risk).
- **The `kubectl wait` Ready/Failed condition contract** on `SeiNodeTask` — the seitask-runner depends on it; changing the condition types/polarity breaks that consumer.
