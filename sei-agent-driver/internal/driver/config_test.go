package driver

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// configEnv is every variable [LoadConfig] reads. Neutralised before each of these
// tests so one sees the defaults rather than whatever the machine running it
// exports -- a developer with OMNIGENT_BASE_URL set would otherwise fail the
// defaults case and pass the override cases for the wrong reason.
var configEnv = []string{
	"OMNIGENT_BASE_URL", "OMNIGENT_ORIGIN", "SEIDROID_AGENT_ID",
	"OMNIGENT_API_TOKEN", "OMNIGENT_API_TOKEN_FILE",
	"OMNIGENT_MACHINE_CLIENT_ID", "OMNIGENT_MACHINE_CLIENT_SECRET",
	"XREVIEW_RUN_DEADLINE_S", "XREVIEW_REQUEST_TIMEOUT_S",
	"XREVIEW_UNARY_TIMEOUT_S", "XREVIEW_STREAM_IDLE_TIMEOUT_S",
}

// clearConfigEnv empties every configuration variable. Empty rather than unset,
// because empty is what this package treats as absent and it is what a workflow
// actually produces; t.Setenv restores whatever was there.
func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, name := range configEnv {
		t.Setenv(name, "")
	}
}

// unset makes a variable genuinely absent, distinct from present-and-empty. The
// Setenv first is what registers the restore.
func unset(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "placeholder")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("unsetting %s: %v", name, err)
	}
}

func TestEnvOrTreatsAnEmptyVariableAsAbsent(t *testing.T) {
	const name = "XREVIEW_TEST_ENVOR"

	t.Run("absent takes the fallback", func(t *testing.T) {
		unset(t, name)
		if got := envOr(name, "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want the fallback", got)
		}
	})

	// The case worth pinning. A workflow writing `env: NAME: ${{ vars.NAME }}`
	// exports NAME= when the variable is undefined, and there is no way to omit the
	// entry from inside the workflow that wrote it. Refusing it would fail every run
	// in the most idiomatic setup there is.
	for _, raw := range []string{"", " ", "\n", "\t\n "} {
		t.Run("blank "+strconv.Quote(raw)+" takes the fallback", func(t *testing.T) {
			t.Setenv(name, raw)
			if got := envOr(name, "fallback"); got != "fallback" {
				t.Errorf("envOr(%q) = %q, want the fallback", raw, got)
			}
		})
	}

	// Trimmed, because the value travels into an HTTP header and a URL, where a
	// heredoc's trailing newline is rejected by something that names neither the
	// variable nor the newline.
	t.Run("a value is trimmed", func(t *testing.T) {
		t.Setenv(name, "  https://host\n")
		if got := envOr(name, "fallback"); got != "https://host" {
			t.Errorf("envOr = %q, want the trimmed value", got)
		}
	})
}

func TestSecondsOrRefusesAValueThatWouldDisableTheBound(t *testing.T) {
	const name = "XREVIEW_TEST_SECONDS"

	// Every rejected shape resolves to zero, which as a duration means an already
	// expired context: a bound that disables itself is worse than a wrong one, so
	// each of these has to be a configuration error rather than a value.
	for _, raw := range []string{"abc", "0", "-1", "-0.5", "NaN", "Inf", "+Inf", "-Inf", "86401", "1e9"} {
		t.Run("refuses "+raw, func(t *testing.T) {
			t.Setenv(name, raw)
			got, err := secondsOr(name, 30)
			if !errors.Is(err, ErrConfig) {
				t.Errorf("secondsOr(%q) error = %v, want ErrConfig", raw, err)
			}
			if got != 0 {
				t.Errorf("secondsOr(%q) = %v, want 0 beside the error", raw, got)
			}
			if err != nil && !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name the variable an operator has to fix", err)
			}
		})
	}

	for _, tc := range []struct {
		name string
		raw  string
		want float64
	}{
		{"absent", "", 30},
		{"blank", "  ", 30},
		{"a whole number", "1200", 1200},
		{"a fraction", "0.5", 0.5},
		{"the ceiling itself", "86400", 86400},
	} {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			if tc.raw == "" {
				unset(t, name)
			} else {
				t.Setenv(name, tc.raw)
			}
			got, err := secondsOr(name, 30)
			if err != nil {
				t.Fatalf("secondsOr(%q) = %v, want no error", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("secondsOr(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestLoadConfigFallsBackToTheDocumentedDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v, want no error with nothing set", err)
	}
	for _, tc := range []struct {
		field string
		got   any
		want  any
	}{
		{"BaseURL", cfg.BaseURL, "http://127.0.0.1:6767"},
		{"Origin", cfg.Origin, "omnigent://internal"},
		{"Agent", cfg.Agent, "seidroid"},
		{"RunDeadline", cfg.RunDeadline, 20 * time.Minute},
		{"RequestTimeout", cfg.RequestTimeout, 30 * time.Second},
		{"UnaryTimeout", cfg.UnaryTimeout, 150 * time.Second},
		{"StreamIdleTimeout", cfg.StreamIdleTimeout, 5 * time.Minute},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.field, tc.got, tc.want)
		}
	}
	// Every duration has to be positive, or the bound it exists to enforce is a
	// context that has already expired.
	if cfg.RunDeadline <= 0 || cfg.RequestTimeout <= 0 || cfg.UnaryTimeout <= 0 || cfg.StreamIdleTimeout <= 0 {
		t.Error("a default duration is not positive; a zero bound expires immediately")
	}
}

