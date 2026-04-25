package handler

import (
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Version handles GET /api/version — mocked locally, never forwarded.
func Version(mockVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.WriteJSON(w, http.StatusOK, translator.OllamaVersionResponse{
			Version: mockVersion,
		})
	}
}
