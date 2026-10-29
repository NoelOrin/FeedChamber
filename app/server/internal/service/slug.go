package service

import (
	"regexp"
	"strings"
	"unicode"
)

var multiDashRe = regexp.MustCompile(`-+`)

// SlugifyTitle lowercases, replaces spaces/unsafe with -, strips junk, max 80.
// Keeps CJK characters for readable Chinese slugs.
func SlugifyTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	if s == "" {
		return "post"
	}
	// replace whitespace with -
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || unicode.In(r, unicode.Han) {
			b.WriteRune(r)
			prevDash = r == '-'
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := multiDashRe.ReplaceAllString(b.String(), "-")
	out = strings.Trim(out, "-")
	if out == "" {
		out = "post"
	}
	// max 80 runes
	runes := []rune(out)
	if len(runes) > 80 {
		out = string(runes[:80])
		out = strings.Trim(out, "-")
	}
	return out
}
