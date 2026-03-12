package commands

import "regexp"

// stripHTMLTags removes HTML tags from user-created content to prevent
// terminal escape injection and keep posts as plain text.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}
