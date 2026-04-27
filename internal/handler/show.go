package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Show handles POST /api/show — returns model details.
// All metadata is sourced from OWUI's GET /api/models response.
// Accepts model names with or without cfg.ModelPrefix and looks up by real name.
func Show(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.ShowRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		modelName := req.ModelName()
		if modelName == "" {
			owuiclient.WriteError(w, http.StatusBadRequest, "model name is required")
			return
		}

		// Strip prefix before looking up in OWUI.
		lookupName := strings.TrimPrefix(modelName, cfg.ModelPrefix)

		slog.Debug("show request", "model", lookupName)

		models, err := client.ListModels(r.Context())
		if err != nil {
			slog.Error("failed to list models for show", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"failed to fetch models from Open WebUI")
			return
		}

		var found *owuiclient.OWUIModel
		for i := range models.Data {
			if models.Data[i].ID == lookupName {
				found = &models.Data[i]
				break
			}
		}

		if found == nil {
			owuiclient.WriteError(w, http.StatusNotFound,
				"model '"+modelName+"' not found")
			return
		}

		resp := translator.ModelToShow(found, cfg.NoDefaultCapabilities, cfg.DefaultContextLength, cfg.NoContextLengthOverride)
		owuiclient.WriteJSON(w, http.StatusOK, resp)
	}
}
