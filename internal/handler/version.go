package handler

import (
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// Version handles GET /api/version — mocked locally, never forwarded.
func Version(mockVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.WriteJSON(w, http.StatusOK, map[string]string{
			"version": mockVersion,
		})
	}
}
