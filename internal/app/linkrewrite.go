package app

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var doubleQuoteHref = regexp.MustCompile(`href="(/wiki/[^"]+)"`)
var singleQuoteHref = regexp.MustCompile(`href='(/wiki/[^']+)'`)
var wikiHref = regexp.MustCompile(`href=['"](/wiki/[^'"#?]+(?:[^'" ]*)?)['"]`)
var anchorTag = regexp.MustCompile(`(?i)<a\b([^>]*?)href=(['"])(/wiki/[^'"#?]+(?:[^'" ]*)?)(['"])([^>]*)>`)

func decorateInternalLinks(content, origin string, missing map[string]struct{}) string {
	if origin != "" {
		content = addOriginParam(content, origin)
	}
	if len(missing) > 0 {
		content = markMissingLinks(content, missing)
	}
	return content
}

func addOriginParam(content, origin string) string {
	rewrite := func(match string, re *regexp.Regexp) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		href := sub[1]
		if strings.Contains(href, "origin=") {
			return match
		}

		href = injectOrigin(href, origin)
		if re == doubleQuoteHref {
			return fmt.Sprintf("href=\"%s\"", href)
		}
		return fmt.Sprintf("href='%s'", href)
	}

	content = doubleQuoteHref.ReplaceAllStringFunc(content, func(s string) string {
		return rewrite(s, doubleQuoteHref)
	})

	content = singleQuoteHref.ReplaceAllStringFunc(content, func(s string) string {
		return rewrite(s, singleQuoteHref)
	})

	return content
}

func markMissingLinks(content string, missing map[string]struct{}) string {
	matches := anchorTag.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}

	type replacement struct {
		start int
		end   int
		text  string
	}

	replacements := make([]replacement, 0, len(matches))
	for _, loc := range matches {
		tagStart := loc[0]
		tagEnd := loc[1]
		hrefStart := loc[6]
		hrefEnd := loc[7]
		hrefValue := content[hrefStart:hrefEnd]
		slug := slugFromHref(hrefValue)
		if slug == "" {
			continue
		}
		if _, ok := missing[slug]; !ok {
			continue
		}

		restLower := strings.ToLower(content[tagEnd:])
		closingIdx := strings.Index(restLower, "</a>")
		if closingIdx == -1 {
			continue
		}
		anchorEnd := tagEnd + closingIdx + len("</a>")
		inner := content[tagEnd : anchorEnd-len("</a>")]
		span := buildMissingLinkSpan(hrefValue, inner)
		replacements = append(replacements, replacement{start: tagStart, end: anchorEnd, text: span})
	}

	if len(replacements) == 0 {
		return content
	}

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start < replacements[j].start
	})

	var b strings.Builder
	last := 0
	for _, rep := range replacements {
		if rep.start < last {
			continue
		}
		b.WriteString(content[last:rep.start])
		b.WriteString(rep.text)
		last = rep.end
	}
	b.WriteString(content[last:])
	return b.String()
}

func buildMissingLinkSpan(href, inner string) string {
	var b strings.Builder
	b.WriteString(`<span class="new-page-link" role="link" tabindex="0" data-href="`)
	b.WriteString(html.EscapeString(href))
	b.WriteString(`">`)
	b.WriteString(inner)
	b.WriteString(`</span>`)
	return b.String()
}

func injectOrigin(href, origin string) string {
	fragment := ""
	if idx := strings.Index(href, "#"); idx >= 0 {
		fragment = href[idx:]
		href = href[:idx]
	}

	if strings.Contains(href, "?") {
		href = href + "&origin=" + origin
	} else {
		href = href + "?origin=" + origin
	}

	return href + fragment
}

func hasLinkTo(content, target string) bool {
	target = strings.ToLower(target)
	for _, slug := range ExtractLinkedSlugs(content) {
		if slug == target {
			return true
		}
	}
	return false
}

// ExtractLinkedSlugs returns normalised wiki slugs referenced within HTML content.
func ExtractLinkedSlugs(content string) []string {
	matches := wikiHref.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		href := match[1]
		slug := slugFromHref(href)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; !ok {
			seen[slug] = struct{}{}
		}
	}

	result := make([]string, 0, len(seen))
	for slug := range seen {
		result = append(result, slug)
	}
	return result
}

func slugFromHref(href string) string {
	withoutFragment := href
	if idx := strings.IndexAny(withoutFragment, "?#"); idx >= 0 {
		withoutFragment = withoutFragment[:idx]
	}

	trimmed := strings.TrimPrefix(withoutFragment, "/wiki/")
	if trimmed == "" {
		return ""
	}

	decoded, err := url.PathUnescape(trimmed)
	if err != nil {
		return ""
	}

	slug, err := NormalizeSlug(decoded)
	if err != nil {
		return ""
	}

	return slug
}
