package translator

import (
	"encoding/json"
	"time"

	ollamaapi "github.com/ollama/ollama/api"
	"github.com/ollama/ollama/types/model"
	openai "github.com/sashabaranov/go-openai"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// ModelsToTags translates an Open WebUI model list to Ollama tags format.
// All metadata is sourced from OWUI's GET /api/models response — the ollama
// sub-object carries details (family, parameter_size, etc.) and the info
// sub-object carries OWUI-level capabilities.
func ModelsToTags(models *owuiclient.OWUIModelList) *ollamaapi.ListResponse {
	ollamaModels := make([]ollamaapi.ListModelResponse, 0, len(models.Data))

	for _, m := range models.Data {
		lmr := ollamaapi.ListModelResponse{
			Name:  m.ID,
			Model: m.ID,
		}

		if m.Ollama != nil {
			// Use the ollama sub-object from OWUI's /api/models response.
			if m.Ollama.ModifiedAt != "" {
				if t, err := time.Parse(time.RFC3339Nano, m.Ollama.ModifiedAt); err == nil {
					lmr.ModifiedAt = t
				} else if t, err := time.Parse(time.RFC3339, m.Ollama.ModifiedAt); err == nil {
					lmr.ModifiedAt = t
				} else {
					lmr.ModifiedAt = time.Unix(m.Created, 0).UTC()
				}
			} else {
				lmr.ModifiedAt = time.Unix(m.Created, 0).UTC()
			}
			lmr.Size = m.Ollama.Size
			lmr.Digest = m.Ollama.Digest
			lmr.Details = ollamaapi.ModelDetails{
				ParentModel:       m.Ollama.Details.ParentModel,
				Format:            m.Ollama.Details.Format,
				Family:            m.Ollama.Details.Family,
				Families:          m.Ollama.Details.Families,
				ParameterSize:     m.Ollama.Details.ParameterSize,
				QuantizationLevel: m.Ollama.Details.QuantizationLevel,
			}
		} else {
			// Non-Ollama model: use created timestamp, leave details empty.
			if m.Created > 0 {
				lmr.ModifiedAt = time.Unix(m.Created, 0).UTC()
			} else {
				lmr.ModifiedAt = time.Now().UTC()
			}
		}

		ollamaModels = append(ollamaModels, lmr)
	}

	return &ollamaapi.ListResponse{Models: ollamaModels}
}

// defaultCapabilities are always appended to every model unless disabled.
var defaultCapabilities = []string{"completion", "vision", "tools", "thinking"}

// ModelToShow synthesizes an Ollama /api/show response from OWUI model data.
// All metadata is sourced from OWUI's GET /api/models — the ollama sub-object
// provides details and model_info, while info.meta provides capabilities.
// Default capabilities (completion, vision, tools, thinking) are always appended
// unless noDefaultCaps is true. Unless noCtxOverride is true, context length is
// ensured to be at least defaultCtxLen.
func ModelToShow(m *owuiclient.OWUIModel, noDefaultCaps bool, defaultCtxLen int, noCtxOverride bool) *ollamaapi.ShowResponse {
	resp := &ollamaapi.ShowResponse{
		Modelfile: "# Proxied via owui-proxy — modelfile not available",
	}

	if m.Ollama != nil {
		resp.Details = ollamaapi.ModelDetails{
			ParentModel:       m.Ollama.Details.ParentModel,
			Format:            m.Ollama.Details.Format,
			Family:            m.Ollama.Details.Family,
			Families:          m.Ollama.Details.Families,
			ParameterSize:     m.Ollama.Details.ParameterSize,
			QuantizationLevel: m.Ollama.Details.QuantizationLevel,
		}
		resp.ModelInfo = m.Ollama.ModelInfo

		// Set ModifiedAt from ollama metadata.
		if m.Ollama.ModifiedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, m.Ollama.ModifiedAt); err == nil {
				resp.ModifiedAt = t
			} else if t, err := time.Parse(time.RFC3339, m.Ollama.ModifiedAt); err == nil {
				resp.ModifiedAt = t
			}
		}
	}

	// Fallback ModifiedAt from created timestamp.
	if resp.ModifiedAt.IsZero() && m.Created > 0 {
		resp.ModifiedAt = time.Unix(m.Created, 0).UTC()
	}

	// Build capabilities from both ollama and OWUI info sources.
	capSet := make(map[string]bool)

	if m.Ollama != nil {
		for _, c := range m.Ollama.Capabilities {
			capSet[c] = true
		}
	}

	if m.Info != nil && m.Info.Meta != nil {
		for k, v := range m.Info.Meta.Capabilities {
			if v {
				capSet[k] = true
			}
		}
	}

	// Always append default capabilities unless explicitly disabled.
	if !noDefaultCaps {
		for _, c := range defaultCapabilities {
			capSet[c] = true
		}
	}

	caps := make([]model.Capability, 0, len(capSet))
	for c := range capSet {
		caps = append(caps, model.Capability(c))
	}
	resp.Capabilities = caps

	// Ensure model_info has a context_length at least as large as the default,
	// unless the context length override is disabled.
	if !noCtxOverride {
		if resp.ModelInfo == nil {
			resp.ModelInfo = make(map[string]any)
		}
		ensureContextLength(resp.ModelInfo, defaultCtxLen)
	}

	return resp
}

