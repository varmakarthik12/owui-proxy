package handler

import (
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Show handles POST /api/show — returns model details.
// Synthesized from the models list since Open WebUI doesn't expose
// individual model details.
func Show(client *owuiclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.OllamaShowRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		modelName := req.ModelName()
		if modelName == "" {
			owuiclient.WriteError(w, http.StatusBadRequest, "model name is required")
			return
		}

		slog.Debug("show request", "model", modelName)

		// Fetch all models and find the requested one.
		models, err := client.ListModels(r.Context())
		if err != nil {
			slog.Error("failed to list models for show", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"failed to fetch models from Open WebUI")
			return
		}

		// Find the matching model.
		var found *owuiclient.Model
		for i := range models.Data {
			if models.Data[i].ID == modelName {
				found = &models.Data[i]
				break
			}
		}

		if found == nil {
			owuiclient.WriteError(w, http.StatusNotFound,
				"model '"+modelName+"' not found")
			return
		}

		resp := translator.ModelToShow(found)
		owuiclient.WriteJSON(w, http.StatusOK, resp)
	}
}
