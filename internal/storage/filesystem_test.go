package storage

import "testing"

func TestComputeHash_Deterministic(t *testing.T) {
	h1 := ComputeHash("hello")
	h2 := ComputeHash("hello")
	if h1 != h2 {
		t.Errorf("hash should be deterministic, got %q vs %q", h1, h2)
	}
	if h1 == ComputeHash("world") {
		t.Error("different content should produce different hash")
	}
}

func TestSanitizeFilename(t *testing.T) {
	got := sanitizeFilename("foo/bar:baz?.md")
	want := "foo-bar-baz-"
	if got != want {
		t.Errorf("sanitizeFilename = %q, want %q", got, want)
	}
}
