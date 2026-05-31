# Judge: narration_comments (LLM-judged)

## Rule

A comment that restates the identifier on the line below it adds zero signal and should be deleted. Function-doc style is the v1 scope:

```go
❌ // Hash returns a hash of the spec.
func (s Spec) Hash() string { ... }

✅ (delete entirely — function signature says this)
```

## Scope

- Files matching `*.go`, `*.py`, `*.ts`
- Only comment lines IMMEDIATELY ABOVE a `func`, `def`, or `function` declaration. v1 does NOT judge inline or multi-line block comments elsewhere.

## Few-shot examples (5: 3 violations + 2 non-violations)

**Violation 1**: `// Hash returns a hash of the spec.` above `func (s Spec) Hash() string`
**Violation 2**: `# Initialize the database connection` above `def connect(...)`
**Violation 3**: `// ChainID is the chain ID.` above `ChainID string`
**Non-violation 1**: `// Both checks required: reflect.DeepEqual gives false-positives on equal-but-reordered maps (#241).` above `if !reflect.DeepEqual(...)` — earns its place (non-obvious WHY, links source-of-truth).
**Non-violation 2**: `// ChainID without the chain-prefix (e.g. "pacific-1" not "sei-pacific-1").` above `ChainID string` — disambiguates a non-obvious format.

## Self-consistency

n=3 samples, temp=0.3, require 2/3 agreement for any finding.

## Cites

Memory: `feedback_narration_comments` — "narration comments are a smell — drop comments that restate names; lift complex context to file/package doc"
