package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Embeddings handles POST /api/embeddings — legacy single-prompt embedding.
func Embeddings(client *owuiclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.OllamaEmbeddingsRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		slog.Info("embeddings request", "model", req.Model)

		openaiReq, err := translator.EmbeddingsToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			owuiclient.WriteError(w, http.StatusInternalServerError, "failed to marshal request")
			return
		}

		resp, err := client.CreateEmbedding(r.Context(), bytes.NewReader(reqBody))
		if err != nil {
			slog.Error("embedding failed", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}

		result := translator.EmbeddingToOllamaEmbeddings(resp)
		owuiclient.WriteJSON(w, http.StatusOK, result)
	}
}

// Embed handles POST /api/embed — newer multi-input embedding.
func Embed(client *owuiclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.OllamaEmbedRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		slog.Info("embed request", "model", req.Model)

		openaiReq, err := translator.EmbedToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			owuiclient.WriteError(w, http.StatusInternalServerError, "failed to marshal request")
			return
		}

		resp, err := client.CreateEmbedding(r.Context(), bytes.NewReader(reqBody))
		if err != nil {
			slog.Error("embed failed", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}

		result := translator.EmbeddingToOllamaEmbed(resp, req.Model)
		owuiclient.WriteJSON(w, http.StatusOK, result)
	}
}
