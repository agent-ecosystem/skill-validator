package links

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/agent-ecosystem/skill-validator/types"
)

const (
	maxConcurrentLinkChecks = 16
	maxLinksPerSkill        = 500
)

type linkResult struct {
	url    string
	result types.Result
}

// CheckLinks validates external (HTTP/HTTPS) links in the skill body.
func CheckLinks(ctx context.Context, dir, body string) []types.Result {
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

	for _, link := range allLinks {
		if strings.Contains(link, "{") {
			continue
		}
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			httpLinks = append(httpLinks, link)
		}
	}

	if len(httpLinks) == 0 {
		return nil
	}

	truncated := false
	if len(httpLinks) > maxLinksPerSkill {
		httpLinks = httpLinks[:maxLinksPerSkill]
		truncated = true
	}

	client := newHTTPClient()
	sem := make(chan struct{}, maxConcurrentLinkChecks)

	httpResults := make([]linkResult, len(httpLinks))
	for i, link := range httpLinks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, url string) {
			defer wg.Done()
			defer func() { <-sem }()
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

	if truncated {
		results = append(results, rctx.Warnf(
			"link checking truncated at %d URLs to bound resource use; remaining links not validated",
			maxLinksPerSkill,
		))
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
