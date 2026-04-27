package handler

import (
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// openAIModel is a minimal OpenAI-format model entry.
type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// openAIModelList is the OpenAI /v1/models response format.
type openAIModelList struct {
	Object string        `json:"object"`
	Data   []openAIModel `json:"data"`
}

// V1Models handles GET /v1/models — OpenAI-compatible models list.
// Model IDs are prefixed with cfg.ModelPrefix so clients treat them as distinct
// from any real OpenAI/Claude endpoints. The same prefix is stripped by V1Chat
// before forwarding to OWUI.
func V1Models(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		models, err := client.ListModels(r.Context())
		if err != nil {
			slog.Error("failed to list models for v1/models", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway, "failed to fetch models from Open WebUI")
			return
		}

		list := openAIModelList{
			Object: "list",
			Data:   make([]openAIModel, 0, len(models.Data)),
		}
		for _, m := range models.Data {
			id := cfg.ModelPrefix + m.ID
			list.Data = append(list.Data, openAIModel{
				ID:      id,
				Object:  "model",
				Created: m.Created,
				OwnedBy: m.OwnedBy,
			})
		}

		owuiclient.WriteJSON(w, http.StatusOK, list)
	}
}
