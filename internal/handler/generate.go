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

// Generate handles POST /api/generate — streaming and non-streaming text generation.
func Generate(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req ollamaapi.GenerateRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Keep the prefixed name for responses; strip before forwarding upstream.
		responseModel := req.Model
		req.Model = strings.TrimPrefix(req.Model, cfg.ModelPrefix)

		slog.Info("generate request", "model", req.Model)

		// Translate to OpenAI format.
		openaiReq := translator.GenerateToOpenAI(&req)

		if translator.IsStreamEnabled(req.Stream) {
			// Streaming mode.
			stream, err := client.ChatCompletionStream(r.Context(), openaiReq)
			if err != nil {
				slog.Error("streaming generate failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}
			defer stream.Close()

			if err := translator.TranslateStreamToOllamaNDJSON(
				r.Context(), w, stream, responseModel, translator.StreamModeGenerate,
			); err != nil {
				slog.Error("stream translation failed", "error", err)
			}
		} else {
			// Non-streaming mode.
			resp, err := client.ChatCompletion(r.Context(), openaiReq)
			if err != nil {
				slog.Error("generate completion failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}

			result := translator.ChatCompletionToGenerate(resp, responseModel)
			owuiclient.WriteJSON(w, http.StatusOK, result)
		}
	}
}
