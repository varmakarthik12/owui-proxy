package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// V1Chat handles POST /v1/chat/completions — OpenAI-compatible chat completion.
// Before forwarding:
//  1. The model prefix (e.g. "owui-proxy/") is stripped from the model field.
//  2. If both "temperature" and "top_p" are set, "top_p" is removed because
//     Claude models reject requests that specify both.
//
// The (modified) request is then forwarded verbatim and the upstream response
// (JSON or SSE) is piped back unchanged.
func V1Chat(client *owuiclient.OwuiClient, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		owuiclient.StripSensitiveHeaders(r.Header)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		body, err = normalizeV1ChatBody(body, cfg.ModelPrefix)
		if err != nil {
			owuiclient.WriteError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		slog.Debug("v1 chat request", "bytes", len(body))

		upstream, err := client.ProxyRequest(r.Context(), http.MethodPost, "/chat/completions", body)
		if err != nil {
			slog.Error("v1 chat upstream error", "error", owuiclient.FormatUpstreamError(err, ""))
			owuiclient.WriteError(w, http.StatusBadGateway,
				"upstream error: "+owuiclient.FormatUpstreamError(err, ""))
			return
		}
		defer upstream.Body.Close()

		// Copy relevant upstream response headers.
		for _, h := range []string{
			"Content-Type",
			"Cache-Control",
			"Transfer-Encoding",
			"X-Request-Id",
		} {
			if v := upstream.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.WriteHeader(upstream.StatusCode)

		// Pipe body — for SSE this streams in real-time.
		flusher, canFlush := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, readErr := upstream.Body.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return
				}
				if canFlush {
					flusher.Flush()
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				slog.Debug("upstream read error", "error", readErr)
				return
			}
		}
	}
}

// normalizeV1ChatBody parses the JSON body, strips the model prefix, and removes
// top_p when temperature is also set (Claude rejects both simultaneously).
func normalizeV1ChatBody(body []byte, prefix string) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	changed := false

	// 1. Strip model prefix.
	if prefix != "" {
		if model, ok := req["model"].(string); ok && strings.HasPrefix(model, prefix) {
			req["model"] = strings.TrimPrefix(model, prefix)
			changed = true
			slog.Debug("v1 chat stripped model prefix", "prefix", prefix, "model", req["model"])
		}
	}

	// 2. Remove top_p when temperature is also present — Claude disallows both.
	_, hasTemp := req["temperature"]
	_, hasTopP := req["top_p"]
	if hasTemp && hasTopP {
		delete(req, "top_p")
		changed = true
		slog.Debug("v1 chat removed top_p (temperature already set)")
	}

	if !changed {
		return body, nil
	}

	return json.Marshal(req)
}
