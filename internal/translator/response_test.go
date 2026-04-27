package translator

import (
	"testing"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

const testDefaultCtxLen = 262144

// T6: ModelsToTags — Ollama-backed model uses OWUI's ollama sub-object metadata.
func TestModelsToTags_OllamaModel(t *testing.T) {
	models := &owuiclient.OWUIModelList{
		Data: []owuiclient.OWUIModel{
			{
				ID:      "codestral:latest",
				Created: 1700000000,
				Ollama: &owuiclient.OWUIOllamaInfo{
					Name:       "codestral:latest",
					Model:      "codestral:latest",
					ModifiedAt: "2024-06-15T10:30:00Z",
					Size:       4500000000,
					Digest:     "sha256:abc123",
					Details: owuiclient.OWUIModelDetails{
						Family:            "starcoder",
						Families:          []string{"starcoder"},
						ParameterSize:     "8B",
						QuantizationLevel: "Q4_0",
						Format:            "gguf",
					},
				},
			},
		},
	}

	result := ModelsToTags(models)

	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}

	m := result.Models[0]
	if m.Name != "codestral:latest" {
		t.Errorf("expected name 'codestral:latest', got '%s'", m.Name)
	}
	if m.Size != 4500000000 {
		t.Errorf("expected size 4500000000, got %d", m.Size)
	}
	if m.Digest != "sha256:abc123" {
		t.Errorf("expected digest 'sha256:abc123', got '%s'", m.Digest)
	}
	if m.Details.Family != "starcoder" {
		t.Errorf("expected family 'starcoder', got '%s'", m.Details.Family)
	}
	if m.Details.ParameterSize != "8B" {
		t.Errorf("expected parameter_size '8B', got '%s'", m.Details.ParameterSize)
	}
	if m.Details.QuantizationLevel != "Q4_0" {
		t.Errorf("expected quantization_level 'Q4_0', got '%s'", m.Details.QuantizationLevel)
	}
}

// T7: ModelsToTags — non-Ollama model has empty Details.
func TestModelsToTags_NonOllamaModel(t *testing.T) {
	models := &owuiclient.OWUIModelList{
		Data: []owuiclient.OWUIModel{
			{
				ID:      "gpt-4o",
				Created: 1700000000,
			},
		},
	}

	result := ModelsToTags(models)

	if len(result.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result.Models))
	}

	m := result.Models[0]
	if m.Name != "gpt-4o" {
		t.Errorf("expected name 'gpt-4o', got '%s'", m.Name)
	}
	if m.Details.Family != "" {
		t.Errorf("expected empty family, got '%s'", m.Details.Family)
	}
	if m.Details.ParameterSize != "" {
		t.Errorf("expected empty parameter_size, got '%s'", m.Details.ParameterSize)
	}
	if m.Size != 0 {
		t.Errorf("expected size 0, got %d", m.Size)
	}
	if m.Digest != "" {
		t.Errorf("expected empty digest, got '%s'", m.Digest)
	}
}

// T8: ModelToShow — default capabilities are always appended, merged with upstream.
func TestModelToShow_CapabilitiesAlwaysAppended(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID: "codestral",
		Ollama: &owuiclient.OWUIOllamaInfo{
			Capabilities: []string{"embedding"},
			Details: owuiclient.OWUIModelDetails{
				Family: "starcoder",
			},
		},
		Info: &owuiclient.OWUIModelInfo{
			Meta: &owuiclient.OWUIModelMeta{
				Capabilities: map[string]bool{
					"usage": true,
				},
			},
		},
	}

	result := ModelToShow(m, false, testDefaultCtxLen, false)

	capSet := make(map[string]bool)
	for _, c := range result.Capabilities {
		capSet[c.String()] = true
	}

	// Default caps always appended.
	for _, expected := range []string{"completion", "vision", "tools", "thinking"} {
		if !capSet[expected] {
			t.Errorf("expected default capability '%s'", expected)
		}
	}
	// Upstream caps also present.
	if !capSet["embedding"] {
		t.Error("expected upstream capability 'embedding'")
	}
	if !capSet["usage"] {
		t.Error("expected upstream capability 'usage'")
	}
	// 4 defaults + embedding + usage = 6
	if len(result.Capabilities) != 6 {
		t.Errorf("expected 6 capabilities, got %d", len(result.Capabilities))
	}
	if result.Details.Family != "starcoder" {
		t.Errorf("expected family 'starcoder', got '%s'", result.Details.Family)
	}
}

