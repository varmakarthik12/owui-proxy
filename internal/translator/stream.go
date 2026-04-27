package translator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	ollamaapi "github.com/ollama/ollama/api"
	openai "github.com/sashabaranov/go-openai"
)

// StreamMode indicates whether we are translating for /api/chat or /api/generate.
type StreamMode int

const (
	// StreamModeChat produces Ollama chat-style NDJSON (with "message" field).
	StreamModeChat StreamMode = iota
	// StreamModeGenerate produces Ollama generate-style NDJSON (with "response" field).
	StreamModeGenerate
)

// TranslateStreamToOllamaNDJSON reads an OpenAI streaming response via the go-openai SDK
// and writes Ollama NDJSON chunks to the ResponseWriter, flushing after each line.
func TranslateStreamToOllamaNDJSON(
	ctx context.Context,
	w http.ResponseWriter,
	stream *openai.ChatCompletionStream,
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

	var content strings.Builder
	var thinking strings.Builder
	var toolCallAcc []openai.ToolCall
	var promptTokens, completionTokens int

	for {
		// Check for client disconnect.
		select {
		case <-ctx.Done():
			slog.Debug("client disconnected, stopping stream translation")
			return ctx.Err()
		default:
		}

		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Stream ended normally — emit final chunk.
			slog.Debug("stream ended (EOF)")
			return writeFinalChunk(w, flusher, model, mode, "stop", content.String(), thinking.String(), nil, promptTokens, completionTokens)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("receiving stream chunk: %w", err)
		}

		// Track usage if provided.
		if response.Usage != nil {
			promptTokens = response.Usage.PromptTokens
			completionTokens = response.Usage.CompletionTokens
		}

		if len(response.Choices) == 0 {
			continue
		}

		choice := response.Choices[0]
		delta := choice.Delta

		// a) Reasoning/thinking content.
		if delta.ReasoningContent != "" {
			thinking.WriteString(delta.ReasoningContent)
			if mode == StreamModeChat {
				if err := writeNDJSON(w, flusher, ollamaapi.ChatResponse{
					Model:     model,
					CreatedAt: time.Now().UTC(),
					Message: ollamaapi.Message{
						Role:     "assistant",
						Thinking: thinking.String(),
					},
					Done: false,
				}); err != nil {
					return err
				}
			}
		}

		// b) Content.
		if delta.Content != "" {
			content.WriteString(delta.Content)
			if err := writeStreamContentChunk(w, flusher, model, mode, delta.Content); err != nil {
				return err
			}
		}

		// c) Tool calls — accumulate across partial fragments.
		if len(delta.ToolCalls) > 0 {
			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				// Grow the accumulator if needed.
				for idx >= len(toolCallAcc) {
					toolCallAcc = append(toolCallAcc, openai.ToolCall{})
				}
				if tc.ID != "" {
					toolCallAcc[idx].ID += tc.ID
				}
				if tc.Type != "" {
					toolCallAcc[idx].Type = tc.Type
				}
				if tc.Function.Name != "" {
					toolCallAcc[idx].Function.Name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCallAcc[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}

		// d) Check finish reason.
		if choice.FinishReason == "tool_calls" {
			// All tool call arguments are fully accumulated.
			ollamaToolCalls := mapOpenAIToolCallsToOllama(toolCallAcc)
			return writeFinalChunk(w, flusher, model, mode, "tool_calls", content.String(), thinking.String(), ollamaToolCalls, promptTokens, completionTokens)
		}
	}
}

// writeStreamContentChunk writes a single content chunk in the appropriate mode.
func writeStreamContentChunk(w http.ResponseWriter, flusher http.Flusher, model string, mode StreamMode, content string) error {
	switch mode {
	case StreamModeChat:
		return writeNDJSON(w, flusher, ollamaapi.ChatResponse{
			Model:     model,
			CreatedAt: time.Now().UTC(),
			Message: ollamaapi.Message{
				Role:    "assistant",
				Content: content,
			},
			Done: false,
		})
	case StreamModeGenerate:
		return writeNDJSON(w, flusher, ollamaapi.GenerateResponse{
			Model:     model,
			CreatedAt: time.Now().UTC(),
			Response:  content,
			Done:      false,
		})
	}
	return nil
}

// writeFinalChunk writes the final NDJSON chunk with done=true.
func writeFinalChunk(
	w http.ResponseWriter,
	flusher http.Flusher,
	model string,
	mode StreamMode,
	doneReason string,
	content string,
	thinkingContent string,
	toolCalls []ollamaapi.ToolCall,
	promptTokens, completionTokens int,
) error {
	switch mode {
	case StreamModeChat:
		msg := ollamaapi.Message{
			Role:    "assistant",
			Content: content,
		}
		if thinkingContent != "" {
			msg.Thinking = thinkingContent
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		return writeNDJSON(w, flusher, ollamaapi.ChatResponse{
			Model:      model,
			CreatedAt:  time.Now().UTC(),
			Message:    msg,
			Done:       true,
			DoneReason: doneReason,
			Metrics: ollamaapi.Metrics{
				PromptEvalCount: promptTokens,
				EvalCount:       completionTokens,
			},
		})
	case StreamModeGenerate:
		return writeNDJSON(w, flusher, ollamaapi.GenerateResponse{
			Model:      model,
			CreatedAt:  time.Now().UTC(),
			Response:   "",
			Done:       true,
			DoneReason: doneReason,
			Metrics: ollamaapi.Metrics{
				PromptEvalCount: promptTokens,
				EvalCount:       completionTokens,
			},
		})
	}
	return nil
}

// writeNDJSON marshals v to JSON, writes it as a single line, and flushes.
func writeNDJSON(w http.ResponseWriter, flusher http.Flusher, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling NDJSON chunk: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("writing NDJSON chunk: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}
	flusher.Flush()
	return nil
}
