package translator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// StreamMode indicates whether we are translating for /api/chat or /api/generate.
type StreamMode int

const (
	// StreamModeChat produces Ollama chat-style NDJSON (with "message" field).
	StreamModeChat StreamMode = iota
	// StreamModeGenerate produces Ollama generate-style NDJSON (with "response" field).
	StreamModeGenerate
)

// sseChunk represents a parsed SSE chunk from the OpenAI stream.
type sseChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []sseChoice    `json:"choices"`
	Usage   *sseChunkUsage `json:"usage,omitempty"`
}

type sseChoice struct {
	Index        int       `json:"index"`
	Delta        sseDelta  `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
}

type sseDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type sseChunkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TranslateSSEToOllamaNDJSON reads an OpenAI SSE response body and writes
// Ollama NDJSON chunks to the ResponseWriter, flushing after each line.
//
// It never buffers the full response in memory. Each SSE event is parsed,
// translated to the corresponding Ollama NDJSON line, written, and flushed
// immediately.
func TranslateSSEToOllamaNDJSON(
	ctx context.Context,
	w http.ResponseWriter,
	body io.Reader,
	model string,
	mode StreamMode,
) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	// Set headers for NDJSON streaming.
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(body)
	// Allow up to 1MB per SSE line (generous for normal usage).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var totalEvalCount int
	var totalPromptEvalCount int

	for scanner.Scan() {
		// Check for client disconnect.
		select {
		case <-ctx.Done():
			slog.Debug("client disconnected, stopping stream translation")
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		// Skip empty lines (SSE uses blank lines as event separators).
		if line == "" {
			continue
		}

		// SSE lines start with "data: ".
		if !strings.HasPrefix(line, "data: ") {
			// Skip non-data lines (e.g., "event:", "id:", "retry:").
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Handle the [DONE] signal.
		if data == "[DONE]" {
			slog.Debug("received SSE [DONE] signal")
			if err := writeFinalChunk(w, flusher, model, mode, totalPromptEvalCount, totalEvalCount); err != nil {
				return fmt.Errorf("writing final chunk: %w", err)
			}
			return nil
		}

		// Parse the SSE JSON chunk.
		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			slog.Warn("failed to parse SSE chunk", "error", err, "data", truncate(data, 200))
			continue
		}

		// Track usage if provided.
		if chunk.Usage != nil {
			totalPromptEvalCount = chunk.Usage.PromptTokens
			totalEvalCount = chunk.Usage.CompletionTokens
		}

		// Extract content from the first choice's delta.
		content := ""
		var finishReason *string
		if len(chunk.Choices) > 0 {
			content = chunk.Choices[0].Delta.Content
			finishReason = chunk.Choices[0].FinishReason
		}

		// If there is a finish reason but no [DONE] yet, we may get another
		// chunk or [DONE] next. Write content if present; we'll write the
		// final done chunk when [DONE] arrives.
		if finishReason != nil && content == "" {
			// Some providers send a chunk with finish_reason but no content.
			// We don't emit anything here; the [DONE] line will trigger the final chunk.
			continue
		}

		// Write the Ollama NDJSON chunk.
		if err := writeStreamChunk(w, flusher, model, mode, content); err != nil {
			return fmt.Errorf("writing stream chunk: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading SSE stream: %w", err)
	}

	// If we reach here without [DONE], write a final chunk anyway.
	slog.Debug("SSE stream ended without [DONE] signal, writing final chunk")
	return writeFinalChunk(w, flusher, model, mode, totalPromptEvalCount, totalEvalCount)
}

// writeStreamChunk writes a single Ollama NDJSON streaming chunk.
func writeStreamChunk(w http.ResponseWriter, flusher http.Flusher, model string, mode StreamMode, content string) error {
	var chunk interface{}
	now := TimeNow()

	switch mode {
	case StreamModeChat:
		chunk = OllamaChatResponse{
			Model:     model,
			CreatedAt: now,
			Message: OllamaMessage{
				Role:    "assistant",
				Content: content,
			},
			Done: false,
		}
	case StreamModeGenerate:
		chunk = OllamaGenerateResponse{
			Model:     model,
			CreatedAt: now,
			Response:  content,
			Done:      false,
		}
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshaling stream chunk: %w", err)
	}

	// Write as NDJSON: one JSON object per line.
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing stream chunk: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}
	flusher.Flush()

	return nil
}

// writeFinalChunk writes the final Ollama NDJSON chunk with done=true.
func writeFinalChunk(w http.ResponseWriter, flusher http.Flusher, model string, mode StreamMode, promptEvalCount, evalCount int) error {
	var chunk interface{}
	now := TimeNow()

	switch mode {
	case StreamModeChat:
		chunk = OllamaChatResponse{
			Model:     model,
			CreatedAt: now,
			Message: OllamaMessage{
				Role:    "assistant",
				Content: "",
			},
			Done:            true,
			DoneReason:      "stop",
			TotalDuration:   0,
			LoadDuration:    0,
			PromptEvalCount: promptEvalCount,
			EvalCount:       evalCount,
			EvalDuration:    0,
		}
	case StreamModeGenerate:
		chunk = OllamaGenerateResponse{
			Model:           model,
			CreatedAt:       now,
			Response:        "",
			Done:            true,
			DoneReason:      "stop",
			Context:         []int{},
			TotalDuration:   0,
			LoadDuration:    0,
			PromptEvalCount: promptEvalCount,
			EvalCount:       evalCount,
			EvalDuration:    0,
		}
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshaling final chunk: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing final chunk: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("writing final newline: %w", err)
	}
	flusher.Flush()

	return nil
}

// truncate returns the first n characters of a string, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
