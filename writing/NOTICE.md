# Third-party standards and the licensing boundary

This repository refers to external standards. It does not republish them. Read this
before you add an anchor or a rule.

## Rule of thumb

Cite the clause. Do not copy the clause. A rule file may contain a rule identifier, a URL,
and a description in your own words. A rule file must not contain the text of a
copyrighted standard, and must not reproduce a controlled dictionary.

## ASD-STE100 (Simplified Technical English)

- Owner: ASD (AeroSpace, Security and Defence Industries Association of Europe).
- Maintainer: STE Maintenance Group (STEMG).
- Current issue: Issue 9, January 2025. It has 53 writing rules and a dictionary of
  approximately 900 approved words.
- Source: <https://www.asd-ste100.org/>
- Availability: the specification is free to download after a request to STEMG.
  It is **not** openly licensed. Redistribution and derivative works are restricted.

Consequences for this repository:

1. No file here contains the specification text or the approved-word dictionary.
2. Rules reference rule numbers and describe the constraint in original words.
3. The vocabulary check derives from the separately licensed OpenSTE wordset
   (MIT), not from the ASD dictionary. See `writing/scripts/sync-openste.sh`.
4. Disclaimer, which must stay in the README of any fork: this work is an
   approximation of ASD-STE100. It is not certified. It is not affiliated with or
   endorsed by ASD or STEMG. A passing Vale run is not a certificate of compliance.

## OpenSTE wordset

- Source: <https://github.com/openste/openste>
- License: MIT. Redistribution is allowed with attribution.
- Used to generate the approved-word substitution rule and `accept.txt`.

## Other standards referenced

| Standard | Steward | Notes |
|---|---|---|
| RFC 2119 | IETF | Public specification. Quotation is normal practice. |
| Conventional Commits | community | CC BY 3.0. |
| Semantic Versioning | Tom Preston-Werner | CC BY 3.0. |
| Diátaxis | Daniele Procida | Public framework, documented at diataxis.fr. |
| arc42 | Gernot Starke, Peter Hruschka | Template is free to use; check the site for terms. |
| ADR | Michael Nygard | Public blog post, widely reimplemented. |
| EARS | Alistair Mavin | Published papers; the syntax itself is short and public. |
| ISO 24495-1 (plain language) | ISO | Paywalled. Reference it; do not copy it. |
| Vale | `vale-cli` | MIT. |

If you cannot state the license of a standard, do not add an anchor for it.
