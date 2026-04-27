package handler

import (
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Tags handles GET /api/tags — lists all available models.
// All metadata is sourced from OWUI's GET /api/models response.
// Model names in the response are prefixed with cfg.ModelPrefix.
func Tags(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		models, err := client.ListModels(r.Context())
		if err != nil {
			slog.Error("failed to list models", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"failed to fetch models from Open WebUI: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}

		tags := translator.ModelsToTags(models)

		// Prefix all model names so clients treat them as distinct from real endpoints.
		if cfg.ModelPrefix != "" {
			for i := range tags.Models {
				tags.Models[i].Name = cfg.ModelPrefix + tags.Models[i].Name
				tags.Models[i].Model = cfg.ModelPrefix + tags.Models[i].Model
			}
		}

		slog.Debug("models translated", "count", len(tags.Models))
		owuiclient.WriteJSON(w, http.StatusOK, tags)
	}
}
