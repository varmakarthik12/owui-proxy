package owuiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/varmakarthik12/owui-proxy/internal/config"
)

// Client is a typed HTTP client for the Open WebUI OpenAI-compatible API.
type Client struct {
	httpClient *http.Client
	cfg        *config.Config
}

// New creates a new Open WebUI client.
func New(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
			// Do NOT set timeout for streaming — we handle context cancellation.
			// The timeout here is a safety net for non-streaming calls.
		},
		cfg: cfg,
	}
}

// ----- OpenAI-compatible response types -----

// ModelList is the response from GET /api/models.
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model represents a single model entry from the OpenAI models endpoint.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// EmbeddingResponse is the response from POST /api/embeddings.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *EmbeddingUsage `json:"usage,omitempty"`
}

// EmbeddingData holds one embedding vector.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage contains token usage info.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ChatCompletionResponse is the non-streaming response from POST /api/chat/completions.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"`
}

// ChatCompletionChoice represents a single choice in the response.
type ChatCompletionChoice struct {
	Index        int                    `json:"index"`
	Message      ChatCompletionMessage  `json:"message"`
	FinishReason string                 `json:"finish_reason"`
}

// ChatCompletionMessage represents a message in the chat completion.
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionUsage contains token usage for chat completions.
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ----- API methods -----

// ListModels fetches all available models from Open WebUI.
func (c *Client) ListModels(ctx context.Context) (*ModelList, error) {
	url := c.cfg.UpstreamURL("/models")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating models request: %w", err)
	}
	c.setHeaders(req)

	slog.Debug("upstream request", "method", "GET", "url", url, "token", maskToken(c.cfg.Token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var models ModelList
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("decoding models response: %w", err)
	}

	return &models, nil
}

// ChatCompletion sends a non-streaming chat completion request.
func (c *Client) ChatCompletion(ctx context.Context, body io.Reader) (*ChatCompletionResponse, error) {
	url := c.cfg.UpstreamURL("/chat/completions")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating chat completion request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("upstream request", "method", "POST", "url", url, "token", maskToken(c.cfg.Token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat completion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding chat completion response: %w", err)
	}

	return &result, nil
}

// ChatCompletionStream sends a streaming chat completion request and returns
// the raw response for SSE processing. The caller is responsible for closing
// the response body.
func (c *Client) ChatCompletionStream(ctx context.Context, body io.Reader) (*http.Response, error) {
	url := c.cfg.UpstreamURL("/chat/completions")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating streaming chat request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	slog.Debug("upstream streaming request", "method", "POST", "url", url, "token", maskToken(c.cfg.Token))

	// Use a client without timeout for streaming — context handles cancellation.
	streamClient := &http.Client{
		Timeout: 0, // no timeout; context controls lifetime
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("streaming chat completion: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	return resp, nil
}

// CreateEmbedding sends an embedding request to Open WebUI.
func (c *Client) CreateEmbedding(ctx context.Context, body io.Reader) (*EmbeddingResponse, error) {
	url := c.cfg.UpstreamURL("/embeddings")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating embedding request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("upstream request", "method", "POST", "url", url, "token", maskToken(c.cfg.Token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creating embedding: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding embedding response: %w", err)
	}

	return &result, nil
}

// ----- helpers -----

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("User-Agent", "owui-proxy/1.0")
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	if strings.HasPrefix(token, "sk-") && len(token) > 12 {
		return token[:7] + "...****"
	}
	if len(token) > 20 {
		return token[:5] + "..." + token[len(token)-5:]
	}
	return token[:4] + "****"
}

// TimeNow returns the current time in RFC3339 format with nanoseconds.
// Exported for use by translators.
func TimeNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// MaskToken is a public alias for token masking.
func MaskToken(token string) string {
	return maskToken(token)
}

// StripSensitiveHeaders removes authentication and forwarding headers from
// inbound requests before processing.
func StripSensitiveHeaders(h http.Header) {
	h.Del("Authorization")
	h.Del("X-Forwarded-For")
	h.Del("X-Real-IP")
}

// ErrorResponse represents an error in the format clients expect.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// ReadJSON decodes a JSON request body into v and returns an error if it fails.
func ReadJSON(r io.Reader, v interface{}) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		if err == io.EOF {
			return fmt.Errorf("empty request body")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// FormatUpstreamError formats an error message from the upstream, ensuring
// the token is never exposed.
func FormatUpstreamError(err error, token string) string {
	msg := err.Error()
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "****")
	}
	return msg
}
