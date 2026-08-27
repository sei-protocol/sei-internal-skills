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
//
// Two rules, and they do different jobs. A shape that could escape a shell is refused
// outright. A shape that is a plain path but names something outside the pull request is
// admitted and then anchored to the tree, which is what makes it harmless -- the
// checkout is a subdirectory, so ".config/gh/hosts.yml" resolves inside it and not onto
// the sandbox's own credential.
func TestAPromptLocationNamesNothingButThePullRequest(t *testing.T) {
	t.Parallel()

	const refused = "(no place in this tree)"

	req := Request{Repo: "sei-protocol/sandbox", PR: 42}
	const tree = "pr-42-tree/"

	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ name, file string }{
			// Expansion. None of these has a leading slash, a parent segment or a
			// space, so no denylist of those three shapes refuses them.
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
			// The shapes the byte set refuses on its own.
			{"an absolute path", "/etc/passwd"},
			{"a parent traversal", "../../etc/passwd"},
			{"a space", "a /etc/passwd"},
			{"empty", ""},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if got := promptLocation(req, tc.file, 12); got != refused {
					t.Errorf("promptLocation(%q) = %q, want %q: the review is told to "+
						"open this", tc.file, got, refused)
				}
			})
		}
	})

	t.Run("admitted", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ name, file, want string }{
			{"a plain path", "internal/omni/host.go", tree + "internal/omni/host.go:12"},
			{"a dot in a name", "go.sum", tree + "go.sum:12"},
			{"a dotfile directory", ".github/workflows/ci.yml", tree + ".github/workflows/ci.yml:12"},
			{"a dash and an underscore", "a-b/c_d.go", tree + "a-b/c_d.go:12"},
			{"a plus", "a+b.go", tree + "a+b.go:12"},
			{"a single dot segment is folded away", "./x.go", tree + "x.go:12"},
			// Plain paths that name something outside the pull request. Each is
			// admitted and confined: the rendered location is inside the checkout,
			// so none of them reaches what its bare form would have.
			{"the gh credential", ".config/gh/hosts.yml", tree + ".config/gh/hosts.yml:12"},
			{"an ssh key", ".ssh/id_ed25519", tree + ".ssh/id_ed25519:12"},
			{"a netrc", ".netrc", tree + ".netrc:12"},
			{"git credentials", ".git-credentials", tree + ".git-credentials:12"},
			// A leading dash an argv would read as an option, defused by the prefix.
			{"an option-shaped path", "-R/etc/passwd", tree + "-R/etc/passwd:12"},
			{"a bare option", "-rf", tree + "-rf:12"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				if got := promptLocation(req, tc.file, 12); got != tc.want {
					t.Errorf("promptLocation(%q) = %q, want %q", tc.file, got, tc.want)
				}
			})
		}
	})

	// A line of zero is not a location, and a refused path stays refused without one.
	t.Run("no line", func(t *testing.T) {
		t.Parallel()
		if got := promptLocation(req, "internal/omni/host.go", 0); got != tree+"internal/omni/host.go" {
			t.Errorf("got %q, want the anchored file alone", got)
		}
		if got := promptLocation(req, "$HOME/.ssh/id_rsa", 0); got != refused {
			t.Errorf("got %q, want %q", got, refused)
		}
	})

	// The cost of the allowlist, recorded rather than left to be discovered: a real path
	// using a byte outside the set keeps its finding and loses its rendering.
	t.Run("the trade", func(t *testing.T) {
		t.Parallel()
		for _, file := range []string{"my notes.md", "café.go", "a@b.go", "a#b.go"} {
			if got := promptLocation(req, file, 12); got != refused {
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

// TestEveryAdmittedLocationIsInsideTheTree is the property the anchoring buys, asserted
// over the union of every fixture above rather than case by case: a location the prompt
// carries either says there is no place, or begins with the checkout directory. Nothing
// in between reaches a path the agent's working directory would resolve elsewhere.
func TestEveryAdmittedLocationIsInsideTheTree(t *testing.T) {
	t.Parallel()

	req := Request{Repo: "sei-protocol/sandbox", PR: 42}
	tree := treePath(req) + "/"

	for _, file := range []string{
		"internal/omni/host.go", ".github/workflows/ci.yml", "./x.go", "go.sum",
		".config/gh/hosts.yml", ".ssh/id_ed25519", ".netrc", ".git-credentials",
		"-R/etc/passwd", "-rf", "a+b.go", "a-b/c_d.go",
		"$HOME/.config/gh/hosts.yml", "${HOME}/x", "$(id)", "`id`", "~/x", "~root/x",
		"/etc/passwd", "../../etc/passwd", "a /etc/passwd", "x.go;id", "", "café.go",
		"a/.././.config/gh/hosts.yml", "...", "a/../..",
	} {
		got := promptLocation(req, file, 12)
		if got == "(no place in this tree)" {
			continue
		}
		if !strings.HasPrefix(got, tree) {
			t.Errorf("promptLocation(%q) = %q: admitted but not anchored to the tree",
				file, got)
		}
	}
}