// ensureContextLength finds any *.context_length key in model_info. If found
// and the value is smaller than minCtxLen, it's updated. If no key exists,
// "general.context_length" is injected with minCtxLen.
func ensureContextLength(info map[string]any, minCtxLen int) {
	for k, v := range info {
		if len(k) >= 15 && k[len(k)-15:] == ".context_length" {
			if intVal, ok := toInt(v); ok && intVal >= minCtxLen {
				return // upstream value is large enough
			}
			info[k] = minCtxLen
			return
		}
	}
	info["general.context_length"] = minCtxLen
}

// toInt converts a numeric value from JSON (float64) or int to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

// ChatCompletionToChat translates a non-streaming OpenAI chat completion to Ollama chat format.
func ChatCompletionToChat(resp openai.ChatCompletionResponse, modelName string) *ollamaapi.ChatResponse {
	chatResp := &ollamaapi.ChatResponse{
		Model:     modelName,
		CreatedAt: time.Now().UTC(),
		Done:      true,
		Metrics: ollamaapi.Metrics{
			PromptEvalCount: resp.Usage.PromptTokens,
			EvalCount:       resp.Usage.CompletionTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		chatResp.Message = ollamaapi.Message{
			Role:    "assistant",
			Content: choice.Message.Content,
		}
		chatResp.DoneReason = string(choice.FinishReason)
		if chatResp.DoneReason == "" {
			chatResp.DoneReason = "stop"
		}

		if choice.Message.ReasoningContent != "" {
			chatResp.Message.Thinking = choice.Message.ReasoningContent
		}

		if len(choice.Message.ToolCalls) > 0 {
			chatResp.Message.ToolCalls = mapOpenAIToolCallsToOllama(choice.Message.ToolCalls)
		}
	} else {
		chatResp.Message = ollamaapi.Message{
			Role: "assistant",
		}
		chatResp.DoneReason = "stop"
	}

	return chatResp
}

// ChatCompletionToGenerate translates a non-streaming OpenAI chat completion to Ollama generate format.
func ChatCompletionToGenerate(resp openai.ChatCompletionResponse, modelName string) *ollamaapi.GenerateResponse {
	genResp := &ollamaapi.GenerateResponse{
		Model:     modelName,
		CreatedAt: time.Now().UTC(),
		Done:      true,
		Metrics: ollamaapi.Metrics{
			PromptEvalCount: resp.Usage.PromptTokens,
			EvalCount:       resp.Usage.CompletionTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		genResp.Response = choice.Message.Content
		genResp.DoneReason = string(choice.FinishReason)
		if genResp.DoneReason == "" {
			genResp.DoneReason = "stop"
		}

		if choice.Message.ReasoningContent != "" {
			genResp.Thinking = choice.Message.ReasoningContent
		}
	} else {
		genResp.DoneReason = "stop"
	}

	return genResp
}

// EmbeddingToOllamaEmbeddings translates an OpenAI embedding response to Ollama embed format.
func EmbeddingToOllamaEmbeddings(resp openai.EmbeddingResponse) ollamaapi.EmbedResponse {
	embeddings := make([][]float32, 0, len(resp.Data))
	for _, d := range resp.Data {
		embeddings = append(embeddings, d.Embedding)
	}
	return ollamaapi.EmbedResponse{
		Model:      string(resp.Model),
		Embeddings: embeddings,
	}
}

// mapOpenAIToolCallsToOllama converts OpenAI tool calls to Ollama format.
func mapOpenAIToolCallsToOllama(toolCalls []openai.ToolCall) []ollamaapi.ToolCall {
	result := make([]ollamaapi.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		otc := ollamaapi.ToolCall{
			Function: ollamaapi.ToolCallFunction{
				Name: tc.Function.Name,
			},
		}

		var args ollamaapi.ToolCallFunctionArguments
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
			otc.Function.Arguments = args
		}

		result = append(result, otc)
	}
	return result
}
