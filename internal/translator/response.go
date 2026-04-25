package translator

import (
	"time"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// ----- Ollama response types -----

// OllamaTagsResponse is the response for GET /api/tags.
type OllamaTagsResponse struct {
	Models []OllamaModelInfo `json:"models"`
}

// OllamaModelInfo describes a model in Ollama format.
type OllamaModelInfo struct {
	Name       string             `json:"name"`
	Model      string             `json:"model"`
	ModifiedAt string             `json:"modified_at"`
	Size       int64              `json:"size"`
	Digest     string             `json:"digest"`
	Details    OllamaModelDetails `json:"details"`
}

// OllamaModelDetails contains detailed model metadata.
type OllamaModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OllamaGenerateResponse is the response for POST /api/generate (non-streaming or final chunk).
type OllamaGenerateResponse struct {
	Model           string `json:"model"`
	CreatedAt       string `json:"created_at"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason,omitempty"`
	Context         []int  `json:"context,omitempty"`
	TotalDuration   int64  `json:"total_duration,omitempty"`
	LoadDuration    int64  `json:"load_duration,omitempty"`
	PromptEvalCount int    `json:"prompt_eval_count,omitempty"`
	EvalCount       int    `json:"eval_count,omitempty"`
	EvalDuration    int64  `json:"eval_duration,omitempty"`
}

// OllamaChatResponse is the response for POST /api/chat (non-streaming or final chunk).
type OllamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         OllamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	LoadDuration    int64         `json:"load_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
	EvalDuration    int64         `json:"eval_duration,omitempty"`
}

// OllamaEmbeddingsResponse is the response for POST /api/embeddings (legacy).
type OllamaEmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}

// OllamaEmbedResponse is the response for POST /api/embed (newer).
type OllamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

// OllamaShowResponse is the response for POST /api/show.
type OllamaShowResponse struct {
	Modelfile    string             `json:"modelfile"`
	Parameters   string             `json:"parameters"`
	Template     string             `json:"template"`
	Details      OllamaModelDetails `json:"details"`
	ModelInfo    map[string]interface{} `json:"model_info"`
	Capabilities []string           `json:"capabilities,omitempty"`
}

// OllamaVersionResponse is the response for GET /api/version.
type OllamaVersionResponse struct {
	Version string `json:"version"`
}

// OllamaPsResponse is the response for GET /api/ps.
type OllamaPsResponse struct {
	Models []interface{} `json:"models"`
}

// ----- Translation functions -----

// ModelsToTags translates an OpenAI model list to Ollama tags format.
func ModelsToTags(models *owuiclient.ModelList) *OllamaTagsResponse {
	ollamaModels := make([]OllamaModelInfo, 0, len(models.Data))

	for _, m := range models.Data {
		modifiedAt := time.Unix(m.Created, 0).UTC().Format(time.RFC3339)
		if m.Created == 0 {
			modifiedAt = time.Now().UTC().Format(time.RFC3339)
		}

		ollamaModels = append(ollamaModels, OllamaModelInfo{
			Name:       m.ID,
			Model:      m.ID,
			ModifiedAt: modifiedAt,
			Size:       0,
			Digest:     "",
			Details: OllamaModelDetails{
				ParentModel:       "",
				Format:            "gguf",
				Family:            inferFamily(m.ID),
				Families:          []string{inferFamily(m.ID)},
				ParameterSize:     "unknown",
				QuantizationLevel: "unknown",
			},
		})
	}

	return &OllamaTagsResponse{Models: ollamaModels}
}

// ModelToShow synthesizes an Ollama /api/show response for a given model.
func ModelToShow(model *owuiclient.Model) *OllamaShowResponse {
	return &OllamaShowResponse{
		Modelfile:  "# Model info not available via Open WebUI proxy",
		Parameters: "",
		Template:   "{{ .Prompt }}",
		Details: OllamaModelDetails{
			ParentModel:       "",
			Format:            "gguf",
			Family:            inferFamily(model.ID),
			Families:          []string{},
			ParameterSize:     "unknown",
			QuantizationLevel: "unknown",
		},
		ModelInfo:    map[string]interface{}{},
		Capabilities: []string{"completion"},
	}
}

// ChatCompletionToGenerate translates a non-streaming OpenAI chat completion to Ollama generate format.
func ChatCompletionToGenerate(resp *owuiclient.ChatCompletionResponse, model string) *OllamaGenerateResponse {
	content := ""
	doneReason := "stop"
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		if resp.Choices[0].FinishReason != "" {
			doneReason = resp.Choices[0].FinishReason
		}
	}

	evalCount := 0
	promptEvalCount := 0
	if resp.Usage != nil {
		evalCount = resp.Usage.CompletionTokens
		promptEvalCount = resp.Usage.PromptTokens
	}

	return &OllamaGenerateResponse{
		Model:           model,
		CreatedAt:       TimeNow(),
		Response:        content,
		Done:            true,
		DoneReason:      doneReason,
		Context:         []int{},
		TotalDuration:   0,
		LoadDuration:    0,
		PromptEvalCount: promptEvalCount,
		EvalCount:       evalCount,
		EvalDuration:    0,
	}
}

// ChatCompletionToChat translates a non-streaming OpenAI chat completion to Ollama chat format.
func ChatCompletionToChat(resp *owuiclient.ChatCompletionResponse, model string) *OllamaChatResponse {
	content := ""
	doneReason := "stop"
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		if resp.Choices[0].FinishReason != "" {
			doneReason = resp.Choices[0].FinishReason
		}
	}

	evalCount := 0
	promptEvalCount := 0
	if resp.Usage != nil {
		evalCount = resp.Usage.CompletionTokens
		promptEvalCount = resp.Usage.PromptTokens
	}

	return &OllamaChatResponse{
		Model:     model,
		CreatedAt: TimeNow(),
		Message: OllamaMessage{
			Role:    "assistant",
			Content: content,
		},
		Done:            true,
		DoneReason:      doneReason,
		TotalDuration:   0,
		LoadDuration:    0,
		PromptEvalCount: promptEvalCount,
		EvalCount:       evalCount,
		EvalDuration:    0,
	}
}

// EmbeddingToOllamaEmbeddings translates an OpenAI embedding response to Ollama legacy format.
func EmbeddingToOllamaEmbeddings(resp *owuiclient.EmbeddingResponse) *OllamaEmbeddingsResponse {
	var embedding []float64
	if len(resp.Data) > 0 {
		embedding = resp.Data[0].Embedding
	}
	return &OllamaEmbeddingsResponse{Embedding: embedding}
}

// EmbeddingToOllamaEmbed translates an OpenAI embedding response to Ollama newer format.
func EmbeddingToOllamaEmbed(resp *owuiclient.EmbeddingResponse, model string) *OllamaEmbedResponse {
	embeddings := make([][]float64, 0, len(resp.Data))
	for _, d := range resp.Data {
		embeddings = append(embeddings, d.Embedding)
	}
	return &OllamaEmbedResponse{
		Model:      model,
		Embeddings: embeddings,
	}
}

// ----- helpers -----

// inferFamily tries to guess a model family from its name for display purposes.
func inferFamily(modelID string) string {
	// Simple heuristic — not authoritative, just for display.
	families := map[string]string{
		"llama":   "llama",
		"mistral": "mistral",
		"mixtral": "mistral",
		"gemma":   "gemma",
		"phi":     "phi",
		"qwen":    "qwen",
		"claude":  "claude",
		"gpt":     "gpt",
		"gemini":  "gemini",
		"codellama": "llama",
		"deepseek": "deepseek",
		"command": "command",
		"nomic":   "nomic",
	}

	lower := toLower(modelID)
	for prefix, family := range families {
		if containsSubstring(lower, prefix) {
			return family
		}
	}
	return "unknown"
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
