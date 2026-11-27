package util

import "regexp"

// CodeBlockStrip removes fenced code blocks (backtick and tilde) from markdown.
// The info string after the opening fence may hold any characters except a
// backtick (c++, objective-c, shell-session, "bash title=x"), as in CommonMark.
var CodeBlockStrip = regexp.MustCompile("(?s)(?:```|~~~)[^`\\r\\n]*\\r?\\n.*?(?:```|~~~)")

// InlineCodeStrip removes inline code spans from markdown.
var InlineCodeStrip = regexp.MustCompile("`[^`]+`")

// CodeBlockPattern extracts fenced code block bodies (capture group 1) from markdown.
var CodeBlockPattern = regexp.MustCompile("(?s)(?:```|~~~)[^`\\r\\n]*\\r?\\n(.*?)(?:```|~~~)")
