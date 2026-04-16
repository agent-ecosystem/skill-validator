package links

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/agent-ecosystem/skill-validator/types"
)

// Options configures external link validation.
type Options struct {
	// IgnorePatterns is a list of substrings. Any URL that contains one of
	// these strings (case-insensitive) is skipped without being checked.
	// Useful for private repositories or localhost URLs that are unreachable
	// from CI but valid in a local development context.
	// Example: --ignore-link=github.com/myorg --ignore-link=localhost
	IgnorePatterns []string
}

type linkResult struct {
	url    string
	result types.Result
}

// CheckLinks validates external (HTTP/HTTPS) links in the skill body.
func CheckLinks(ctx context.Context, dir, body string, opts Options) []types.Result {
	rctx := types.ResultContext{Category: "Links", File: "SKILL.md"}
	allLinks := ExtractLinks(body)
	if len(allLinks) == 0 {
		return nil
	}

	var (
		results   []types.Result
		httpLinks []string
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	// Collect HTTP links only
	for _, link := range allLinks {
		// Skip template URLs containing {placeholder} variables (RFC 6570 URI Templates)
		if strings.Contains(link, "{") {
			continue
		}
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
			continue
		}
		if isIgnored(link, opts.IgnorePatterns) {
			continue
		}
		httpLinks = append(httpLinks, link)
	}

	if len(httpLinks) == 0 {
		return nil
	}

	// Shared client for connection reuse across concurrent checks.
	// The client uses a safe transport that blocks requests to private IPs.
	client := newHTTPClient()

	// Check HTTP links concurrently
	httpResults := make([]linkResult, len(httpLinks))
	for i, link := range httpLinks {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			r := checkHTTPLink(rctx, client, url)
			mu.Lock()
			httpResults[idx] = linkResult{url: url, result: r}
			mu.Unlock()
		}(i, link)
	}
	wg.Wait()

	for _, hr := range httpResults {
		results = append(results, hr.result)
	}

	return results
}

func checkHTTPLink(rctx types.ResultContext, client *http.Client, url string) types.Result {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return rctx.Errorf("%s (invalid URL: %v)", url, err)
	}
	req.Header.Set("User-Agent", "skill-validator/1.0")
	req.Header.Set("Accept", "text/html, */*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		return rctx.Errorf("%s (request failed: %v)", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Some sites don't handle HEAD correctly (e.g. SPAs like crates.io return
	// 404 for HEAD even though the page exists). Fall back to GET when HEAD
	// returns 404 or 405, which is the standard approach used by lychee,
	// markdown-link-check, and other link validators.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return checkHTTPLinkGET(rctx, client, url)
	}

	return classifyResponse(rctx, url, resp.StatusCode)
}

func checkHTTPLinkGET(rctx types.ResultContext, client *http.Client, url string) types.Result {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return rctx.Errorf("%s (invalid URL: %v)", url, err)
	}
	req.Header.Set("User-Agent", "skill-validator/1.0")
	req.Header.Set("Accept", "text/html, */*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		return rctx.Errorf("%s (request failed: %v)", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return classifyResponse(rctx, url, resp.StatusCode)
}

// isIgnored reports whether url contains any of the given patterns
// (case-insensitive substring match).
func isIgnored(url string, patterns []string) bool {
	lower := strings.ToLower(url)
	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func classifyResponse(rctx types.ResultContext, url string, statusCode int) types.Result {
	if statusCode >= 200 && statusCode < 300 {
		return rctx.Passf("%s (HTTP %d)", url, statusCode)
	}
	if statusCode >= 300 && statusCode < 400 {
		return rctx.Passf("%s (HTTP %d redirect)", url, statusCode)
	}
	if statusCode == http.StatusForbidden {
		return rctx.Infof("%s (HTTP 403 — may block automated requests)", url)
	}
	return rctx.Errorf("%s (HTTP %d)", url, statusCode)
}
