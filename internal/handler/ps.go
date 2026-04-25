package handler

import (
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Ps handles GET /api/ps — returns empty running models list (mocked).
func Ps() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.WriteJSON(w, http.StatusOK, translator.OllamaPsResponse{
			Models: []interface{}{},
		})
	}
}
