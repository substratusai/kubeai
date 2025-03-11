package benchmark_test

import (
	"encoding/json"
	"fmt"
	"log"
	"multi-turn-chat-go/benchmark"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

const testTemp = 1.1

func TestRunner(t *testing.T) {
	// Sanity check...
	require.Equal(t, 3, testInputThreadMessageCount())

	runnerCfg := benchmark.Config{
		RequestModel:         "test-model",
		MaxConcurrentThreads: 1,
		MaxCompletionTokens:  1024,
		Temperature:          testTemp,
	}

	require.NoError(t, runnerCfg.Validate())

	requestTimeout := 30 * time.Second

	testServer := httptest.NewServer(http.HandlerFunc(mockChatCompletionsHandler))
	defer testServer.Close()
	openaiCfg := openai.DefaultConfig("test-api-key")
	openaiCfg.BaseURL = testServer.URL + "/v1"
	httpc := &http.Client{Timeout: requestTimeout}
	openaiCfg.HTTPClient = httpc
	client := openai.NewClientWithConfig(openaiCfg)

	runner := benchmark.New(client, runnerCfg, inputThreads)
	result, err := runner.Run()
	require.NoError(t, err)

	fmt.Println(result.String())

	require.Equal(t, testInputThreadMessageCount(), result.RequestCount)
	require.Equal(t, 0, result.FailedThreads)

	require.Equal(t, len(inputThreads), result.InputThreadCount)
	require.Equal(t, float64(testInputThreadMessageCount())/float64(len(inputThreads)), result.InputMessagesPerThread.Mean)

	require.Equal(t, testInputThreadMessageCount()*testPromptTokensPerRequest, result.PromptTokens)
	require.Equal(t, testInputThreadMessageCount()*testCachedPromptTokensPerRequest, result.CachedPromptTokens)
	require.Equal(t, testInputThreadMessageCount()*testNumOfChunksPerRequest*testCompletionTokensPerChunk, result.CompletionTokens)
	require.Equal(t, testInputThreadMessageCount()*((testNumOfChunksPerRequest*testCompletionTokensPerChunk)+testPromptTokensPerRequest), result.TotalTokens)
	require.EqualValues(t, testNumOfChunksPerRequest, result.ChunksPerRequest.Mean)

	requireRoughlyEqualTo(t, testTimeBetweenChunks/time.Duration(testCompletionTokensPerChunk), time.Duration(result.ITL.Mean), 10*time.Millisecond)
	requireRoughlyEqualTo(t, testTimeBeforeChunk0, time.Duration(result.TTFT.Mean), 10*time.Millisecond)
	requireRoughlyEqualTo(t,
		time.Duration(testInputThreadMessageCount())*(testTimeBeforeChunk0+((testNumOfChunksPerRequest-1)*testTimeBetweenChunks)),
		time.Duration(result.Duration), 100*time.Millisecond)

}

func requireRoughlyEqualTo(t *testing.T, want, actual, threshold time.Duration) {
	require.Greater(t, actual, want-threshold)
	require.Less(t, actual, want+threshold)
}

func testInputThreadMessageCount() int {
	var n int
	for _, thread := range inputThreads {
		n += len(thread.Messages)
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
				Role:    openai.ChatMessageRoleUser,
				Content: "Are you sure?",
			},
		},
	},
}

const (
	testChunkText                    = "test chunk text"
	testTimeBeforeChunk0             = 1 * time.Second
	testTimeBetweenChunks            = time.Second / 10
	testPromptTokensPerRequest       = 5
	testCachedPromptTokensPerRequest = 2

	testCompletionTokensPerChunk = 10
	testNumOfChunksPerRequest    = 3
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
	json.NewEncoder(os.Stdout).Encode(req)
	if req.Temperature != testTemp {
		log.Fatalf("Temperature mismatch: %v (expected %v)", req.Temperature, testTemp)
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

	for i := 0; i < testNumOfChunksPerRequest+1; i++ {
		if i != 0 && i != testNumOfChunksPerRequest {
			// Avoid adding the extra delay on the first and last (usage) chunk.
			time.Sleep(testTimeBetweenChunks)
		}

		var streamResp openai.ChatCompletionStreamResponse
		if i == testNumOfChunksPerRequest {
			streamResp = openai.ChatCompletionStreamResponse{
				Usage: &openai.Usage{
					CompletionTokens: testNumOfChunksPerRequest * testCompletionTokensPerChunk,
					PromptTokens:     testPromptTokensPerRequest,
					TotalTokens:      testPromptTokensPerRequest + testNumOfChunksPerRequest*testCompletionTokensPerChunk,
					PromptTokensDetails: &openai.PromptTokensDetails{
						CachedTokens: testCachedPromptTokensPerRequest,
					},
				},
			}
		} else {
			streamResp = openai.ChatCompletionStreamResponse{
				Choices: []openai.ChatCompletionStreamChoice{
					{
						Delta: openai.ChatCompletionStreamChoiceDelta{
							Role:    openai.ChatMessageRoleAssistant,
							Content: testChunkText,
						},
					},
				},
			}
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
