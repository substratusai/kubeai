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
	// Sanity check...
	require.Equal(t, 3, testInputThreadUserMessageCount())

	runnerCfg := benchmark.Config{
		MaxConcurrentThreads: 1,
		MaxCompletionTokens:  1024,
		RequestTimeout:       benchmark.Duration(30 * time.Second),
	}

	require.NoError(t, runnerCfg.Validate())

	testServer := httptest.NewServer(http.HandlerFunc(mockChatCompletionsHandler))
	openaiCfg := openai.DefaultConfig("test-api-key")
	openaiCfg.BaseURL = testServer.URL + "/v1"
	openaiCfg.HTTPClient = &http.Client{Timeout: time.Duration(runnerCfg.RequestTimeout)}
	client := openai.NewClientWithConfig(openaiCfg)

	runner := benchmark.New(client, runnerCfg, inputThreads)
	result, err := runner.Run()
	require.NoError(t, err)

	fmt.Println(result.String())

	require.Equal(t, testInputThreadUserMessageCount(), result.Requests)
	require.Equal(t, 0, result.FailedThreads)

	require.Equal(t, testInputThreadUserMessageCount()*testNumOfChunksPerRequest*testPromptTokensPerChunk, result.PromptTokens)
	require.Equal(t, testInputThreadUserMessageCount()*testNumOfChunksPerRequest*testCachedPromptTokensPerChunk, result.CachedPromptTokens)
	require.Equal(t, testInputThreadUserMessageCount()*testNumOfChunksPerRequest*testCompletionTokensPerChunk, result.CompletionTokens)
	require.Equal(t, testInputThreadUserMessageCount()*testNumOfChunksPerRequest*(testPromptTokensPerChunk+testCompletionTokensPerChunk), result.TotalTokens)

	requireRoughlyEqualTo(t, testTimeBetweenChunks/time.Duration(testCompletionTokensPerChunk), time.Duration(result.TPOT.Mean), 1*time.Millisecond)
	requireRoughlyEqualTo(t, testTimeBeforeChunk0, time.Duration(result.TTFT.Mean), 10*time.Millisecond)
	requireRoughlyEqualTo(t,
		time.Duration(testInputThreadUserMessageCount())*(testTimeBeforeChunk0+((testNumOfChunksPerRequest-1)*testTimeBetweenChunks)),
		time.Duration(result.Duration), 100*time.Millisecond)

}

func requireRoughlyEqualTo(t *testing.T, want, actual, threshold time.Duration) {
	require.Greater(t, actual, want-threshold)
	require.Less(t, actual, want+threshold)
}

func testInputThreadUserMessageCount() int {
	var n int
	for _, thread := range inputThreads {
		for _, message := range thread.Messages {
			if message.Role == openai.ChatMessageRoleUser {
				n++
			}
		}
	}
	return n
}

var inputThreads = []benchmark.InputThread{
	{
		ID: "a",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello, how are you?",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "I'm just a computer program, so I don't have feelings, but I'm here to help!",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Come again?",
			},
		},
	},
	{
		ID: "b",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "What's your favorite color?",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "My favorite color is blue!",
			},
		},
	},
}

const (
	testTimeBeforeChunk0           = 3 * time.Second
	testTimeBetweenChunks          = time.Second
	testPromptTokensPerChunk       = 5
	testCachedPromptTokensPerChunk = 2
	testCompletionTokensPerChunk   = 10
	testNumOfChunksPerRequest      = 3
)

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

	time.Sleep(testTimeBeforeChunk0)

	for i := 0; i < testNumOfChunksPerRequest; i++ {
		if i != 0 {
			time.Sleep(testTimeBetweenChunks)
		}

		streamResp := openai.ChatCompletionStreamResponse{
			Usage: &openai.Usage{
				CompletionTokens: testCompletionTokensPerChunk,
				PromptTokens:     testPromptTokensPerChunk,
				TotalTokens:      testPromptTokensPerChunk + testCompletionTokensPerChunk,
				PromptTokensDetails: &openai.PromptTokensDetails{
					CachedTokens: testCachedPromptTokensPerChunk,
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
