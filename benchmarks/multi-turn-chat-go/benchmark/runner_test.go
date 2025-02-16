package benchmark_test

import (
	"encoding/json"
	"fmt"
	"log"
	"multi-turn-chat-go/benchmark"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

func TestRunner(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(mockChatCompletionsHandler))
	openaiCfg := openai.DefaultConfig("test-api-key")
	openaiCfg.BaseURL = testServer.URL + "/v1"
	openaiCfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	client := openai.NewClientWithConfig(openaiCfg)

	runnerCfg := benchmark.Config{
		MaxConcurrency: 1,
		MaxTokens:      1024,
	}
	runner := benchmark.New(client, runnerCfg, inputThreads)
	result, err := runner.Run()
	require.NoError(t, err)

	fmt.Println(result.String())
}

var inputThreads = [][]openai.ChatCompletionMessage{
	{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Hello, how are you?",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "I'm just a computer program, so I don't have feelings, but I'm here to help!",
		},
	},
	{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "What's your favorite color?",
		},
		{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "My favorite color is blue!",
		},
	},
}

func mockChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body into ChatCompletionRequest
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Failed to parse request", http.StatusBadRequest)
		return
	}

	// Ensure streaming is enabled in the request (for demonstration purposes)
	if !req.Stream {
		http.Error(w, "Request does not have streaming enabled", http.StatusBadRequest)
		return
	}

	// Set headers for SSE (Server-Sent Events)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	// Make sure our writer supports flushing for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream chunks.

	time.Sleep(2 * time.Second)

	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)

		streamResp := openai.ChatCompletionStreamResponse{
			Usage: &openai.Usage{
				CompletionTokens: 10,
				PromptTokens:     5,
				TotalTokens:      15,
				PromptTokensDetails: &openai.PromptTokensDetails{
					CachedTokens: 3,
				},
			},
		}

		// Encode response to JSON
		respData, err := json.Marshal(streamResp)
		if err != nil {
			log.Printf("Error marshalling stream response: %v", err)
			continue
		}

		// Write SSE event
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(respData)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}

	// Send the SSE terminator
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}
