package xreview

import (
	"strings"
	"testing"
)

// TestAPromptLocationNamesNothingButThePullRequest is the guard between a scout's model
// output and a shell in a sandbox holding a live credential.
//
// The review is told to open what a claim names, and the policy the workflow passes
// permits Bash, so a value that expands is a value that reads whatever it expands to. A
// refused claim is not dropped -- it reaches the review as text -- so refusing wrongly
// costs a rendering, and admitting wrongly costs the credential.
//
// Driven through promptLocation rather than the predicate, because the check runs on the
// flattened and clipped value and that is what a prompt carries.
func TestAPromptLocationNamesNothingButThePullRequest(t *testing.T) {
	t.Parallel()

	const refused = "(no place in this tree)"

	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ name, file string }{
			// Expansion. None of these has a leading slash, a parent segment or a
			// space, so every one passed the rules that came before.
			{"a shell variable", "$HOME/.config/gh/hosts.yml"},
			{"a braced variable", "${HOME}/.config/gh/hosts.yml"},
			{"a variable mid-path", "src/$HOME/x.go"},
			{"a command substitution", "$(id)"},
			{"a backtick substitution", "`id`"},
			{"a tilde in front", "~/.config/gh/hosts.yml"},
			{"a tilde on a user", "~root/.ssh/id_rsa"},
			{"a glob", "*/../../etc/passwd"},
			{"a brace expansion", "{a,b}/x.go"},
			// Separators, which would make the location a second command.
			{"a semicolon", "x.go;id"},
			{"a pipe", "x.go|id"},
			{"an ampersand", "x.go&id"},
			{"a redirect", "x.go>out"},
			{"a newline", "x.go\nid"},
			// The rules that stood alone before, which the byte set now owns.
			{"an absolute path", "/etc/passwd"},
			{"a parent traversal", "../../etc/passwd"},
			{"a space", "a /etc/passwd"},
			{"empty", ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if got := promptLocation(tc.file, 12); got != refused {
					t.Errorf("promptLocation(%q) = %q, want %q: the review is told to "+
						"open this", tc.file, got, refused)
				}
			})
		}
	})

	t.Run("admitted", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ name, file, want string }{
			{"a plain path", "internal/omni/host.go", "internal/omni/host.go:12"},
			{"a dot in a name", "go.sum", "go.sum:12"},
			{"a dotfile", ".github/workflows/ci.yml", ".github/workflows/ci.yml:12"},
			{"a dash and an underscore", "a-b/c_d.go", "a-b/c_d.go:12"},
			{"a plus", "a+b.go", "a+b.go:12"},
			{"a single dot segment", "./x.go", "./x.go:12"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if got := promptLocation(tc.file, 12); got != tc.want {
					t.Errorf("promptLocation(%q) = %q, want %q", tc.file, got, tc.want)
				}
			})
		}
	})

	// A line of zero is not a location, and a refused path stays refused without one.
	t.Run("no line", func(t *testing.T) {
		t.Parallel()
		if got := promptLocation("internal/omni/host.go", 0); got != "internal/omni/host.go" {
			t.Errorf("got %q, want the file alone", got)
		}
		if got := promptLocation("$HOME/.ssh/id_rsa", 0); got != refused {
			t.Errorf("got %q, want %q", got, refused)
		}
	})

	// The cost of the allowlist, recorded rather than left to be discovered: a real path
	// using a byte outside the set keeps its finding and loses its rendering.
	t.Run("the trade", func(t *testing.T) {
		t.Parallel()
		for _, file := range []string{"my notes.md", "café.go", "a@b.go", "a#b.go"} {
			if got := promptLocation(file, 12); got != refused {
				t.Errorf("promptLocation(%q) = %q; if this now renders, the byte set "+
					"widened and the doc has to say so", file, got)
			}
		}
	})
}

// TestEveryPromptLocationByteIsAccountedFor pins the set itself, so widening it is a
// deliberate edit rather than a side effect.
func TestEveryPromptLocationByteIsAccountedFor(t *testing.T) {
	t.Parallel()

	const want = "+-./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_" +
		"abcdefghijklmnopqrstuvwxyz"

	var got strings.Builder
	for c := 0; c < 256; c++ {
		if pathBytesOnly(string([]byte{byte(c)})) {
			got.WriteByte(byte(c))
		}
	}
	if got.String() != want {
		t.Errorf("accepted bytes = %q\n                want %q", got.String(), want)
	}
}
