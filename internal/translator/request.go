package translator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	ollamaapi "github.com/ollama/ollama/api"
	openai "github.com/sashabaranov/go-openai"
)

// ChatToOpenAI translates an Ollama /api/chat request to an OpenAI chat completion request.
func ChatToOpenAI(req *ollamaapi.ChatRequest) openai.ChatCompletionRequest {
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))

	for _, m := range req.Messages {
		msg := convertMessage(m)
		messages = append(messages, msg)
	}

	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Map tools — serialize Ollama's custom ToolFunctionParameters to a
	// plain map so the OpenAI SDK can marshal them as a regular JSON schema.
	if len(req.Tools) > 0 {
		openaiReq.Tools = make([]openai.Tool, 0, len(req.Tools))
		for _, t := range req.Tools {
			params := normalizeToolParams(t.Function.Parameters)
			openaiReq.Tools = append(openaiReq.Tools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  params,
				},
			})
		}
	}

	// Map think/reasoning.
	mapThink(req.Think, &openaiReq)

	// Map options.
	mapOptions(req.Options, &openaiReq)

	// Map format.
	mapFormat(req.Format, &openaiReq)

	// Request usage in stream so we get token counts.
	if stream {
		openaiReq.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
	}

	return openaiReq
}

// GenerateToOpenAI translates an Ollama /api/generate request to an OpenAI chat completion request.
func GenerateToOpenAI(req *ollamaapi.GenerateRequest) openai.ChatCompletionRequest {
	messages := make([]openai.ChatCompletionMessage, 0, 2)

	// Prepend system message if present.
	if req.System != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Build user message.
	userContent := req.Prompt
	if req.Suffix != "" {
		// FIM via chat is best-effort; Ollama's suffix maps to infill-style prompting.
		userContent = "Complete the following:\n" + req.Prompt + "\n[...]\n" + req.Suffix
	}

	userMsg := openai.ChatCompletionMessage{
		Role:    "user",
		Content: userContent,
	}

	// Attach images to user message if present.
	if len(req.Images) > 0 {
		parts := []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeText,
				Text: userContent,
			},
		}
		for _, img := range req.Images {
			b64 := base64.StdEncoding.EncodeToString(img)
			parts = append(parts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: "data:image/jpeg;base64," + b64,
				},
			})
		}
		userMsg.Content = ""
		userMsg.MultiContent = parts
	}

	messages = append(messages, userMsg)

	stream := true
	if req.Stream != nil {
		stream = *req.Stream
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Map options.
	mapOptions(req.Options, &openaiReq)

	// Map format.
	mapFormat(req.Format, &openaiReq)

	// Request usage in stream.
	if stream {
		openaiReq.StreamOptions = &openai.StreamOptions{IncludeUsage: true}
	}

	return openaiReq
}

// IsStreamEnabled checks if streaming is enabled (defaults to true per Ollama spec).
func IsStreamEnabled(stream *bool) bool {
	if stream == nil {
		return true
	}
	return *stream
}

// convertMessage converts an Ollama message to an OpenAI message.
func convertMessage(m ollamaapi.Message) openai.ChatCompletionMessage {
	msg := openai.ChatCompletionMessage{
		Role:    m.Role,
		Content: m.Content,
	}

	// Map tool role messages.
	if m.Role == "tool" {
		msg.ToolCallID = m.ToolCallID
	}

	// Map tool calls from assistant messages.
	if len(m.ToolCalls) > 0 {
		msg.ToolCalls = make([]openai.ToolCall, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			argBytes, _ := json.Marshal(tc.Function.Arguments)
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: string(argBytes),
				},
			})
		}
	}

	// Map images to multi-part content.
	if len(m.Images) > 0 {
		parts := []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeText,
				Text: m.Content,
			},
		}
		for _, img := range m.Images {
			b64 := base64.StdEncoding.EncodeToString(img)
			parts = append(parts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL: "data:image/jpeg;base64," + b64,
				},
			})
		}
		msg.Content = ""
		msg.MultiContent = parts
	}

	return msg
}

// mapOptions maps Ollama options to OpenAI request fields.
func mapOptions(opts map[string]any, req *openai.ChatCompletionRequest) {
	if opts == nil {
		return
	}

	if v, ok := getFloat32(opts, "temperature"); ok {
		req.Temperature = v
	}
	if v, ok := getFloat32(opts, "top_p"); ok {
		req.TopP = v
	}
	if v, ok := getInt(opts, "num_predict"); ok {
		req.MaxTokens = v
	}
	if v, ok := getInt(opts, "seed"); ok {
		req.Seed = &v
	}
	if v, ok := opts["stop"]; ok {
		if stops, ok := toStringSlice(v); ok {
			req.Stop = stops
		}
	}
	if v, ok := getFloat32(opts, "repeat_penalty"); ok {
		req.PresencePenalty = v
	}
	if v, ok := getFloat32(opts, "frequency_penalty"); ok {
		req.FrequencyPenalty = v
	}
	if v, ok := getFloat32(opts, "presence_penalty"); ok {
		req.PresencePenalty = v
	}
}

