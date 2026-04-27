package translator

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// T11: Tool call streaming accumulation — partial argument chunks are
// correctly joined across multiple stream deltas into one ToolCall.
func TestToolCallStreamAccumulation(t *testing.T) {
	// Simulate the accumulation logic used in TranslateStreamToOllamaNDJSON.
	var toolCallAcc []openai.ToolCall

	// Delta 1: tool call starts with ID and function name.
	delta1 := []openai.ToolCall{
		{
			Index: intPtr(0),
			ID:    "call_abc",
			Type:  openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"loc`,
			},
		},
	}

	// Delta 2: continuation of arguments.
	delta2 := []openai.ToolCall{
		{
			Index: intPtr(0),
			Function: openai.FunctionCall{
				Arguments: `ation":`,
			},
		},
	}

	// Delta 3: end of arguments.
	delta3 := []openai.ToolCall{
		{
			Index: intPtr(0),
			Function: openai.FunctionCall{
				Arguments: `"Paris"}`,
			},
		},
	}

	// Process all deltas (same logic as in stream.go).
	for _, deltas := range [][]openai.ToolCall{delta1, delta2, delta3} {
		for _, tc := range deltas {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			for idx >= len(toolCallAcc) {
				toolCallAcc = append(toolCallAcc, openai.ToolCall{})
			}
			if tc.ID != "" {
				toolCallAcc[idx].ID += tc.ID
			}
			if tc.Type != "" {
				toolCallAcc[idx].Type = tc.Type
			}
			if tc.Function.Name != "" {
				toolCallAcc[idx].Function.Name += tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCallAcc[idx].Function.Arguments += tc.Function.Arguments
			}
		}
	}

	// Verify accumulated result.
	if len(toolCallAcc) != 1 {
		t.Fatalf("expected 1 accumulated tool call, got %d", len(toolCallAcc))
	}

	tc := toolCallAcc[0]
	if tc.ID != "call_abc" {
		t.Errorf("expected ID 'call_abc', got '%s'", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("expected function name 'get_weather', got '%s'", tc.Function.Name)
	}
	expectedArgs := `{"location":"Paris"}`
	if tc.Function.Arguments != expectedArgs {
		t.Errorf("expected arguments '%s', got '%s'", expectedArgs, tc.Function.Arguments)
	}

	// Verify mapping to Ollama format.
	ollamaToolCalls := mapOpenAIToolCallsToOllama(toolCallAcc)
	if len(ollamaToolCalls) != 1 {
		t.Fatalf("expected 1 ollama tool call, got %d", len(ollamaToolCalls))
	}
	if ollamaToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected ollama function name 'get_weather', got '%s'", ollamaToolCalls[0].Function.Name)
	}
}

func intPtr(i int) *int {
	return &i
}
