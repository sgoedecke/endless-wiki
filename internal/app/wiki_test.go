package app

import "testing"

func TestNormalizeSlug(t *testing.T) {
	tests := map[string]string{
		"Main Page":      "main_page",
		"Quantum-Flux":   "quantum_flux",
		"  spaced out  ": "spaced_out",
		"Emoji😀Test":     "emojitest",
	}

	for input, want := range tests {
		got, err := NormalizeSlug(input)
		if err != nil {
			t.Fatalf("NormalizeSlug(%q) unexpected error: %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeSlugInvalid(t *testing.T) {
	inputs := []string{"", "../etc/passwd", "white space?", "Привет"}
	for _, input := range inputs {
		if _, err := NormalizeSlug(input); err == nil {
			t.Fatalf("NormalizeSlug(%q) expected error", input)
		}
	}
}

func TestSlugTitle(t *testing.T) {
	if got, want := SlugTitle("main_page"), "Main Page"; got != want {
		t.Fatalf("SlugTitle() = %q, want %q", got, want)
	}
	if got, want := SlugTitle("orbital_gardening"), "Orbital Gardening"; got != want {
		t.Fatalf("SlugTitle() = %q, want %q", got, want)
	}
}
