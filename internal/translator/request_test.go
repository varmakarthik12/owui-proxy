package translator

import (
	"encoding/json"
	"testing"

	ollamaapi "github.com/ollama/ollama/api"
	openai "github.com/sashabaranov/go-openai"
)

// T1: ChatToOpenAI — request with tools is forwarded.
func TestChatToOpenAI_WithTools(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "llama3.1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "What is the weather?"},
		},
		Tools: ollamaapi.Tools{
			{
				Type: "function",
				Function: ollamaapi.ToolFunction{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: ollamaapi.ToolFunctionParameters{
						Type:     "object",
						Required: []string{"location"},
					},
				},
			},
		},
	}
	stream := false
	req.Stream = &stream

	result := ChatToOpenAI(req)

	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Function.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got '%s'", result.Tools[0].Function.Name)
	}
	if result.Tools[0].Type != openai.ToolTypeFunction {
		t.Errorf("expected tool type 'function', got '%s'", result.Tools[0].Type)
	}
}

// T2: ChatToOpenAI — message with role="tool" maps to OpenAI tool message.
func TestChatToOpenAI_ToolMessage(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "llama3.1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "What is the weather?"},
			{Role: "assistant", Content: "Let me check."},
			{Role: "tool", Content: `{"temperature": 22}`, ToolCallID: "call_123"},
		},
	}
	stream := false
	req.Stream = &stream

	result := ChatToOpenAI(req)

	if len(result.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result.Messages))
	}
	toolMsg := result.Messages[2]
	if toolMsg.Role != "tool" {
		t.Errorf("expected role 'tool', got '%s'", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "call_123" {
		t.Errorf("expected tool_call_id 'call_123', got '%s'", toolMsg.ToolCallID)
	}
	if toolMsg.Content != `{"temperature": 22}` {
		t.Errorf("expected tool content, got '%s'", toolMsg.Content)
	}
}

// T3: ChatToOpenAI — message with images maps to multi-part content.
func TestChatToOpenAI_WithImages(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "llava",
		Messages: []ollamaapi.Message{
			{
				Role:    "user",
				Content: "Describe this image",
				Images:  []ollamaapi.ImageData{[]byte("fake-image-data")},
			},
		},
	}
	stream := false
	req.Stream = &stream

	result := ChatToOpenAI(req)

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	msg := result.Messages[0]
	if len(msg.MultiContent) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(msg.MultiContent))
	}
	if msg.MultiContent[0].Type != openai.ChatMessagePartTypeText {
		t.Errorf("expected first part to be text, got '%s'", msg.MultiContent[0].Type)
	}
	if msg.MultiContent[1].Type != openai.ChatMessagePartTypeImageURL {
		t.Errorf("expected second part to be image_url, got '%s'", msg.MultiContent[1].Type)
	}
	if msg.MultiContent[1].ImageURL == nil {
		t.Fatal("expected image URL to be set")
	}
}

// T9: GenerateToOpenAI — suffix is included in prompt when present.
func TestGenerateToOpenAI_WithSuffix(t *testing.T) {
	req := &ollamaapi.GenerateRequest{
		Model:  "codellama",
		Prompt: "func hello() {",
		Suffix: "}",
	}
	stream := false
	req.Stream = &stream

	result := GenerateToOpenAI(req)

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}
	content := result.Messages[0].Content
	if content == "" {
		t.Fatal("expected non-empty content")
	}
	// Should contain both prompt and suffix.
	if !containsStr(content, "func hello() {") || !containsStr(content, "}") {
		t.Errorf("expected prompt and suffix in content, got '%s'", content)
	}
}

// T4: ChatCompletionToChat — tool_calls in response are mapped correctly.
func TestChatCompletionToChat_WithToolCalls(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Model: "llama3.1",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "",
					ToolCalls: []openai.ToolCall{
						{
							ID:   "call_1",
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"location":"Paris"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	result := ChatCompletionToChat(resp, "llama3.1")

	if len(result.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.Message.ToolCalls))
	}
	tc := result.Message.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%s'", tc.Function.Name)
	}
	// Check arguments were parsed.
	argsJSON, _ := json.Marshal(tc.Function.Arguments)
	if !containsStr(string(argsJSON), "Paris") {
		t.Errorf("expected arguments to contain 'Paris', got '%s'", string(argsJSON))
	}
	if result.DoneReason != "tool_calls" {
		t.Errorf("expected done_reason 'tool_calls', got '%s'", result.DoneReason)
	}
}

