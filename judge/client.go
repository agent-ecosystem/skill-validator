package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DefaultMaxResponseTokens is the response budget used when
// ClientOptions.MaxResponseTokens is zero. The judge's JSON answer is a few
// hundred tokens, but reasoning models (OpenAI o-series and gpt-5, GLM,
// Kimi, DeepSeek) think inside the same budget before they answer, and a
// budget sized for the answer alone leaves them no room to produce it.
const DefaultMaxResponseTokens = 8192

// ErrEmptyResponse reports a successful API call that carried no answer
// text. The wrapped message says why when the response makes it clear.
var ErrEmptyResponse = errors.New("model returned no text")

// defaultHTTPClient is used for all LLM API calls. It sets a timeout so
// that a hanging upstream doesn't block the caller indefinitely. Reasoning
// models can spend minutes on a large response budget.
var defaultHTTPClient = &http.Client{Timeout: 3 * time.Minute}

// lookPath is used to locate the claude binary. It is a variable so tests
// can substitute a stub when the real binary is not installed.
var lookPath = exec.LookPath

// runClaude executes the claude binary. It is a variable so tests can
// capture the invocation instead of running the real CLI.
var runClaude = func(ctx context.Context, args []string, stdin io.Reader) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "claude", args...)
	var out, errBuf bytes.Buffer
	cmd.Stdin = stdin
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return out.String(), errBuf.String(), err
}

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
	MaxResponseTokens int    // Response budget in tokens, reasoning included; 0 means DefaultMaxResponseTokens
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
		maxResp = DefaultMaxResponseTokens
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

// budgetExhausted builds the error for a response whose budget was used up
// before any answer text appeared, which is what a reasoning model does
// when the budget is too small for its thinking.
func budgetExhausted(maxTokens int, detail string) error {
	return fmt.Errorf("%w: the %d-token response budget was used up before an answer was produced (%s); raise it with --max-response-tokens or ClientOptions.MaxResponseTokens",
		ErrEmptyResponse, maxTokens, detail)
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
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
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

	// A gateway with extended thinking on returns thinking blocks before
	// the text block, and only thinking blocks, or an empty text block,
	// when the budget ran out. The answer is the first text block with
	// something in it.
	for _, block := range result.Content {
		if (block.Type == "text" || block.Type == "") && block.Text != "" {
			return block.Text, nil
		}
	}
	if result.StopReason == "max_tokens" {
		return "", budgetExhausted(c.maxTokens, "stop_reason max_tokens, no answer text")
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("%w: empty content in response", ErrEmptyResponse)
	}
	return "", fmt.Errorf("%w: no text in response", ErrEmptyResponse)
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
			// Reasoning models that expose their thinking put it here
			// (reasoning_content on GLM, Kimi, and DeepSeek; reasoning on
			// some gateways). It is not the answer.
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		CompletionTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
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

	choice := result.Choices[0]
	if choice.Message.Content != "" {
		return choice.Message.Content, nil
	}
	// OpenAI's own reasoning models keep their thinking private and report
	// only its size in usage; others return the text itself.
	reasoningTokens := result.Usage.CompletionTokensDetails.ReasoningTokens
	reasoned := reasoningTokens > 0 || choice.Message.ReasoningContent != "" || choice.Message.Reasoning != ""
	if reasoned || choice.FinishReason == "length" {
		detail := fmt.Sprintf("finish_reason %q", choice.FinishReason)
		switch {
		case reasoningTokens > 0:
			detail += fmt.Sprintf(", %d tokens of reasoning, no answer text", reasoningTokens)
		case reasoned:
			detail += ", reasoning only"
		default:
			detail += ", no answer text"
		}
		return "", budgetExhausted(c.maxTokens, detail)
	}
	return "", fmt.Errorf("%w: empty content in response", ErrEmptyResponse)
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

// buildArgs returns the CLI arguments for a claude invocation. The user
// content is not among them: it goes to the CLI on stdin, and the system
// prompt through a file, because the judge prompt alone is about 24 KB and
// Windows limits a command line to 32 KB (8 KB when launched through a
// .cmd shim), which the old prompt-as-argument form exceeded (#103).
func (c *claudeCLIClient) buildArgs(systemPromptFile string) []string {
	args := []string{
		"-p",
		"--output-format", "text",
		"--model", c.model,
	}
	if systemPromptFile != "" {
		args = append(args, "--system-prompt-file", systemPromptFile)
	}
	return args
}

func (c *claudeCLIClient) Complete(ctx context.Context, systemPrompt, userContent string) (string, error) {
	var promptFile string
	if systemPrompt != "" {
		f, err := os.CreateTemp("", "skill-validator-system-prompt-*.md")
		if err != nil {
			return "", fmt.Errorf("writing system prompt file: %w", err)
		}
		promptFile = f.Name()
		defer func() { _ = os.Remove(promptFile) }()
		if _, err := f.WriteString(systemPrompt); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("writing system prompt file: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("writing system prompt file: %w", err)
		}
	}

	stdout, stderr, err := runClaude(ctx, c.buildArgs(promptFile), strings.NewReader(userContent))
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w: %s", err, stderr)
	}
	out := strings.TrimSpace(stdout)
	if out == "" {
		// The CLI manages its own token budget, so there is no budget to
		// point at; say what happened instead of failing later in the
		// JSON parser with nothing to show.
		return "", fmt.Errorf("%w: the claude CLI printed nothing", ErrEmptyResponse)
	}
	return out, nil
}