// T8b: ModelToShow — non-Ollama model still gets default capabilities.
func TestModelToShow_NonOllamaGetsDefaults(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID:      "gpt-4o",
		Created: 1700000000,
	}

	result := ModelToShow(m, false, testDefaultCtxLen, false)

	capSet := make(map[string]bool)
	for _, c := range result.Capabilities {
		capSet[c.String()] = true
	}

	for _, expected := range []string{"completion", "vision", "tools", "thinking"} {
		if !capSet[expected] {
			t.Errorf("expected default capability '%s'", expected)
		}
	}
	if len(result.Capabilities) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(result.Capabilities))
	}
}

// T8c: ModelToShow — noDefaultCaps=true disables appending defaults.
func TestModelToShow_NoDefaultCaps(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID: "codestral",
		Ollama: &owuiclient.OWUIOllamaInfo{
			Capabilities: []string{"embedding"},
		},
	}

	result := ModelToShow(m, true, testDefaultCtxLen, false)

	if len(result.Capabilities) != 1 {
		t.Errorf("expected 1 capability (only upstream), got %d", len(result.Capabilities))
	}
	if result.Capabilities[0].String() != "embedding" {
		t.Errorf("expected 'embedding', got '%s'", result.Capabilities[0])
	}
}

// T8d: ModelToShow — model without model_info gets default context length.
func TestModelToShow_DefaultContextLength(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID:      "custom-model",
		Created: 1700000000,
	}

	result := ModelToShow(m, false, testDefaultCtxLen, false)

	if result.ModelInfo == nil {
		t.Fatal("expected model_info to be populated")
	}

	ctxLen, ok := result.ModelInfo["general.context_length"]
	if !ok {
		t.Fatal("expected general.context_length in model_info")
	}
	if ctxLen != 262144 {
		t.Errorf("expected context_length 262144, got %v", ctxLen)
	}
}

// T8e: ModelToShow — small upstream context_length is raised to default.
func TestModelToShow_SmallContextLengthRaised(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID: "codestral",
		Ollama: &owuiclient.OWUIOllamaInfo{
			Capabilities: []string{"completion"},
			ModelInfo: map[string]any{
				"starcoder.context_length": 32768,
			},
		},
	}

	result := ModelToShow(m, false, testDefaultCtxLen, false)

	if result.ModelInfo["starcoder.context_length"] != testDefaultCtxLen {
		t.Errorf("expected context_length raised to %d, got %v", testDefaultCtxLen, result.ModelInfo["starcoder.context_length"])
	}
	if _, ok := result.ModelInfo["general.context_length"]; ok {
		t.Error("should not inject general.context_length when model already has a context_length key")
	}
}

// T8f: ModelToShow — large upstream context_length is preserved.
func TestModelToShow_LargeContextLengthPreserved(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID: "gemma4",
		Ollama: &owuiclient.OWUIOllamaInfo{
			Capabilities: []string{"completion"},
			ModelInfo: map[string]any{
				"gemma4.context_length": float64(524288),
			},
		},
	}

	result := ModelToShow(m, false, testDefaultCtxLen, false)

	val := result.ModelInfo["gemma4.context_length"]
	if intVal, ok := val.(float64); !ok || int(intVal) != 524288 {
		t.Errorf("expected context_length 524288 preserved, got %v", val)
	}
}

// T8g: ModelToShow — noCtxOverride=true leaves model_info untouched.
func TestModelToShow_NoContextLengthOverride(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID: "custom-model",
		Ollama: &owuiclient.OWUIOllamaInfo{
			ModelInfo: map[string]any{
				"custom.context_length": 4096,
			},
		},
	}

	result := ModelToShow(m, false, testDefaultCtxLen, true)

	// With override disabled, the small value must be left alone.
	val := result.ModelInfo["custom.context_length"]
	if intVal, ok := toInt(val); !ok || intVal != 4096 {
		t.Errorf("expected context_length 4096 untouched, got %v", val)
	}
}

// T8h: ModelToShow — noCtxOverride=true, no ModelInfo, nothing injected.
func TestModelToShow_NoContextLengthOverride_NoModelInfo(t *testing.T) {
	m := &owuiclient.OWUIModel{
		ID:      "custom-model",
		Created: 1700000000,
	}

	result := ModelToShow(m, false, testDefaultCtxLen, true)

	// model_info should be nil (nothing injected).
	if result.ModelInfo != nil {
		t.Errorf("expected nil model_info when override disabled, got %v", result.ModelInfo)
	}
}
