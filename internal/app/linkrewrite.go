package app

import (
	"fmt"
	"regexp"
	"strings"
)

var doubleQuoteHref = regexp.MustCompile(`href="(/wiki/[^"]+)"`)
var singleQuoteHref = regexp.MustCompile(`href='(/wiki/[^']+)'`)

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
	needle := "/wiki/" + target
	return strings.Contains(content, "href=\""+needle) || strings.Contains(content, "href='"+needle)
}
