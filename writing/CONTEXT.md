<!-- GENERATED FILE. Source: writing/anchors/registry.yaml. Run writing/scripts/render-context.py. -->
# Writing contract

You write text that another agent or a non-native reader must parse without a
back-channel. Follow the anchors below. Each anchor names a public standard, so you can
resolve it from the name alone. A more specific instruction from the user or from the file
you edit takes precedence on whatever it addresses.

- **ASD-STE100 Simplified Technical English** — approved words in their approved part of
  speech, active voice, simple tenses, one instruction per sentence, no more than three
  nouns in a noun cluster. Procedures: 20 words per sentence. Descriptions: 25 words.
- **RFC 2119** — in a specification, write MUST, SHOULD, and MAY in uppercase. Do not use
  them for a statement that is not a requirement.
- **BLUF** — put the conclusion and the requested action in the first sentence.
- **Diátaxis** — one page is a tutorial, a how-to guide, reference, or explanation. Not
  two.
- **EARS** — write a requirement as ubiquitous, WHEN, IF..THEN, WHILE, or WHERE.
- **ADR (Nygard)** — a decision goes in `docs/adr/` with Status, Context, Decision,
  Consequences.
- **Conventional Commits 1.0.0** — `type(scope)!: description`.

## Verify before you claim compliance

Run `vale <path>` before you report the work as done. A finding names a rule. A rule names
a clause. If you disagree with a finding, say which rule and why. Do not silence a rule to
make the output pass.

Vale checks part of ASD-STE100, not all of it. `vale` exit code 0 means "no finding at or
above the gate", not "compliant".
