package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
	"github.com/varmakarthik12/owui-proxy/internal/translator"
)

// Generate handles POST /api/generate — streaming and non-streaming text generation.
func Generate(client *owuiclient.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		var req translator.OllamaGenerateRequest
		if err := owuiclient.ReadJSON(r.Body, &req); err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		slog.Info("generate request", "model", req.Model)

		// Translate to OpenAI format.
		openaiReq, err := translator.GenerateToOpenAI(&req)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			owuiclient.WriteError(w, http.StatusInternalServerError, "failed to marshal request")
			return
		}

		slog.Debug("translated generate request", "request_body_bytes", len(reqBody))

		if translator.IsStreamEnabled(req.Stream) {
			// Streaming mode.
			resp, err := client.ChatCompletionStream(r.Context(), bytes.NewReader(reqBody))
			if err != nil {
				slog.Error("streaming generate failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}
			defer resp.Body.Close()

			if err := translator.TranslateSSEToOllamaNDJSON(
				r.Context(), w, resp.Body, req.Model, translator.StreamModeGenerate,
			); err != nil {
				slog.Error("stream translation failed", "error", err)
			}
		} else {
			// Non-streaming mode.
			resp, err := client.ChatCompletion(r.Context(), bytes.NewReader(reqBody))
			if err != nil {
				slog.Error("generate completion failed", "error", owuiclient.FormatUpstreamError(err, ""))
				owuiclient.WriteError(w, http.StatusBadGateway,
					"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
				return
			}

			result := translator.ChatCompletionToGenerate(resp, req.Model)
			owuiclient.WriteJSON(w, http.StatusOK, result)
		}
	}
}
