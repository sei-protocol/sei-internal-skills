<!-- Golden fixture for the generated section rules. Ticket-Problem stands for all
     eighteen: they share one generated body, so the fence behaviour is the same in
     each. The heading below sits inside a fenced code block and must not count.
     Before these rules tracked the fence, it did, and a document that only showed
     the required format satisfied the check.

     The last case is a four-space indented code block, which is a code block by
     CommonMark and not a fence. The rule handles it because an ATX heading takes
     at most three spaces of indentation; a wider pattern read that line as a real
     heading and the rule went silent. -->

# A ticket that documents the format without using it

The house body opens with:

```markdown
## Problem

State the problem.
```

~~~
## Problem
~~~

    ## Problem

## Impact

Only Ticket-Problem runs here, so the missing sections above it are not the point.
The point is that four quoted headings do not add up to one real one.
