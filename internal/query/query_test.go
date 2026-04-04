package query

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"What is self-attention?", "what-is-self-attention"},
		{"How does Flash Attention work", "how-does-flash-attention-work"},
		{"", ""},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.expected {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSlugifyLong(t *testing.T) {
	long := "this is a very long question that should be truncated to fifty characters maximum for the filename"
	slug := slugify(long)
	if len(slug) > 50 {
		t.Errorf("slug too long: %d chars", len(slug))
	}
}
