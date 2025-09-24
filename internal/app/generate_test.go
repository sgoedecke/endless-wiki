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
	if !containsAll(html, []string{"<div class=\"infiniwiki-body\">", "/wiki/test_topic_history"}) {
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
