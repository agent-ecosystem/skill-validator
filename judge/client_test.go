package judge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	AllowInsecureBaseURL = true
	os.Exit(m.Run())
}

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

func TestClaudeCLIBuildArgs(t *testing.T) {
	c := &claudeCLIClient{model: "sonnet"}

	t.Run("with system prompt", func(t *testing.T) {
		args := c.buildArgs("you are a judge", "score this")
		want := []string{"-p", "--output-format", "text", "--model", "sonnet", "--system-prompt", "you are a judge", "--", "score this"}
		if len(args) != len(want) {
			t.Fatalf("got %d args, want %d: %v", len(args), len(want), args)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
			}
		}
	})

	t.Run("without system prompt", func(t *testing.T) {
		args := c.buildArgs("", "score this")
		for _, a := range args {
			if a == "--system-prompt" {
				t.Error("--system-prompt should not be present when system prompt is empty")
			}
		}
		if args[len(args)-1] != "score this" {
			t.Errorf("last arg = %q, want %q", args[len(args)-1], "score this")
		}
		if args[len(args)-2] != "--" {
			t.Errorf("expected '--' before user content, got %q", args[len(args)-2])
		}
	})

	t.Run("flag-like user content cannot be parsed as option", func(t *testing.T) {
		args := c.buildArgs("", "--dangerous-flag=value")
		var sawSeparator bool
		for i, a := range args {
			if a == "--" {
				sawSeparator = true
				if i != len(args)-2 {
					t.Errorf("'--' should precede the user content; got args=%v", args)
				}
			}
		}
		if !sawSeparator {
			t.Errorf("expected '--' separator in args=%v", args)
		}
	})
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

func TestNewClient_RefusesInsecureBaseURLByDefault(t *testing.T) {
	prev := AllowInsecureBaseURL
	AllowInsecureBaseURL = false
	t.Cleanup(func() { AllowInsecureBaseURL = prev })

	cases := []struct {
		name     string
		provider string
		baseURL  string
	}{
		{"openai http", "openai", "http://attacker.example/v1"},
		{"anthropic http", "anthropic", "http://attacker.example"},
		{"openai garbage scheme", "openai", "gopher://attacker.example"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(ClientOptions{Provider: tc.provider, APIKey: "k", BaseURL: tc.baseURL})
			if err == nil {
				t.Fatalf("expected error for insecure base URL %q", tc.baseURL)
			}
			if !strings.Contains(err.Error(), "refusing") && !strings.Contains(err.Error(), "invalid base URL") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	t.Run("https accepted", func(t *testing.T) {
		_, err := NewClient(ClientOptions{Provider: "openai", APIKey: "k", BaseURL: "https://api.openai.com/v1"})
		if err != nil {
			t.Errorf("https base URL rejected: %v", err)
		}
	})

	t.Run("http accepted when AllowInsecureBaseURL set", func(t *testing.T) {
		AllowInsecureBaseURL = true
		defer func() { AllowInsecureBaseURL = false }()
		_, err := NewClient(ClientOptions{Provider: "openai", APIKey: "k", BaseURL: "http://localhost:11434/v1"})
		if err != nil {
			t.Errorf("http base URL rejected after opt-in: %v", err)
		}
	})
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
				resp := openaiResponse{
					Choices: []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					}{
						{Message: struct {
							Content string `json:"content"`
						}{Content: `{"score": 4}`}},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					t.Errorf("encoding response: %v", err)
				}
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
