package output

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestTruncate_ShorterThanMax(t *testing.T) {
	got := Truncate("hello", 10)
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestTruncate_LongerThanMax(t *testing.T) {
	got := Truncate("hello world", 8)
	// max=8, so first 5 chars + "..." = "hello..."
	want := "hello..."
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := Truncate("hello", 5)
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestJSON_OutputsValidJSON(t *testing.T) {
	// Capture stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	input := map[string]string{"key": "value"}
	if err := JSON(input); err != nil {
		w.Close()
		os.Stdout = origStdout
		t.Fatalf("JSON() returned error: %v", err)
	}

	w.Close()
	os.Stdout = origStdout

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, string(out))
	}
	if parsed["key"] != "value" {
		t.Fatalf("expected key=value, got key=%s", parsed["key"])
	}
}
