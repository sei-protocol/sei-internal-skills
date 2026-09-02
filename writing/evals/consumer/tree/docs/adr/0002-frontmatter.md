---
name: configmap-auditor
description: "A file whose frontmatter carries an identifier."
---

# 2. Seed the vocabulary with hyphenated identifiers

## Status

Accepted.

## Context

Vale reads YAML frontmatter as prose. The `name` above holds an identifier, and
Vale checks it like a sentence.

## Decision

We seed `.vale/vocab/accept.txt` with the hyphenated names this repository uses.

## Consequences

A repository that skips that step gets one finding on every skill file. The
golden beside this fixture records what that looks like.