// mapFormat maps Ollama format to OpenAI response format.
func mapFormat(format json.RawMessage, req *openai.ChatCompletionRequest) {
	if len(format) == 0 {
		return
	}

	// Try to detect if it's a simple string "json".
	var formatStr string
	if err := json.Unmarshal(format, &formatStr); err == nil {
		if formatStr == "json" {
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		}
		return
	}

	// Otherwise, assume it's a JSON schema object.
	req.ResponseFormat = &openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
		JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
			Name:   "response",
			Schema: jsonRawMessageMarshaler(format),
			Strict: true,
		},
	}
}

// jsonRawMessageMarshaler wraps json.RawMessage to implement json.Marshaler.
type jsonRawMessageMarshaler json.RawMessage

func (j jsonRawMessageMarshaler) MarshalJSON() ([]byte, error) {
	return json.RawMessage(j), nil
}

// normalizeToolParams converts Ollama's custom ToolFunctionParameters (which
// uses ordered-map types with custom JSON marshaling) into a plain map[string]any
// so the OpenAI SDK serializes it as a regular JSON schema.
func normalizeToolParams(params any) any {
	if params == nil {
		return nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return params // fallback to original
	}
	var plain map[string]any
	if err := json.Unmarshal(data, &plain); err != nil {
		return params
	}
	return plain
}

// mapThink maps Ollama's Think field to OpenAI's ReasoningEffort.
// Ollama Think can be a bool (true → "high") or a string ("high","medium","low").
func mapThink(think *ollamaapi.ThinkValue, req *openai.ChatCompletionRequest) {
	if think == nil {
		return
	}
	if think.IsBool() {
		if think.Bool() {
			req.ReasoningEffort = "high"
		}
		return
	}
	if think.IsString() {
		req.ReasoningEffort = think.String()
	}
}

// ----- option extraction helpers -----

func getFloat32(m map[string]any, key string) (float32, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return float32(n), true
	case float32:
		return n, true
	case int:
		return float32(n), true
	case json.Number:
		f, err := n.Float64()
		return float32(f), err == nil
	default:
		return 0, false
	}
}

func getInt(m map[string]any, key string) (int, bool) {
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

func toStringSlice(v any) ([]string, bool) {
	switch s := v.(type) {
	case []string:
		return s, true
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, len(result) > 0
	case string:
		return []string{s}, true
	default:
		return nil, false
	}
}

// ShowRequest is a simple type for parsing /api/show request bodies.
type ShowRequest struct {
	Model string `json:"model"`
	Name  string `json:"name"`
}

// ModelName returns the effective model name, preferring the Name field.
func (r *ShowRequest) ModelName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Model
}

// LegacyEmbeddingsRequest represents an Ollama /api/embeddings request (legacy).
type LegacyEmbeddingsRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Options map[string]any `json:"options,omitempty"`
}

// EmbedToOpenAI translates an Ollama /api/embed request to an OpenAI embedding request.
func EmbedToOpenAI(req *ollamaapi.EmbedRequest) (openai.EmbeddingRequest, error) {
	if req.Model == "" {
		return openai.EmbeddingRequest{}, fmt.Errorf("model is required")
	}

	// Normalize input to []string.
	var input []string
	switch v := req.Input.(type) {
	case string:
		input = []string{v}
	case []any:
		input = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				input = append(input, s)
			}
		}
	case []string:
		input = v
	default:
		data, err := json.Marshal(req.Input)
		if err != nil {
			return openai.EmbeddingRequest{}, fmt.Errorf("invalid input type: %w", err)
		}
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			var s string
			if err := json.Unmarshal(data, &s); err != nil {
				return openai.EmbeddingRequest{}, fmt.Errorf("input must be a string or array of strings")
			}
			input = []string{s}
		} else {
			input = arr
		}
	}

	return openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(req.Model),
		Input: input,
	}, nil
}

// LegacyEmbeddingsToOpenAI translates an Ollama /api/embeddings (legacy) request to OpenAI format.
func LegacyEmbeddingsToOpenAI(req *LegacyEmbeddingsRequest) (openai.EmbeddingRequest, error) {
	if req.Model == "" {
		return openai.EmbeddingRequest{}, fmt.Errorf("model is required")
	}

	return openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(req.Model),
		Input: []string{req.Prompt},
	}, nil
}
