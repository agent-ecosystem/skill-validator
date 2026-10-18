package judge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// stubLookPath replaces the lookPath variable for the duration of a test,
// restoring the original when the test completes.
func stubLookPath(t *testing.T, found bool) {
	t.Helper()
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	if found {
		lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	} else {
		lookPath = func(file string) (string, error) { return "", fmt.Errorf("not found: %s", file) }
	}
}

func TestClaudeCLIClientDefaults(t *testing.T) {
	stubLookPath(t, true)
	client, err := NewClient(ClientOptions{Provider: "claude-cli"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Provider() != "claude-cli" {
		t.Errorf("Provider() = %q, want %q", client.Provider(), "claude-cli")
	}
	if client.ModelName() != "sonnet" {
		t.Errorf("ModelName() = %q, want %q", client.ModelName(), "sonnet")
	}
}

func TestClaudeCLIClientCustomModel(t *testing.T) {
	stubLookPath(t, true)
	client, err := NewClient(ClientOptions{Provider: "claude-cli", Model: "opus"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.ModelName() != "opus" {
		t.Errorf("ModelName() = %q, want %q", client.ModelName(), "opus")
	}
}

func TestClaudeCLINoAPIKeyRequired(t *testing.T) {
	stubLookPath(t, true)

	// claude-cli should not require an API key
	_, err := NewClient(ClientOptions{Provider: "claude-cli"})
	if err != nil {
		t.Fatalf("expected no error without API key for claude-cli, got: %v", err)
	}

	// Other providers still require it
	_, err = NewClient(ClientOptions{Provider: "anthropic"})
	if err == nil {
		t.Fatal("expected error without API key for anthropic")
	}
}

func TestClaudeCLIMissingBinary(t *testing.T) {
	stubLookPath(t, false)

	_, err := NewClient(ClientOptions{Provider: "claude-cli"})
	if err == nil {
		t.Fatal("expected error when claude binary is not found")
	}
	if got := err.Error(); !strings.Contains(got, "claude-cli provider requires") {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestUseMaxCompletionTokens(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		// Legacy models use max_tokens
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4-turbo", false},
		{"gpt-3.5-turbo", false},
		// GPT-5+ models use max_completion_tokens
		{"gpt-5", true},
		{"gpt-5.2", true},
		{"gpt-5-turbo", true},
		// O-series reasoning models use max_completion_tokens
		{"o1", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		// Case insensitivity
		{"GPT-5.2", true},
		{"O3-mini", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := useMaxCompletionTokens(tt.model)
			if got != tt.want {
				t.Errorf("useMaxCompletionTokens(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsOpenAIHost(t *testing.T) {
	tests := []struct {
		baseURL string
		want    bool
	}{
		{"https://api.openai.com/v1", true},
		{"https://us.api.openai.com/v1", true},
		{"https://eu.api.openai.com/v1", true},
		{"http://localhost:11434/v1", false},
		{"https://my-proxy.example.com/v1", false},
		{"https://notopenai.com/v1", false},
		{"not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			got := isOpenAIHost(tt.baseURL)
			if got != tt.want {
				t.Errorf("isOpenAIHost(%q) = %v, want %v", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestOpenAIClient_OrgProjectHeaders(t *testing.T) {
	t.Run("headers sent for openai.com", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("OpenAI-Organization"); got != "org-123" {
				t.Errorf("OpenAI-Organization = %q, want %q", got, "org-123")
			}
			if got := r.Header.Get("OpenAI-Project"); got != "proj-456" {
				t.Errorf("OpenAI-Project = %q, want %q", got, "proj-456")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"choices": [{"message": {"content": "ok"}}]}`)
		}))
		defer server.Close()

		// The test server isn't on openai.com, so we construct the client directly
		// to test the header logic with an openai.com baseURL that actually points
		// at the test server. Instead, we test the client with the test server URL
		// and verify via a different approach: construct the openaiClient directly.
		c := &openaiClient{
			apiKey:    "test-key",
			baseURL:   "https://api.openai.com/v1",
			model:     "gpt-4o",
			maxTokens: 500,
			orgID:     "org-123",
			projectID: "proj-456",
		}

		// Override defaultHTTPClient to proxy to test server
		origClient := defaultHTTPClient
		defer func() { defaultHTTPClient = origClient }()

		// Use a custom transport that rewrites the host to the test server
		testURL := server.URL
		defaultHTTPClient = &http.Client{
			Timeout:   5 * time.Second,
			Transport: &rewriteTransport{target: testURL},
		}

		_, err := c.Complete(t.Context(), "system", "user")
		if err != nil {
			t.Fatalf("Complete failed: %v", err)
		}
	})

	t.Run("headers not sent for custom base URL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("OpenAI-Organization"); got != "" {
				t.Errorf("OpenAI-Organization should be empty for non-OpenAI host, got %q", got)
			}
			if got := r.Header.Get("OpenAI-Project"); got != "" {
				t.Errorf("OpenAI-Project should be empty for non-OpenAI host, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"choices": [{"message": {"content": "ok"}}]}`)
		}))
		defer server.Close()

		client, err := NewClient(ClientOptions{
			Provider:  "openai",
			APIKey:    "test-key",
			BaseURL:   server.URL,
			Model:     "llama3",
			OrgID:     "org-123",
			ProjectID: "proj-456",
		})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		_, err = client.Complete(t.Context(), "system", "user")
		if err != nil {
			t.Fatalf("Complete failed: %v", err)
		}
	})

	t.Run("empty org and project not sent", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("OpenAI-Organization"); got != "" {
				t.Errorf("OpenAI-Organization should be empty, got %q", got)
			}
			if got := r.Header.Get("OpenAI-Project"); got != "" {
				t.Errorf("OpenAI-Project should be empty, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"choices": [{"message": {"content": "ok"}}]}`)
		}))
		defer server.Close()

		client, err := NewClient(ClientOptions{
			Provider: "openai",
			APIKey:   "test-key",
			BaseURL:  server.URL,
			Model:    "gpt-4o",
		})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		_, err = client.Complete(t.Context(), "system", "user")
		if err != nil {
			t.Fatalf("Complete failed: %v", err)
		}
	})
}

func TestOpenAIClient_RegionalHostnameError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"Attempted to access resource with incorrect regional hostname. Please make your request to us.api.openai.com","type":"invalid_request_error","code":"incorrect_hostname","param":null},"status":401}`)
	}))
	defer server.Close()

	client, err := NewClient(ClientOptions{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  server.URL,
		Model:    "gpt-4o",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Complete(t.Context(), "system", "user")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "regional endpoint") {
		t.Errorf("expected error to mention regional endpoint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "OPENAI_BASE_URL") {
		t.Errorf("expected error to mention OPENAI_BASE_URL, got: %v", err)
	}
}

// rewriteTransport rewrites requests to a different target URL while
// preserving the original Host header for testing.
type rewriteTransport struct {
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	targetURL, _ := url.Parse(t.target)
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	return http.DefaultTransport.RoundTrip(req)
}

func TestMaxTokensStyleOverride(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		maxTokensStyle string
		wantMaxTokens  bool // true = max_tokens field set; false = max_completion_tokens field set
	}{
		{
			name:           "auto with legacy model uses max_tokens",
			model:          "gpt-4o",
			maxTokensStyle: "auto",
			wantMaxTokens:  true,
		},
		{
			name:           "auto with o-series uses max_completion_tokens",
			model:          "o3-mini",
			maxTokensStyle: "auto",
			wantMaxTokens:  false,
		},
		{
			name:           "explicit max_tokens overrides o-series detection",
			model:          "o3-mini",
			maxTokensStyle: "max_tokens",
			wantMaxTokens:  true,
		},
		{
			name:           "explicit max_completion_tokens overrides legacy detection",
			model:          "gpt-4o",
			maxTokensStyle: "max_completion_tokens",
			wantMaxTokens:  false,
		},
		{
			name:           "empty string defaults to auto behavior",
			model:          "gpt-4o",
			maxTokensStyle: "",
			wantMaxTokens:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody openaiRequest

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
					t.Fatalf("decoding request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"score\": 4}"}}]}`)
			}))
			defer srv.Close()

			client, err := NewClient(ClientOptions{Provider: "openai", APIKey: "test-key", BaseURL: srv.URL, Model: tt.model, MaxTokensStyle: tt.maxTokensStyle})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			_, err = client.Complete(t.Context(), "system", "user")
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}

			if tt.wantMaxTokens {
				if capturedBody.MaxTokens == 0 {
					t.Error("expected max_tokens to be set, but it was 0")
				}
				if capturedBody.MaxCompletionTokens != 0 {
					t.Errorf("expected max_completion_tokens to be 0, got %d", capturedBody.MaxCompletionTokens)
				}
			} else {
				if capturedBody.MaxCompletionTokens == 0 {
					t.Error("expected max_completion_tokens to be set, but it was 0")
				}
				if capturedBody.MaxTokens != 0 {
					t.Errorf("expected max_tokens to be 0, got %d", capturedBody.MaxTokens)
				}
			}
		})
	}
}