func TestLoadConfigTrimsTheTrailingSlashTheSDKWouldDouble(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("OMNIGENT_BASE_URL", "https://omnigent.example.com/// ")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	if cfg.BaseURL != "https://omnigent.example.com" {
		t.Errorf("BaseURL = %q, want it trimmed of every trailing slash", cfg.BaseURL)
	}
}

func TestLoadConfigPrefersAMountedTokenOverAnInlineOne(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  from-the-file\n"), 0o600); err != nil {
		t.Fatalf("writing the token file: %v", err)
	}
	t.Setenv("OMNIGENT_API_TOKEN", "from-the-variable")
	t.Setenv("OMNIGENT_API_TOKEN_FILE", path)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v", err)
	}
	// Trimmed as well as preferred: a mounted secret ends in a newline more often
	// than not, and it travels into an Authorization header.
	if cfg.Token != "from-the-file" {
		t.Errorf("Token = %q, want the file's trimmed contents", cfg.Token)
	}
}

func TestLoadConfigLeavesAnUnreadableTokenFileForRequireAuth(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("OMNIGENT_API_TOKEN_FILE", filepath.Join(t.TempDir(), "absent"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig = %v, want the load itself to succeed", err)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
	// The named variable matters: an operator who mounted the file at the wrong path
	// is not helped by being told to set the inline one.
	if err := cfg.RequireAuth(); !errors.Is(err, ErrConfig) {
		t.Errorf("RequireAuth = %v, want ErrConfig", err)
	}
}

func TestLoadConfigReportsABadDurationAndReturnsNothingUsable(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("XREVIEW_RUN_DEADLINE_S", "twenty minutes")

	cfg, err := LoadConfig()
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("LoadConfig error = %v, want ErrConfig", err)
	}
	// The zero Config, not a half-populated one. A caller that logged the error and
	// carried on would otherwise run with some defaults and some zero bounds.
	if cfg != (Config{}) {
		t.Errorf("Config = %v, want the zero value beside the error", cfg)
	}
}

func TestConfigNeverRendersACredential(t *testing.T) {
	const (
		token  = "sk-ant-thisisthebearertokenvalue"
		secret = "the-machine-client-secret-value"
	)
	cfg := Config{
		BaseURL:             "https://omnigent.example.com",
		Origin:              "omnigent://internal",
		Agent:               "seidroid",
		Token:               token,
		MachineClientID:     "seidroid-machine",
		MachineClientSecret: secret,
		RunDeadline:         20 * time.Minute,
	}

	// Through a real handler, not by calling LogValue directly. What could silently
	// fail is the handler consulting it at all, and only logging a record proves it
	// does.
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("starting", "config", cfg)

	for _, rendered := range []struct {
		how  string
		text string
	}{
		{"slog", buf.String()},
		{"String", cfg.String()},
		{"%v", fmt.Sprintf("%v", cfg)},
		{"%s", fmt.Sprintf("%s", cfg)},
	} {
		if strings.Contains(rendered.text, token) {
			t.Errorf("%s rendered the bearer token", rendered.how)
		}
		if strings.Contains(rendered.text, secret) {
			t.Errorf("%s rendered the machine client secret", rendered.how)
		}
		// Reported as set rather than dropped, because which credential a run used is
		// the first question a 401 raises.
		if !strings.Contains(rendered.text, "token_set=true") {
			t.Errorf("%s does not report that a token was set: %s", rendered.how, rendered.text)
		}
		if !strings.Contains(rendered.text, "machine_client_secret_set=true") {
			t.Errorf("%s does not report that a secret was set", rendered.how)
		}
		// The non-secret fields still have to arrive, or redaction has cost the log
		// its purpose.
		if !strings.Contains(rendered.text, "seidroid") {
			t.Errorf("%s dropped the agent name", rendered.how)
		}
	}
}
