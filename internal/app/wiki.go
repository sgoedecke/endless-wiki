package app

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Page represents a persisted wiki article.
type Page struct {
	Slug      string
	Content   string
	CreatedAt time.Time
}

var slugAllowed = regexp.MustCompile(`^[a-z0-9_\-]+$`)

// NormalizeSlug normalizes raw slug input into the canonical database slug.
func NormalizeSlug(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if strings.ContainsAny(trimmed, "/\\?&:#'\"") || strings.Contains(trimmed, "..") {
		return "", errors.New("slug contains invalid path characters")
	}

	trimmed = stripDiacritics(trimmed)
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	trimmed = strings.ReplaceAll(trimmed, "%20", "_")
	trimmed = normalizeUnicode(trimmed)
	trimmed = strings.ToLower(trimmed)
	trimmed = strings.Trim(trimmed, "_")

	if trimmed == "" {
		return "", errors.New("empty slug")
	}

	// collapse consecutive underscores
	trimmed = regexp.MustCompile(`_+`).ReplaceAllString(trimmed, "_")

	if !slugAllowed.MatchString(trimmed) {
		return "", errors.New("slug contains invalid characters")
	}

	return trimmed, nil
}

func normalizeUnicode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '-' || r == '_':
			b.WriteRune('_')
		default:
			// skip everything else
		}
	}
	return b.String()
}

// SlugTitle converts a slug into a human-friendly title for rendering.
func SlugTitle(slug string) string {
	parts := strings.Split(slug, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

var diacriticStripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func stripDiacritics(s string) string {
	stripped, _, err := transform.String(diacriticStripper, s)
	if err != nil {
		return s
	}
	return stripped
}
