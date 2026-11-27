// Package links extracts and validates hyperlinks found in skill markdown
// content. It handles both markdown-style links and bare URLs, checks HTTP
// links concurrently, and reports broken or unreachable URLs.
package links

import (
	"regexp"
	"strings"

	"github.com/agent-ecosystem/skill-validator/util"
)

var (
	// mdLinkPattern matches [text](url) markdown links.
	mdLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	// bareURLPattern matches bare URLs starting with http:// or https://.
	bareURLPattern = regexp.MustCompile("(?:^|\\s)(https?://[^\\s<>\\)`]+)")
	// linkTitlePattern matches the optional title that may follow a link
	// destination: "title", 'title', or (title). mdLinkPattern stops at the
	// first ")", so the closing parenthesis of a (title) is already gone.
	linkTitlePattern = regexp.MustCompile(`\s+(?:"[^"]*"|'[^']*'|\([^)]*)$`)
)

// linkDestination returns the destination of a markdown link target,
// without an optional title and without the angle brackets of the
// <destination> form.
func linkDestination(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "<") {
		if end := strings.IndexByte(target, '>'); end > 0 {
			return target[1:end]
		}
	}
	return linkTitlePattern.ReplaceAllString(target, "")
}

// ExtractLinks extracts all unique links from a markdown body.
func ExtractLinks(body string) []string {
	seen := make(map[string]bool)
	var links []string

	// Strip code fences and inline code spans so URLs in code are not extracted.
	cleaned := util.CodeBlockStrip.ReplaceAllString(body, "")
	cleaned = util.InlineCodeStrip.ReplaceAllString(cleaned, "")

	// Markdown links
	for _, match := range mdLinkPattern.FindAllStringSubmatch(cleaned, -1) {
		url := linkDestination(match[2])
		if !seen[url] {
			seen[url] = true
			links = append(links, url)
		}
	}

	// Bare URLs
	for _, match := range bareURLPattern.FindAllStringSubmatch(cleaned, -1) {
		url := trimTrailingDelimiters(strings.TrimSpace(match[1]))
		if !seen[url] {
			seen[url] = true
			links = append(links, url)
		}
	}

	return links
}

var entitySuffix = regexp.MustCompile(`&[a-zA-Z0-9]+;$`)

// trimTrailingDelimiters strips trailing punctuation and entity references
// from bare URLs, following cmark-gfm's autolink delimiter rules.
func trimTrailingDelimiters(url string) string {
	for {
		changed := false

		// Strip trailing HTML entity references (e.g. &amp;)
		if strings.HasSuffix(url, ";") {
			if loc := entitySuffix.FindStringIndex(url); loc != nil {
				url = url[:loc[0]]
				changed = true
				continue
			}
		}

		// Strip unbalanced trailing closing parenthesis
		if strings.HasSuffix(url, ")") {
			open := strings.Count(url, "(")
			close := strings.Count(url, ")")
			if close > open {
				url = url[:len(url)-1]
				changed = true
				continue
			}
		}

		// Strip trailing punctuation
		if len(url) > 0 && strings.ContainsRune("?!.,:*_~'\";<", rune(url[len(url)-1])) {
			url = url[:len(url)-1]
			changed = true
			continue
		}

		if !changed {
			break
		}
	}
	return url
}
