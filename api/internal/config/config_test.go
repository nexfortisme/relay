package config

import "testing"

func TestSanitizeEnvValueTrimsInlineComments(t *testing.T) {
	got := sanitizeEnvValue(" http://localhost:1234/v1  # local model ")
	want := "http://localhost:1234/v1"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEnvIntHelpersUseFallbackForInvalidValues(t *testing.T) {
	t.Setenv("RELAY_TEST_INT", "42")
	if got := envIntOrDefault("RELAY_TEST_INT", 7); got != 42 {
		t.Fatalf("expected parsed int 42, got %d", got)
	}

	t.Setenv("RELAY_TEST_INT", "-1")
	if got := envIntOrDefault("RELAY_TEST_INT", 7); got != 7 {
		t.Fatalf("expected fallback for negative int, got %d", got)
	}

	t.Setenv("RELAY_TEST_INT64", "not-a-number")
	if got := envInt64OrDefault("RELAY_TEST_INT64", 9); got != 9 {
		t.Fatalf("expected fallback for invalid int64, got %d", got)
	}
}