func TestClaudeCLIBuildArgs(t *testing.T) {
	c := &claudeCLIClient{model: "sonnet"}

	t.Run("with system prompt file", func(t *testing.T) {
		args := c.buildArgs("/tmp/prompt.md")
		want := []string{"-p", "--output-format", "text", "--model", "sonnet", "--system-prompt-file", "/tmp/prompt.md"}
		if !reflect.DeepEqual(args, want) {
			t.Errorf("args = %v, want %v", args, want)
		}
	})

	t.Run("without system prompt", func(t *testing.T) {
		for _, a := range c.buildArgs("") {
			if strings.HasPrefix(a, "--system-prompt") {
				t.Errorf("no system prompt flag expected, got %v", c.buildArgs(""))
			}
		}
	})
}

// stubRunClaude replaces runClaude for the duration of a test.
func stubRunClaude(t *testing.T, fn func(ctx context.Context, args []string, stdin io.Reader) (string, string, error)) {
	t.Helper()
	orig := runClaude
	t.Cleanup(func() { runClaude = orig })
	runClaude = fn
}

// Issue #103: the judge prompt is about 24 KB, so passing it and the content
// as arguments exceeded the Windows command-line limit. The content travels
// on stdin and the system prompt in a file that exists only for the call.
func TestClaudeCLIComplete_StdinAndPromptFile(t *testing.T) {
	stubLookPath(t, true)
	var gotArgs []string
	var gotStdin, gotPromptFile, gotPromptContent string
	stubRunClaude(t, func(_ context.Context, args []string, stdin io.Reader) (string, string, error) {
		gotArgs = args
		b, err := io.ReadAll(stdin)
		if err != nil {
			t.Errorf("reading stdin: %v", err)
		}
		gotStdin = string(b)
		for i, a := range args {
			if a == "--system-prompt-file" && i+1 < len(args) {
				gotPromptFile = args[i+1]
				data, err := os.ReadFile(gotPromptFile)
				if err != nil {
					t.Errorf("reading prompt file during the call: %v", err)
				}
				gotPromptContent = string(data)
			}
		}
		return "  {\"score\": 4}\n", "", nil
	})

	client, err := NewClient(ClientOptions{Provider: "claude-cli"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Larger than any Windows command line, and flag-like, since nothing
	// on stdin can be mistaken for an option.
	big := strings.Repeat("x", 40000)
	system, user := "SYSTEM "+big, "--not-a-flag "+big
	out, err := client.Complete(t.Context(), system, user)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if out != `{"score": 4}` {
		t.Errorf("output = %q, want trimmed stdout", out)
	}
	if gotStdin != user {
		t.Error("user content did not arrive on stdin intact")
	}
	if gotPromptContent != system {
		t.Error("system prompt file did not hold the system prompt")
	}
	for _, a := range gotArgs {
		if strings.Contains(a, "SYSTEM") || strings.Contains(a, "not-a-flag") {
			t.Errorf("prompt text passed as an argument: %.40s", a)
		}
	}
	if gotPromptFile == "" {
		t.Fatal("no --system-prompt-file argument")
	}
	if _, err := os.Stat(gotPromptFile); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("prompt file %s left behind (stat error %v)", gotPromptFile, err)
	}
}

func TestClaudeCLIComplete_NoSystemPrompt(t *testing.T) {
	stubLookPath(t, true)
	var gotArgs []string
	stubRunClaude(t, func(_ context.Context, args []string, _ io.Reader) (string, string, error) {
		gotArgs = args
		return "ok", "", nil
	})
	client, _ := NewClient(ClientOptions{Provider: "claude-cli"})
	if _, err := client.Complete(t.Context(), "", "content"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	for _, a := range gotArgs {
		if a == "--system-prompt-file" {
			t.Errorf("unexpected system prompt file in %v", gotArgs)
		}
	}
}

func TestClaudeCLIComplete_Failure(t *testing.T) {
	stubLookPath(t, true)
	stubRunClaude(t, func(context.Context, []string, io.Reader) (string, string, error) {
		return "", "boom from stderr", fmt.Errorf("exit status 1")
	})
	client, _ := NewClient(ClientOptions{Provider: "claude-cli"})
	_, err := client.Complete(t.Context(), "sys", "user")
	if err == nil || !strings.Contains(err.Error(), "boom from stderr") {
		t.Errorf("error = %v, want the CLI's stderr included", err)
	}
}

// Issue #103: a reasoning model that spends the whole response budget
// thinking returns empty content. The error must say so rather than
// report an unparseable empty reply.
func TestOpenAIClient_EmptyContent(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantBudget bool
	}{
		{
			name:       "reasoning exhausted the budget",
			body:       `{"choices":[{"message":{"content":"","reasoning_content":"let me think"},"finish_reason":"length"}]}`,
			wantBudget: true,
		},
		{
			name:       "reasoning field variant",
			body:       `{"choices":[{"message":{"content":"","reasoning":"let me think"},"finish_reason":"stop"}]}`,
			wantBudget: true,
		},
		{
			name:       "cut off without visible reasoning",
			body:       `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`,
			wantBudget: true,
		},
		{
			name:       "empty for another reason",
			body:       `{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`,
			wantBudget: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			client, err := NewClient(ClientOptions{Provider: "openai", APIKey: "k", BaseURL: server.URL, Model: "glm-4.7", MaxResponseTokens: 500})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			_, err = client.Complete(t.Context(), "sys", "user")
			if !errors.Is(err, ErrEmptyResponse) {
				t.Fatalf("error = %v, want ErrEmptyResponse", err)
			}
			mentionsBudget := strings.Contains(err.Error(), "500-token response budget") && strings.Contains(err.Error(), "--max-response-tokens")
			if mentionsBudget != tc.wantBudget {
				t.Errorf("error = %v, want budget mention %v", err, tc.wantBudget)
			}
		})
	}
}