// T5: ChatCompletionToChat — thinking/reasoning_content is mapped to Message.Thinking.
func TestChatCompletionToChat_WithThinking(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Model: "deepseek-r1",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Role:             "assistant",
					Content:          "The answer is 42.",
					ReasoningContent: "Let me think about this...",
				},
				FinishReason: "stop",
			},
		},
	}

	result := ChatCompletionToChat(resp, "deepseek-r1")

	if result.Message.Thinking != "Let me think about this..." {
		t.Errorf("expected thinking content, got '%s'", result.Message.Thinking)
	}
	if result.Message.Content != "The answer is 42." {
		t.Errorf("expected content 'The answer is 42.', got '%s'", result.Message.Content)
	}
}

// T10: EmbeddingToOllamaEmbeddings — all embeddings are preserved.
func TestEmbeddingToOllamaEmbeddings(t *testing.T) {
	resp := openai.EmbeddingResponse{
		Model: "nomic-embed",
		Data: []openai.Embedding{
			{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
			{Embedding: []float32{0.4, 0.5, 0.6}, Index: 1},
			{Embedding: []float32{0.7, 0.8, 0.9}, Index: 2},
		},
	}

	result := EmbeddingToOllamaEmbeddings(resp)

	if len(result.Embeddings) != 3 {
		t.Fatalf("expected 3 embeddings, got %d", len(result.Embeddings))
	}
	for i, emb := range result.Embeddings {
		if len(emb) != 3 {
			t.Errorf("embedding %d: expected 3 dimensions, got %d", i, len(emb))
		}
	}
	if result.Model != "nomic-embed" {
		t.Errorf("expected model 'nomic-embed', got '%s'", result.Model)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && findStr(s, sub)
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// T11: ChatToOpenAI — Think=true maps to ReasoningEffort "high".
func TestChatToOpenAI_ThinkTrue(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "deepseek-r1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "Explain quantum computing."},
		},
	}
	stream := false
	req.Stream = &stream

	think := ollamaapi.ThinkValue{Value: true}
	req.Think = &think

	result := ChatToOpenAI(req)

	if result.ReasoningEffort != "high" {
		t.Errorf("expected reasoning_effort 'high', got '%s'", result.ReasoningEffort)
	}
}

// T12: ChatToOpenAI — Think="medium" maps to ReasoningEffort "medium".
func TestChatToOpenAI_ThinkMedium(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "deepseek-r1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "Hello"},
		},
	}
	stream := false
	req.Stream = &stream

	think := ollamaapi.ThinkValue{Value: "medium"}
	req.Think = &think

	result := ChatToOpenAI(req)

	if result.ReasoningEffort != "medium" {
		t.Errorf("expected reasoning_effort 'medium', got '%s'", result.ReasoningEffort)
	}
}

// T13: ChatToOpenAI — Think=false does not set ReasoningEffort.
func TestChatToOpenAI_ThinkFalse(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "llama3.1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "Hello"},
		},
	}
	stream := false
	req.Stream = &stream

	think := ollamaapi.ThinkValue{Value: false}
	req.Think = &think

	result := ChatToOpenAI(req)

	if result.ReasoningEffort != "" {
		t.Errorf("expected empty reasoning_effort, got '%s'", result.ReasoningEffort)
	}
}

// T14: ChatToOpenAI — tool parameters are serialized as plain JSON.
func TestChatToOpenAI_ToolParamsNormalized(t *testing.T) {
	req := &ollamaapi.ChatRequest{
		Model: "llama3.1",
		Messages: []ollamaapi.Message{
			{Role: "user", Content: "What is the weather?"},
		},
		Tools: ollamaapi.Tools{
			{
				Type: "function",
				Function: ollamaapi.ToolFunction{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: ollamaapi.ToolFunctionParameters{
						Type:     "object",
						Required: []string{"location"},
					},
				},
			},
		},
	}
	stream := false
	req.Stream = &stream

	result := ChatToOpenAI(req)

	if len(result.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result.Tools))
	}

	// The parameters should be a plain map, not the Ollama custom type.
	params := result.Tools[0].Function.Parameters
	paramsMap, ok := params.(map[string]any)
	if !ok {
		t.Fatalf("expected params to be map[string]any, got %T", params)
	}
	if paramsMap["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", paramsMap["type"])
	}

	// Verify it serializes cleanly to JSON.
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal normalized params: %v", err)
	}
	if !containsStr(string(data), `"type":"object"`) {
		t.Errorf("expected JSON to contain type:object, got %s", string(data))
	}
}
