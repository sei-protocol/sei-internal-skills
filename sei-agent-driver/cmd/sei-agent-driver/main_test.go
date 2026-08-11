package main

import "testing"

// TestParseScouts guards the two ways a scout list goes wrong quietly.
//
// A malformed entry must not be skipped: the review would publish having weighed
// fewer opinions than it was configured to hear, and say nothing about it. Two
// scouts under one name must not be accepted: they derive the same run key, so the
// second adopts the first's session and reports its findings back as a second
// reading of the same pull request.
func TestParseScouts(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name    string
		raw     string
		want    []scoutSpec
		wantErr bool
	}{
		{name: "none configured runs the review alone", raw: "  "},
		{
			name: "dispatch order is preserved",
			raw:  "codex=sei-droid-codex, cursor=sei-droid-cursor",
			want: []scoutSpec{
				{name: "codex", agent: "sei-droid-codex"},
				{name: "cursor", agent: "sei-droid-cursor"},
			},
		},
		{name: "missing agent", raw: "codex=", wantErr: true},
		{name: "missing name", raw: "=sei-droid-codex", wantErr: true},
		{name: "no separator", raw: "codex", wantErr: true},
		{name: "duplicate name collides on the run key", raw: "codex=a,codex=b", wantErr: true},
		// The bundle fixes the harness, so both of these produce a reading that is
		// not independent of the thing it is meant to check.
		{name: "scout on the review's own agent", raw: "codex=sei-droid", wantErr: true},
		{name: "two scouts on one agent", raw: "codex=a,cursor=a", wantErr: true},
	} {
		got, err := parseScouts(c.raw, "sei-droid")
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: parseScouts(%q) succeeded; a silently dropped scout makes a "+
					"thinner review look like a full one", c.name, c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: parseScouts(%q) = %v", c.name, c.raw, err)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d scouts, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: scout %d = %+v, want %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
}
