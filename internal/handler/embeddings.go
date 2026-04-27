package handler

import (
	"log/slog"
	"net/http"
	"strings"

	ollamaapi "github.com/ollama/ollama/api"
	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Embeddings handles POST /api/embeddings — legacy single-prompt embedding.
func Embeddings(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.LegacyEmbeddingsRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		req.Model = strings.TrimPrefix(req.Model, cfg.ModelPrefix)
		slog.Info("embeddings request", "model", req.Model)

		openaiReq, err := translator.LegacyEmbeddingsToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := client.CreateEmbedding(r.Context(), openaiReq)
		if err != nil {
			slog.Error("embedding failed", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}

		// Legacy format: return only the first embedding vector.
		var embedding []float32
		if len(resp.Data) > 0 {
			embedding = resp.Data[0].Embedding
		}
		owuiclient.WriteJSON(w, http.StatusOK, map[string]any{
			"embedding": embedding,
		})
	}
}

// Embed handles POST /api/embed — newer multi-input embedding.
func Embed(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req ollamaapi.EmbedRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		req.Model = strings.TrimPrefix(req.Model, cfg.ModelPrefix)
		slog.Info("embed request", "model", req.Model)

		openaiReq, err := translator.EmbedToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		resp, err := client.CreateEmbedding(r.Context(), openaiReq)
		if err != nil {
			slog.Error("embed failed", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}

		result := translator.EmbeddingToOllamaEmbeddings(resp)
		owuiclient.WriteJSON(w, http.StatusOK, result)
	}
}
