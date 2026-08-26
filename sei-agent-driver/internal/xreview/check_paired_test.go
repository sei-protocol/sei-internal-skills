package xreview

import (
	"strings"
	"testing"
)

// TestAnAlreadyEscapedTagStaysInert covers the escape this package adds on top of
// one the model wrote.
//
// A backslash escapes the next character only when the run before it is even, so
// "\<h3>" is inert and "\\<h3>" is a literal backslash beside a live tag. Adding a
// backslash without counting turned the first into the second — reviving exactly
// what the model had already made harmless, on the failure-check path an attacker
// can aim at.
//
// Block starts need no counting, and the last case pins why: the renderer decides
// them from the raw line before it processes escapes, so a leading backslash keeps
// a heading closed at any parity.
func TestAnAlreadyEscapedTagStaysInert(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, in string }{
		{"unescaped", `<h3>Blocking</h3>`},
		{"escaped once by the model", `\<h3>Blocking</h3>`},
		{"a literal backslash before a tag", `\\<h3>Blocking</h3>`},
		{"three", `\\\<h3>Blocking</h3>`},
		{"a tag inside prose", `see \<h3>Blocking</h3> here`},
		{"a block start already escaped", `\### Blocking`},
		{"a literal backslash before a block start", `\\### Blocking`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := defuseMarkup(tc.in)
			if at := liveTagIndex(out); at >= 0 {
				t.Errorf("defuseMarkup(%q) = %q leaves a live tag at %d", tc.in, out, at)
			}
			// A heading must not survive either, at any parity.
			if h := visibleHeadings(out); len(h) != 0 {
				t.Errorf("defuseMarkup(%q) = %q still opens %v", tc.in, out, h)
			}
		})
	}
}

// liveTagIndex reports where a tag-opening "<" survives defusing, or -1.
//
// Counts the backslash run before it rather than looking for one, because one
// backslash and two mean opposite things.
func liveTagIndex(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '<' || i+1 >= len(s) || !tagStart(s[i+1]) {
			continue
		}
		run := 0
		for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
			run++
		}
		if run%2 == 0 {
			return i
		}
	}
	return -1
}

var _ = strings.TrimSpace
