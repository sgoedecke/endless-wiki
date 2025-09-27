package app

import (
	"context"
	"strings"
	"testing"
)

func TestGeneratePageHTMLWithoutGroqKeyUsesStub(t *testing.T) {
	cfg := Config{}
	html, err := GeneratePageHTML(context.Background(), nil, cfg, "test_topic")
	if err != nil {
		t.Fatalf("GeneratePageHTML returned error: %v", err)
	}
	if !containsAll(html, []string{"<div class=\"endlesswiki-body\">", "/wiki/test_topic_history"}) {
		t.Fatalf("stub html missing expected structure: %s", html)
	}
}

func containsAll(html string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(html, needle) {
			return false
		}
	}
	return true
}

func TestStripHTMLCodeFence(t *testing.T) {
	input := "```html\n<h1>Example</h1>\n<p>Body</p>\n```\n"
	got := stripHTMLCodeFence(input)
	want := "<h1>Example</h1>\n<p>Body</p>"
	if got != want {
		t.Fatalf("stripHTMLCodeFence html fence: got %q want %q", got, want)
	}

	plain := "<h1>Plain</h1>"
	if stripHTMLCodeFence(plain) != plain {
		t.Fatalf("stripHTMLCodeFence should leave plain content unchanged")
	}

	code := "```\n<h1>Loose</h1>\n```"
	codeStripped := stripHTMLCodeFence(code)
	if codeStripped != "<h1>Loose</h1>" {
		t.Fatalf("stripHTMLCodeFence generic fence: got %q want %q", codeStripped, "<h1>Loose</h1>")
	}
}
