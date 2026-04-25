package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Chat handles POST /api/chat — streaming and non-streaming chat completion.
func Chat(client *owuiclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.OllamaChatRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		slog.Info("chat request", "model", req.Model, "messages", len(req.Messages))

		// Translate to OpenAI format.
		openaiReq, err := translator.ChatToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			owuiclient.WriteError(w, http.StatusInternalServerError, "failed to marshal request")
			return
		}

		slog.Debug("translated chat request", "request_body_bytes", len(reqBody))

		if translator.IsStreamEnabled(req.Stream) {
			// Streaming mode.
			resp, err := client.ChatCompletionStream(r.Context(), bytes.NewReader(reqBody))
			if err != nil {
				slog.Error("streaming chat failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}
			defer resp.Body.Close()

			if err := translator.TranslateSSEToOllamaNDJSON(
				r.Context(), w, resp.Body, req.Model, translator.StreamModeChat,
			); err != nil {
				slog.Error("stream translation failed", "error", err)
				// Can't write error response — headers already sent.
			}
		} else {
			// Non-streaming mode.
			resp, err := client.ChatCompletion(r.Context(), bytes.NewReader(reqBody))
			if err != nil {
				slog.Error("chat completion failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}

			result := translator.ChatCompletionToChat(resp, req.Model)
			owuiclient.WriteJSON(w, http.StatusOK, result)
		}
	}
}
