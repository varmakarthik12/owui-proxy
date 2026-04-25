package translator

import (
	"encoding/json"
	"fmt"
	"time"
)

// ----- Ollama request types -----

// OllamaGenerateRequest represents an Ollama /api/generate request.
type OllamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system,omitempty"`
	Stream  *bool                  `json:"stream,omitempty"`
	Options map[string]interface{} `json:"options,omitempty"`
	Context []int                  `json:"context,omitempty"` // ignored
	Format  interface{}            `json:"format,omitempty"`
}

// OllamaChatRequest represents an Ollama /api/chat request.
type OllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   *bool                  `json:"stream,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Format   interface{}            `json:"format,omitempty"`
}

// OllamaMessage represents a message in an Ollama chat request.
type OllamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

// OllamaEmbeddingsRequest represents an Ollama /api/embeddings request (legacy).
type OllamaEmbeddingsRequest struct {
	Model  string                 `json:"model"`
	Prompt string                 `json:"prompt"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaEmbedRequest represents an Ollama /api/embed request (newer).
type OllamaEmbedRequest struct {
	Model   string                 `json:"model"`
	Input   interface{}            `json:"input"` // string or []string
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaShowRequest represents an Ollama /api/show request.
type OllamaShowRequest struct {
	Model string `json:"model"`
	Name  string `json:"name"`
}

// ----- OpenAI request types -----

// OpenAIChatRequest is the OpenAI-compatible chat completion request sent upstream.
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Stop        interface{}     `json:"stop,omitempty"`
}

// OpenAIMessage is a message in the OpenAI format.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIEmbeddingRequest is the OpenAI-compatible embedding request.
type OpenAIEmbeddingRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

// ----- Translation functions -----

// GenerateToOpenAI translates an Ollama /api/generate request to an OpenAI chat completion request.
func GenerateToOpenAI(req *OllamaGenerateRequest) (*OpenAIChatRequest, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	messages := make([]OpenAIMessage, 0, 2)

	// Prepend system message if present.
	if req.System != "" {
		messages = append(messages, OpenAIMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// User prompt.
	messages = append(messages, OpenAIMessage{
		Role:    "user",
		Content: req.Prompt,
	})

	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	openaiReq := &OpenAIChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Map options.
	mapOptions(req.Options, openaiReq)

	return openaiReq, nil
}

// ChatToOpenAI translates an Ollama /api/chat request to an OpenAI chat completion request.
func ChatToOpenAI(req *OllamaChatRequest) (*OpenAIChatRequest, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	messages := make([]OpenAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, OpenAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	openaiReq := &OpenAIChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Map options.
	mapOptions(req.Options, openaiReq)

	return openaiReq, nil
}

// EmbeddingsToOpenAI translates an Ollama /api/embeddings request to OpenAI format.
func EmbeddingsToOpenAI(req *OllamaEmbeddingsRequest) (*OpenAIEmbeddingRequest, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	return &OpenAIEmbeddingRequest{
		Model: req.Model,
		Input: []string{req.Prompt},
	}, nil
}

// EmbedToOpenAI translates an Ollama /api/embed request to OpenAI format.
func EmbedToOpenAI(req *OllamaEmbedRequest) (*OpenAIEmbeddingRequest, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Normalize input to []string.
	var input []string
	switch v := req.Input.(type) {
	case string:
		input = []string{v}
	case []interface{}:
		input = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				input = append(input, s)
			}
		}
	default:
		// Try marshaling and unmarshaling to handle edge cases.
		data, err := json.Marshal(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input type: %w", err)
		}
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			// Single string maybe?
			var s string
			if err := json.Unmarshal(data, &s); err != nil {
				return nil, fmt.Errorf("input must be a string or array of strings")
			}
			input = []string{s}
		} else {
			input = arr
		}
	}

	return &OpenAIEmbeddingRequest{
		Model: req.Model,
		Input: input,
	}, nil
}

// IsStreamEnabled checks if streaming is enabled (defaults to true per Ollama spec).
func IsStreamEnabled(stream *bool) bool {
	if stream == nil {
		return true // Ollama defaults to streaming
	}
	return *stream
}

// ----- helpers -----

func mapOptions(opts map[string]interface{}, req *OpenAIChatRequest) {
	if opts == nil {
		return
	}

	if v, ok := getFloat64(opts, "temperature"); ok {
		req.Temperature = &v
	}
	if v, ok := getFloat64(opts, "top_p"); ok {
		req.TopP = &v
	}
	if v, ok := getInt(opts, "num_predict"); ok {
		req.MaxTokens = &v
	}
	if v, ok := opts["stop"]; ok {
		req.Stop = v
	}
}

func getFloat64(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func getInt(m map[string]interface{}, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// TimeNow returns the current UTC time in RFC3339Nano format.
func TimeNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// ModelName returns the effective model name, preferring the Name field for show requests.
func (r *OllamaShowRequest) ModelName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Model
}
