package app

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var doubleQuoteHref = regexp.MustCompile(`href="(/wiki/[^"]+)"`)
var singleQuoteHref = regexp.MustCompile(`href='(/wiki/[^']+)'`)
var wikiHref = regexp.MustCompile(`href=['"](/wiki/[^'"#?]+(?:[^'" ]*)?)['"]`)

func decorateInternalLinks(content, origin string) string {
	if origin == "" {
		return content
	}

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
