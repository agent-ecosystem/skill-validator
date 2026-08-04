package links

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agent-ecosystem/skill-validator/types"
)

// writeFile creates a file at dir/relPath with the given content, creating directories as needed.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// requireResultContaining asserts that at least one result has the given level and message containing substr.
func requireResultContaining(t *testing.T, results []types.Result, level types.Level, substr string) {
	t.Helper()
	for _, r := range results {
		if r.Level == level && strings.Contains(r.Message, substr) {
			return
		}
	}
	t.Errorf("expected result with level=%d message containing %q, got:", level, substr)
	for _, r := range results {
		t.Logf("  level=%d category=%s message=%q", r.Level, r.Category, r.Message)
	}
}

func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func TestCheckLinks_SkipsRelative(t *testing.T) {
	t.Run("relative-only links returns nil", func(t *testing.T) {
		dir := t.TempDir()
		body := "See [guide](references/guide.md)."
		results := CheckLinks(t.Context(), dir, body)
		if results != nil {
			t.Errorf("expected nil for relative-only links, got %v", results)
		}
	})

	t.Run("mailto and anchors are skipped", func(t *testing.T) {
		dir := t.TempDir()
		body := "[email](mailto:user@example.com) and [section](#heading)"
		results := CheckLinks(t.Context(), dir, body)
		if results != nil {
			t.Errorf("expected nil for mailto/anchor links, got %v", results)
		}
	})

	t.Run("template URLs are skipped", func(t *testing.T) {
		dir := t.TempDir()
		body := "[PR](https://github.com/{OWNER}/{REPO}/pull/{PR}) and https://api.example.com/{version}/users/{id}"
		results := CheckLinks(t.Context(), dir, body)
		if results != nil {
			t.Errorf("expected nil for template URLs, got %v", results)
		}
	})

	t.Run("no links returns nil", func(t *testing.T) {
		dir := t.TempDir()
		body := "No links here."
		results := CheckLinks(t.Context(), dir, body)
		if results != nil {
			t.Errorf("expected nil for no links, got %v", results)
		}
	})
}

func TestCheckLinks_HTTP(t *testing.T) {
	// httptest servers listen on 127.0.0.1, which the safe transport blocks.
	// Swap in an unrestricted client for these integration tests.
	orig := newHTTPClient
	newHTTPClient = func() *http.Client { return testHTTPClient() }
	t.Cleanup(func() { newHTTPClient = orig })

	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/head-404-get-200", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/head-405-get-200", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/forbidden", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/server-error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("successful HTTP link", func(t *testing.T) {
		dir := t.TempDir()
		body := "[ok](" + server.URL + "/ok)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Pass, "HTTP 200")
	})

	t.Run("404 HTTP link", func(t *testing.T) {
		dir := t.TempDir()
		body := "[missing](" + server.URL + "/not-found)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Error, "HTTP 404")
	})

	t.Run("403 HTTP link", func(t *testing.T) {
		dir := t.TempDir()
		body := "[blocked](" + server.URL + "/forbidden)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Info, "HTTP 403")
	})

	t.Run("500 HTTP link", func(t *testing.T) {
		dir := t.TempDir()
		body := "[error](" + server.URL + "/server-error)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Error, "HTTP 500")
	})

	t.Run("HEAD 404 falls back to GET 200", func(t *testing.T) {
		dir := t.TempDir()
		body := "[spa](" + server.URL + "/head-404-get-200)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Pass, "HTTP 200")
	})

	t.Run("HEAD 405 falls back to GET 200", func(t *testing.T) {
		dir := t.TempDir()
		body := "[nohead](" + server.URL + "/head-405-get-200)"
		results := CheckLinks(t.Context(), dir, body)
		requireResultContaining(t, results, types.Pass, "HTTP 200")
	})

	t.Run("mixed relative and HTTP only checks HTTP", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "content")
		body := "[guide](references/guide.md) and [site](" + server.URL + "/ok)"
		results := CheckLinks(t.Context(), dir, body)
		if len(results) != 1 {
			t.Fatalf("expected 1 result (HTTP only), got %d", len(results))
		}
		requireResultContaining(t, results, types.Pass, "HTTP 200")
	})
}

func testHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}}
}

func TestCheckLinks_ConcurrencyAndTruncation(t *testing.T) {
	orig := newHTTPClient
	newHTTPClient = func() *http.Client { return testHTTPClient() }
	t.Cleanup(func() { newHTTPClient = orig })

	var (
		inFlight    int32
		maxInFlight int32
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)
		for {
			prev := atomic.LoadInt32(&maxInFlight)
			if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var body strings.Builder
	totalLinks := maxLinksPerSkill + 10
	for i := range totalLinks {
		fmt.Fprintf(&body, "[l%d](%s/x?i=%d)\n", i, server.URL, i)
	}

	results := CheckLinks(t.Context(), t.TempDir(), body.String())

	var passes int
	var sawTruncationWarning bool
	for _, r := range results {
		if r.Level == types.Pass {
			passes++
		}
		if r.Level == types.Warning && strings.Contains(r.Message, "truncated") {
			sawTruncationWarning = true
		}
	}
	if passes != maxLinksPerSkill {
		t.Errorf("expected %d passes, got %d (total results=%d)", maxLinksPerSkill, passes, len(results))
	}
	if !sawTruncationWarning {
		t.Errorf("expected truncation warning, got results=%+v", results)
	}
	if int(maxInFlight) > maxConcurrentLinkChecks {
		t.Errorf("observed %d concurrent requests, want <= %d", maxInFlight, maxConcurrentLinkChecks)
	}
}

func TestCheckHTTPLink(t *testing.T) {
	client := testHTTPClient()

	t.Run("connection refused", func(t *testing.T) {
		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, "http://127.0.0.1:1")
		if result.Level != types.Error {
			t.Errorf("expected Error level, got %d", result.Level)
		}
		requireContains(t, result.Message, "request failed")
	})

	t.Run("redirect 3xx", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "/dest")
			w.WriteHeader(http.StatusMovedPermanently)
		})
		mux.HandleFunc("/dest", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL+"/redirect")
		if result.Level != types.Pass {
			t.Errorf("expected Pass for followed redirect, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("redirect without follow results in 3xx", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://127.0.0.1:1/nowhere")
			w.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Error {
			t.Errorf("expected Error for broken redirect target, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("too many redirects", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", r.URL.Path)
			w.WriteHeader(http.StatusFound)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL+"/loop")
		if result.Level != types.Error {
			t.Errorf("expected Error for redirect loop, got level=%d message=%q", result.Level, result.Message)
		}
		requireContains(t, result.Message, "request failed")
	})

	t.Run("403 forbidden", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Info {
			t.Errorf("expected Info level for 403, got %d", result.Level)
		}
		requireContains(t, result.Message, "HTTP 403")
	})

	t.Run("HEAD 404 retries with GET", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Pass {
			t.Errorf("expected Pass after GET fallback, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("HEAD 405 retries with GET", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Pass {
			t.Errorf("expected Pass after GET fallback, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("SPA requiring Accept text/html resolves via GET fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") == "" || r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Pass {
			t.Errorf("expected Pass for SPA with Accept header, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("genuine 404 still errors after GET fallback", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, server.URL)
		if result.Level != types.Error {
			t.Errorf("expected Error for genuine 404, got level=%d message=%q", result.Level, result.Message)
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		result := checkHTTPLink(types.ResultContext{Category: "Links", File: "SKILL.md"}, client, "http://invalid host with spaces/")
		if result.Level != types.Error {
			t.Errorf("expected Error for invalid URL, got level=%d", result.Level)
		}
		requireContains(t, result.Message, "invalid URL")
	})
}
