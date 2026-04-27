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

// Chat handles POST /api/chat — streaming and non-streaming chat completion.
func Chat(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req ollamaapi.ChatRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Keep the prefixed name for responses (clients expect to see what they sent).
		// Strip the prefix before forwarding upstream.
		responseModel := req.Model
		req.Model = strings.TrimPrefix(req.Model, cfg.ModelPrefix)

		slog.Info("chat request", "model", req.Model, "messages", len(req.Messages))

		// Translate to OpenAI format.
		openaiReq := translator.ChatToOpenAI(&req)

		if translator.IsStreamEnabled(req.Stream) {
			// Streaming mode.
			stream, err := client.ChatCompletionStream(r.Context(), openaiReq)
			if err != nil {
				slog.Error("streaming chat failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}
			defer stream.Close()

			if err := translator.TranslateStreamToOllamaNDJSON(
				r.Context(), w, stream, responseModel, translator.StreamModeChat,
			); err != nil {
				slog.Error("stream translation failed", "error", err)
			}
		} else {
			// Non-streaming mode.
			resp, err := client.ChatCompletion(r.Context(), openaiReq)
			if err != nil {
				slog.Error("chat completion failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}

			result := translator.ChatCompletionToChat(resp, responseModel)
			owuiclient.WriteJSON(w, http.StatusOK, result)
		}
	}
}
