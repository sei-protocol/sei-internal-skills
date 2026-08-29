# The layout the template teaches

## Success Criteria

- **SC-001**: The suite records one verdict per anchor.
  *Verifier:* `tools/measure.sh`
- **SC-002**: A second reader reaches the same verdict from the stored answer.
  *Verifier:* judgement — a second scorer reads ten answers and compares.
- **SC-003**: Every verdict names its model.
  *Verifier:* not built — no check reads the registry for a missing model.
