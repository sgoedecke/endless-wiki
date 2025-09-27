package app

import (
	"strings"
	"testing"
)

func TestDecorateInternalLinksConvertsMissingAnchors(t *testing.T) {
	content := `<p><a href="/wiki/made_up">New</a> and <a href="/wiki/existing">Old</a></p>`
	missing := map[string]struct{}{"made_up": {}}

	result := decorateInternalLinks(content, "source_page", missing)

	if !contains(result, `class="new-page-link"`) {
		t.Fatalf("missing link was not converted to span: %s", result)
	}
	if !contains(result, `data-href="/wiki/made_up?origin=source_page"`) {
		t.Fatalf("missing link data-href missing origin: %s", result)
	}
	if contains(result, `<a href="/wiki/made_up?origin=source_page"`) {
		t.Fatalf("missing link anchor should not remain: %s", result)
	}
	if !contains(result, `<a href="/wiki/existing?origin=source_page"`) {
		t.Fatalf("existing link did not retain anchor: %s", result)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
