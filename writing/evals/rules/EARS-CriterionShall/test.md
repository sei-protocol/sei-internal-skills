# Requirements Document

## Requirements

### Requirement 1: Token acceptance

#### Acceptance Criteria

1. When a request carries a valid scoped token in the header, the Parser shall
   authenticate the request as the token's owner.
2. The system should probably handle a bad token somehow.
3. WHEN the token expires, THE Parser SHALL reject the request.
4. Users can also pass the token as a query parameter.

### Requirement 2: Grouped criteria

#### Acceptance Criteria

##### Group A: rejection

1. If the token is expired, then the Parser shall reject the request.
2. The parser returns 401.

### Requirement 3: Criteria with a code sample

#### Acceptance Criteria

1. WHERE the config names a header, THE Parser SHALL read it.

The criterion above refers to this shape:

```yaml
# a comment that looks like a heading
key: value
```

2. The parser needs a fallback.

## Notes

These are steps, not criteria, and must never be flagged:

1. Install the proxy.
2. Start the agent.

Documenting the required format, which must not switch the section on:

```markdown
#### Acceptance Criteria

1. This is a code sample and is not a criterion.
```
