package owuiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/varmakarthik12/owui-proxy/internal/config"
)

// ----- Rich model types for Open WebUI /api/models -----

// OWUIModelList is the full response from GET /api/models.
type OWUIModelList struct {
	Object string      `json:"object"`
	Data   []OWUIModel `json:"data"`
}

// OWUIModel represents a single model entry from Open WebUI.
type OWUIModel struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	OwnedBy string          `json:"owned_by"`
	Ollama  *OWUIOllamaInfo `json:"ollama,omitempty"`
	Info    *OWUIModelInfo  `json:"info,omitempty"`
}

// OWUIOllamaInfo contains Ollama-specific metadata for a model.
type OWUIOllamaInfo struct {
	Name          string           `json:"name"`
	Model         string           `json:"model"`
	ModifiedAt    string           `json:"modified_at"`
	Size          int64            `json:"size"`
	Digest        string           `json:"digest"`
	Details       OWUIModelDetails `json:"details"`
	ContextLength int              `json:"context_length"`
	Capabilities  []string         `json:"capabilities"`
	ModelInfo     map[string]any   `json:"model_info"`
}

// OWUIModelDetails contains detailed model metadata from Ollama.
type OWUIModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OWUIModelInfo contains Open WebUI model info metadata.
type OWUIModelInfo struct {
	Meta *OWUIModelMeta `json:"meta,omitempty"`
}

// OWUIModelMeta contains Open WebUI model meta information.
type OWUIModelMeta struct {
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	Description  string          `json:"description,omitempty"`
}

// ----- Client -----

// OwuiClient is a typed client for the Open WebUI OpenAI-compatible API,
// backed by the go-openai SDK.
type OwuiClient struct {
	client           *openai.Client
	streamClient     *openai.Client
	httpClient       *http.Client
	streamHTTPClient *http.Client
	cfg              *config.Config
}

// New creates a new Open WebUI client using the go-openai SDK.
func New(cfg *config.Config) *OwuiClient {
	baseURL := cfg.UpstreamURL("")

	// Normal client with configured timeout.
	normalCfg := openai.DefaultConfig(cfg.Token)
	normalCfg.BaseURL = baseURL
	normalCfg.APIType = openai.APITypeOpenAI
	normalCfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}

	// Streaming client with no timeout (context controls lifetime).
	streamCfg := openai.DefaultConfig(cfg.Token)
	streamCfg.BaseURL = baseURL
	streamCfg.APIType = openai.APITypeOpenAI
	streamCfg.HTTPClient = &http.Client{Timeout: 0}

	return &OwuiClient{
		client:           openai.NewClientWithConfig(normalCfg),
		streamClient:     openai.NewClientWithConfig(streamCfg),
		httpClient:       &http.Client{Timeout: cfg.Timeout},
		streamHTTPClient: &http.Client{Timeout: 0},
		cfg:              cfg,
	}
}

// ListModels fetches all available models from Open WebUI.
// Uses raw net/http because the go-openai SDK forces /v1/models,
// but Open WebUI serves /api/models.
func (c *OwuiClient) ListModels(ctx context.Context) (*OWUIModelList, error) {
	url := c.cfg.UpstreamURL("/models")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("User-Agent", "owui-proxy/1.0")

	slog.Debug("upstream request", "method", "GET", "url", url, "token", MaskToken(c.cfg.Token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	var models OWUIModelList
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("decoding models response: %w", err)
	}

	return &models, nil
}

// ChatCompletion sends a non-streaming chat completion request.
func (c *OwuiClient) ChatCompletion(
	ctx context.Context,
	req openai.ChatCompletionRequest,
) (openai.ChatCompletionResponse, error) {
	return c.client.CreateChatCompletion(ctx, req)
}

// ChatCompletionStream sends a streaming chat completion request.
// The returned stream is owned by the caller who must call stream.Close().
func (c *OwuiClient) ChatCompletionStream(
	ctx context.Context,
	req openai.ChatCompletionRequest,
) (*openai.ChatCompletionStream, error) {
	return c.streamClient.CreateChatCompletionStream(ctx, req)
}

// CreateEmbedding sends an embedding request to Open WebUI.
func (c *OwuiClient) CreateEmbedding(
	ctx context.Context,
	req openai.EmbeddingRequest,
) (openai.EmbeddingResponse, error) {
	return c.client.CreateEmbeddings(ctx, req)
}

// ProxyRequest forwards a raw HTTP request body to the given upstream path
// and returns the upstream response. The caller is responsible for closing
// the response body. Uses the streaming HTTP client (no timeout) so SSE
// responses are not cut off.
func (c *OwuiClient) ProxyRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	url := c.cfg.UpstreamURL(path)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating proxy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "owui-proxy/1.0")

	slog.Debug("upstream proxy request", "method", method, "url", url)

	return c.streamHTTPClient.Do(req)
}

// ----- Helpers -----

// MaskToken masks a token for logging.
func MaskToken(token string) string {
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
func TimeNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
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