func TestAnthropicClient_ThinkingBlocks(t *testing.T) {
	serve := func(t *testing.T, body string) LLMClient {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, body)
		}))
		t.Cleanup(server.Close)
		client, err := NewClient(ClientOptions{Provider: "anthropic", APIKey: "k", BaseURL: server.URL, Model: "m", MaxResponseTokens: 500})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		return client
	}

	t.Run("text after thinking is returned", func(t *testing.T) {
		client := serve(t, `{"content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`)
		got, err := client.Complete(t.Context(), "sys", "user")
		if err != nil || got != "ok" {
			t.Errorf("got %q, %v; want \"ok\"", got, err)
		}
	})

	t.Run("thinking only at max_tokens names the budget", func(t *testing.T) {
		client := serve(t, `{"content":[{"type":"thinking","thinking":"hmm"}],"stop_reason":"max_tokens"}`)
		_, err := client.Complete(t.Context(), "sys", "user")
		if !errors.Is(err, ErrEmptyResponse) || !strings.Contains(err.Error(), "500-token response budget") {
			t.Errorf("error = %v, want ErrEmptyResponse naming the budget", err)
		}
	})
}

func TestMaxResponseTokens(t *testing.T) {
	t.Run("default reaches the openai request", func(t *testing.T) {
		var got openaiRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
		}))
		defer server.Close()
		client, _ := NewClient(ClientOptions{Provider: "openai", APIKey: "k", BaseURL: server.URL})
		if _, err := client.Complete(t.Context(), "sys", "user"); err != nil {
			t.Fatal(err)
		}
		if got.MaxCompletionTokens != DefaultMaxResponseTokens {
			t.Errorf("max_completion_tokens = %d, want %d", got.MaxCompletionTokens, DefaultMaxResponseTokens)
		}
	})

	t.Run("explicit value reaches the anthropic request", func(t *testing.T) {
		var got anthropicRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"content":[{"type":"text","text":"ok"}]}`)
		}))
		defer server.Close()
		client, _ := NewClient(ClientOptions{Provider: "anthropic", APIKey: "k", BaseURL: server.URL, MaxResponseTokens: 1234})
		if _, err := client.Complete(t.Context(), "sys", "user"); err != nil {
			t.Fatal(err)
		}
		if got.MaxTokens != 1234 {
			t.Errorf("max_tokens = %d, want 1234", got.MaxTokens)
		}
	})
}
