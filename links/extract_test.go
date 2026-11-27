package links

import "testing"

func TestExtractLinks(t *testing.T) {
	t.Run("markdown links", func(t *testing.T) {
		body := "See [guide](references/guide.md) and [docs](https://example.com/docs)."
		links := ExtractLinks(body)
		if len(links) != 2 {
			t.Fatalf("expected 2 links, got %d: %v", len(links), links)
		}
		if links[0] != "references/guide.md" {
			t.Errorf("links[0] = %q, want %q", links[0], "references/guide.md")
		}
		if links[1] != "https://example.com/docs" {
			t.Errorf("links[1] = %q, want %q", links[1], "https://example.com/docs")
		}
	})

	t.Run("bare URLs", func(t *testing.T) {
		body := "Visit https://example.com for details.\nAlso http://other.com/page"
		links := ExtractLinks(body)
		if len(links) != 2 {
			t.Fatalf("expected 2 links, got %d: %v", len(links), links)
		}
		if links[0] != "https://example.com" {
			t.Errorf("links[0] = %q, want %q", links[0], "https://example.com")
		}
		if links[1] != "http://other.com/page" {
			t.Errorf("links[1] = %q, want %q", links[1], "http://other.com/page")
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		body := "[link1](https://example.com) and [link2](https://example.com) and https://example.com"
		links := ExtractLinks(body)
		if len(links) != 1 {
			t.Fatalf("expected 1 deduplicated link, got %d: %v", len(links), links)
		}
	})

	t.Run("no links", func(t *testing.T) {
		body := "Just plain text with no links at all."
		links := ExtractLinks(body)
		if len(links) != 0 {
			t.Fatalf("expected 0 links, got %d: %v", len(links), links)
		}
	})

	t.Run("mixed link types", func(t *testing.T) {
		body := "[file](scripts/run.sh)\n[site](https://example.com)\nmailto:user@example.com\n#anchor"
		links := ExtractLinks(body)
		if len(links) != 2 {
			t.Fatalf("expected 2 links (markdown only), got %d: %v", len(links), links)
		}
	})

	t.Run("bare URL in code span is ignored", func(t *testing.T) {
		body := "`curl https://example.com/docs` and https://example.com/real"
		links := ExtractLinks(body)
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d: %v", len(links), links)
		}
		if links[0] != "https://example.com/real" {
			t.Errorf("links[0] = %q, want %q", links[0], "https://example.com/real")
		}
	})

	t.Run("URL in fenced code block is ignored", func(t *testing.T) {
		body := "```bash\ncurl https://example.com/api\n```"
		links := ExtractLinks(body)
		if len(links) != 0 {
			t.Fatalf("expected 0 links, got %d: %v", len(links), links)
		}
	})

	t.Run("URL in tilde-fenced code block is ignored", func(t *testing.T) {
		body := "~~~bash\ncurl https://example.com/api\n~~~"
		links := ExtractLinks(body)
		if len(links) != 0 {
			t.Fatalf("expected 0 links, got %d: %v", len(links), links)
		}
	})

	t.Run("URL outside code block still extracted", func(t *testing.T) {
		body := "```bash\ncurl https://example.com/api\n```\nVisit https://example.com/real for details."
		links := ExtractLinks(body)
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d: %v", len(links), links)
		}
		if links[0] != "https://example.com/real" {
			t.Errorf("links[0] = %q, want %q", links[0], "https://example.com/real")
		}
	})

	t.Run("link title is not part of the destination", func(t *testing.T) {
		body := `[a](references/a.md "Guide A") [b](references/b.md 'Guide B') ` +
			`[c](references/c.md (Guide C)) [d](https://example.com/d "Docs")`
		links := ExtractLinks(body)
		want := []string{"references/a.md", "references/b.md", "references/c.md", "https://example.com/d"}
		if len(links) != len(want) {
			t.Fatalf("expected %d links, got %d: %q", len(want), len(links), links)
		}
		for i := range want {
			if links[i] != want[i] {
				t.Errorf("links[%d] = %q, want %q", i, links[i], want[i])
			}
		}
	})

	t.Run("angle-bracket destination", func(t *testing.T) {
		body := `See [guide](<references/my guide.md> "Guide").`
		links := ExtractLinks(body)
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d: %q", len(links), links)
		}
		if links[0] != "references/my guide.md" {
			t.Errorf("links[0] = %q, want %q", links[0], "references/my guide.md")
		}
	})

	t.Run("empty link text", func(t *testing.T) {
		body := "[](references/empty.md)"
		links := ExtractLinks(body)
		if len(links) != 1 {
			t.Fatalf("expected 1 link, got %d: %v", len(links), links)
		}
		if links[0] != "references/empty.md" {
			t.Errorf("links[0] = %q, want %q", links[0], "references/empty.md")
		}
	})
}

func TestTrimTrailingDelimiters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"trailing period", "https://example.com.", "https://example.com"},
		{"trailing comma", "https://example.com,", "https://example.com"},
		{"trailing exclamation", "https://example.com!", "https://example.com"},
		{"trailing question mark", "https://example.com?", "https://example.com"},
		{"query string preserved", "https://example.com?q=test", "https://example.com?q=test"},
		{"path with extension", "https://example.com/file.html", "https://example.com/file.html"},
		{"balanced parens", "https://en.wikipedia.org/wiki/Foo_(bar)", "https://en.wikipedia.org/wiki/Foo_(bar)"},
		{"unbalanced trailing paren", "https://example.com)", "https://example.com"},
		{"entity reference", "https://example.com&amp;", "https://example.com"},
		{"multiple trailing", "https://example.com.\"", "https://example.com"},
		{"no trimming needed", "https://example.com/path", "https://example.com/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimTrailingDelimiters(tt.in)
			if got != tt.want {
				t.Errorf("trimTrailingDelimiters(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
