package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// defaultHTTPClient is used for all LLM API calls. It sets a timeout so
// that a hanging upstream doesn't block the caller indefinitely.
var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// lookPath is used to locate the claude binary. It is a variable so tests
// can substitute a stub when the real binary is not installed.
var lookPath = exec.LookPath

// LLMClient is the interface for making LLM API calls.
type LLMClient interface {
	// Complete sends a system prompt and user content to the LLM and returns the text response.
	Complete(ctx context.Context, systemPrompt, userContent string) (string, error)
	// Provider returns the provider name (e.g. "anthropic", "openai").
	Provider() string
	// Model returns the model identifier.
	ModelName() string
}

// ClientOptions holds configuration for creating an LLM client.
type ClientOptions struct {
	Provider          string // "anthropic", "openai", or "claude-cli"
	APIKey            string // Required for anthropic and openai; unused for claude-cli
	BaseURL           string // Optional; defaults per provider
	Model             string // Optional; defaults per provider
	MaxTokensStyle    string // "auto", "max_tokens", or "max_completion_tokens"
	MaxResponseTokens int    // Maximum tokens in the LLM response; 0 defaults to 500
	OrgID             string // Optional OpenAI organization ID; sent as OpenAI-Organization header
	ProjectID         string // Optional OpenAI project ID; sent as OpenAI-Project header
}

// NewClient creates an LLMClient for the given options.
// If Model is empty, a default is chosen per provider.
// For the openai provider, BaseURL defaults to "https://api.openai.com/v1" if empty.
// The claude-cli provider shells out to the "claude" CLI and does not require an API key.
func NewClient(opts ClientOptions) (LLMClient, error) {
	if strings.ToLower(opts.Provider) != "claude-cli" && opts.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	maxResp := opts.MaxResponseTokens
	if maxResp <= 0 {
		maxResp = 500
	}

	switch strings.ToLower(opts.Provider) {
	case "claude-cli":
		if _, err := lookPath("claude"); err != nil {
			return nil, fmt.Errorf("claude-cli provider requires the \"claude\" binary: %w", err)
		}
		model := opts.Model
		if model == "" {
			model = "sonnet"
		}
		return &claudeCLIClient{model: model}, nil
	case "anthropic":
		model := opts.Model
		if model == "" {
			model = "claude-sonnet-4-5-20250929"
		}
		baseURL := "https://api.anthropic.com"
		if opts.BaseURL != "" {
			baseURL = strings.TrimRight(opts.BaseURL, "/")
		}
		return &anthropicClient{apiKey: opts.APIKey, model: model, baseURL: baseURL, maxTokens: maxResp}, nil
	case "openai":
		model := opts.Model
		if model == "" {
			model = "gpt-5.2"
		}
		baseURL := opts.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		baseURL = strings.TrimRight(baseURL, "/")
		return &openaiClient{apiKey: opts.APIKey, baseURL: baseURL, model: model, maxTokensStyle: opts.MaxTokensStyle, maxTokens: maxResp, orgID: opts.OrgID, projectID: opts.ProjectID}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q (use \"anthropic\", \"openai\", or \"claude-cli\")", opts.Provider)
	}
}

// --- Anthropic client ---

type anthropicClient struct {
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
}

func (c *anthropicClient) Provider() string  { return "anthropic" }
func (c *anthropicClient) ModelName() string { return c.model }

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *anthropicClient) Complete(ctx context.Context, systemPrompt, userContent string) (string, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userContent},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return result.Content[0].Text, nil
}

// --- OpenAI-compatible client ---

type openaiClient struct {
	apiKey         string
	baseURL        string
	model          string
	maxTokensStyle string
	maxTokens      int
	orgID          string
	projectID      string
}

func (c *openaiClient) Provider() string  { return "openai" }
func (c *openaiClient) ModelName() string { return c.model }

// isOpenAIHost reports whether the given base URL points to an OpenAI endpoint
// (api.openai.com, us.api.openai.com, etc.) as opposed to a third-party
// compatible API like Ollama or vLLM.
func isOpenAIHost(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return strings.HasSuffix(host, ".openai.com")
}

type openaiRequest struct {
	Model               string          `json:"model"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Messages            []openaiMessage `json:"messages"`
}

// useMaxCompletionTokens reports whether the given model requires
// "max_completion_tokens" instead of the older "max_tokens" parameter.
// OpenAI's o-series reasoning models and gpt-5+ models require this.
func useMaxCompletionTokens(model string) bool {
	m := strings.ToLower(model)
	// o1, o3, o4-mini, etc.
	if strings.HasPrefix(m, "o") {
		return true
	}
	// gpt-5 and later
	if strings.HasPrefix(m, "gpt-5") {
		return true
	}
	return false
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openaiClient) Complete(ctx context.Context, systemPrompt, userContent string) (string, error) {
	messages := []openaiMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userContent},
	}

	reqBody := openaiRequest{
		Model:    c.model,
		Messages: messages,
	}
	switch c.maxTokensStyle {
	case "max_completion_tokens":
		reqBody.MaxCompletionTokens = c.maxTokens
	case "max_tokens":
		reqBody.MaxTokens = c.maxTokens
	default: // "auto" or empty
		if useMaxCompletionTokens(c.model) {
			reqBody.MaxCompletionTokens = c.maxTokens
		} else {
			reqBody.MaxTokens = c.maxTokens
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Only send OpenAI-specific org/project headers when targeting an
	// OpenAI endpoint, so they don't leak to Ollama or other compatible APIs.
	if isOpenAIHost(c.baseURL) {
		if c.orgID != "" {
			req.Header.Set("OpenAI-Organization", c.orgID)
		}
		if c.projectID != "" {
			req.Header.Set("OpenAI-Project", c.projectID)
		}
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Code == "incorrect_hostname" {
			return "", fmt.Errorf("your OpenAI organization requires a regional endpoint: %s\nSet OPENAI_BASE_URL or use --base-url to specify the correct host (e.g. https://us.api.openai.com/v1)",
				errResp.Error.Message)
		}
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result openaiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// --- Claude CLI client ---

// claudeCLIClient invokes the "claude" CLI for completions.
// This is useful when the CLI is already authenticated (e.g. via a company
// subscription) and no explicit API key is needed.
type claudeCLIClient struct {
	model string
}

func (c *claudeCLIClient) Provider() string  { return "claude-cli" }
func (c *claudeCLIClient) ModelName() string { return c.model }

// buildArgs returns the CLI arguments for a claude invocation.
func (c *claudeCLIClient) buildArgs(systemPrompt, userContent string) []string {
	args := []string{
		"-p",
		"--output-format", "text",
		"--model", c.model,
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}
	args = append(args, "--", userContent)
	return args
}

func (c *claudeCLIClient) Complete(ctx context.Context, systemPrompt, userContent string) (string, error) {
	args := c.buildArgs(systemPrompt, userContent)

	cmd := exec.CommandContext(ctx, "claude", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("claude CLI failed: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
