package handler

import (
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Tags handles GET /api/tags — lists all available models.
func Tags(client *owuiclient.Client) http.HandlerFunc {
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
		slog.Debug("models translated", "count", len(tags.Models))

		owuiclient.WriteJSON(w, http.StatusOK, tags)
	}
}
