package benchmark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Config struct {
	MaxConcurrency int
	MaxTokens      int
}

type Runner struct {
	client  *openai.Client
	cfg     Config
	threads []*Thread
}

type Result struct {
	// Duration of the entire run, waits for all threads to finish (includes stragglers).
	Duration      time.Duration
	FailedThreads int
	TTFT          DurationGroup
	TPOT          DurationGroup
}

func (r Result) String() string {
	return fmt.Sprintf(`Run Duration: %s
Failed Thread Count: %d
TTFT (mean): %s
TPOT (mean): %s`,
		r.Duration,
		r.FailedThreads,
		r.TTFT.Mean,
		r.TPOT.Mean,
	)
}

type DurationGroup struct {
	Mean time.Duration
	// TODO: p95, etc.
}

type RequestResult struct {
	Duration time.Duration
	Chunks   []ChunkResult
}

type ChunkResult struct {
	Duration           time.Duration
	PromptTokens       int
	CachedPromptTokens int
	CompletionTokens   int
	TotalTokens        int
}

type Thread struct {
	inputMsgs   []openai.ChatCompletionMessage
	currentMsgs []openai.ChatCompletionMessage
	results     []RequestResult
	err         error
}

func New(client *openai.Client, cfg Config, inputThreads [][]openai.ChatCompletionMessage) *Runner {
	threads := make([]*Thread, len(inputThreads))
	for i := range inputThreads {
		threads[i] = &Thread{
			inputMsgs: inputThreads[i],
		}
	}
	return &Runner{
		client:  client,
		cfg:     cfg,
		threads: threads,
	}
}

func (r *Runner) Run() (Result, error) {
	// Run up to cfg.Concurrency threads at a time, until all threads are ran.

	var wg sync.WaitGroup
	sem := make(chan struct{}, r.cfg.MaxConcurrency)

	t0 := time.Now()
	for i, t := range r.threads {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			if err := r.RunThread(t); err != nil {
				log.Printf("Thread %d failed: %v\n", i, err)
				t.err = err
			}
		}()
	}

	wg.Wait()

	duration := time.Since(t0)

	return r.summarizeResults(duration), nil
}

func (r *Runner) summarizeResults(duration time.Duration) Result {
	var (
		ttfts         []time.Duration
		tpots         []time.Duration
		failedThreads int
	)
	for _, t := range r.threads {
		if t.err != nil {
			// TODO: Should we gather the metrics from any successful chunks?
			failedThreads++
			continue
		}
		for _, reqResult := range t.results {
			for chunkIdx, chunkResult := range reqResult.Chunks {
				if chunkIdx == 0 {
					ttfts = append(ttfts, chunkResult.Duration)
				} else {
					for i := 0; i < chunkResult.CompletionTokens; i++ {
						// If 3 tokens are generated in a chunk that took 3 seconds to be returned,
						// count each token's generation time as 1 second.
						tpots = append(tpots, chunkResult.Duration/time.Duration(chunkResult.CompletionTokens))
					}
				}
			}
		}
	}
	return Result{
		Duration:      duration,
		FailedThreads: failedThreads,
		TTFT:          DurationGroup{Mean: mean(ttfts)},
		TPOT:          DurationGroup{Mean: mean(tpots)},
	}
}

func mean(durs []time.Duration) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	sum := time.Duration(0)
	for _, d := range durs {
		sum += d
	}
	return sum / time.Duration(len(durs))
}

func (r *Runner) RunThread(t *Thread) error {
	// Load all messages up but not including the first user message.
	// (Usually this is just a single system message).
	for _, msg := range t.inputMsgs {
		if msg.Role == openai.ChatMessageRoleUser {
			break
		}
		t.currentMsgs = append(t.currentMsgs, msg)
	}

	// Start the multi-turn conversation.
	// Append the next user message.
	// Append the response from the LLM.
	// Iterate until all user messages are processed.
	for _, msg := range t.inputMsgs {
		if msg.Role != openai.ChatMessageRoleUser {
			continue
		}
		t.currentMsgs = append(t.currentMsgs, msg)

		var result RequestResult

		t0 := time.Now()

		stream, err := r.client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:     openai.GPT3Babbage002,
				MaxTokens: r.cfg.MaxTokens,
				Stream:    true,
				Messages:  t.currentMsgs,
				StreamOptions: &openai.StreamOptions{
					IncludeUsage: true,
				},
			},
		)
		if err != nil {
			return fmt.Errorf("request: %v", err)
		}
		defer stream.Close()

		var i int
		tChunk0 := t0
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("stream: %v", err)
			}

			if response.Usage.CompletionTokens > 0 {
				chunkResult := ChunkResult{
					Duration:         time.Since(tChunk0),
					PromptTokens:     response.Usage.PromptTokens,
					CompletionTokens: response.Usage.CompletionTokens,
					TotalTokens:      response.Usage.TotalTokens,
				}
				if response.Usage.PromptTokensDetails != nil {
					chunkResult.CachedPromptTokens = response.Usage.PromptTokensDetails.CachedTokens
				}
				result.Chunks = append(result.Chunks, chunkResult)
			}

			tChunk0 = time.Now()
			i++
		}

		result.Duration = time.Since(t0)
		t.results = append(t.results, result)
	}

	return nil
}
