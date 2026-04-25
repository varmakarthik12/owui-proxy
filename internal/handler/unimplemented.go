package handler

import (
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

const unimplementedMessage = "This operation is not supported by owui-proxy. Manage models directly in Open WebUI."

// Unimplemented returns a handler that responds with 501 Not Implemented
// for Ollama endpoints that cannot be proxied (pull, push, delete, copy, create, blobs).
func Unimplemented() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.WriteError(w, http.StatusNotImplemented, unimplementedMessage)
	}
}
