package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Config struct {
	MaxConcurrentThreads int `json:"max_concurrent_threads"`
	MaxCompletionTokens  int `json:"max_completion_tokens"`
	// RequestTimeout must be passed to the openai.Client outside of this package.
	RequestTimeout Duration `json:"request_timeout"`
}

type Runner struct {
	client  *openai.Client
	cfg     Config
	threads []*Thread
}

type Result struct {
	// Duration of the entire run, waits for all threads to finish (includes stragglers).
	Duration           Duration      `json:"duration"`
	Requests           int           `json:"requests"`
	FailedThreads      int           `json:"failed_threads"`
	TTFT               DurationGroup `json:"ttft"`
	TPOT               DurationGroup `json:"tpot"`
	PromptTokens       int           `json:"prompt_tokens"`
	CachedPromptTokens int           `json:"cached_prompt_tokens"`
	CompletionTokens   int           `json:"completion_tokens"`
	TotalTokens        int           `json:"total_tokens"`
}

func (r Result) String() string {
	return fmt.Sprintf(`=================== Results ===================
       Run Duration: %s
Failed Thread Count: %d
           Requests: %d
      Prompt Tokens: %d (%d cached)
  Completion Tokens: %d
       Total Tokens: %d
        TTFT (mean): %s
        TPOT (mean): %s
===============================================`,
		time.Duration(r.Duration),
		r.FailedThreads,
		r.Requests,
		r.PromptTokens,
		r.CachedPromptTokens,
		r.CompletionTokens,
		r.TotalTokens,
		time.Duration(r.TTFT.Mean),
		time.Duration(r.TPOT.Mean),
	)
}

type DurationGroup struct {
	Mean Duration `json:"mean"`
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
	id          string
	inputMsgs   []openai.ChatCompletionMessage
	currentMsgs []openai.ChatCompletionMessage
	requests    int
	results     []RequestResult
	err         error
}

type InputThread struct {
	ID       string                         `json:"id"`
	Messages []openai.ChatCompletionMessage `json:"messages"`
}

func (c Config) Validate() error {
	if c.MaxConcurrentThreads <= 0 {
		return errors.New("max_concurrent_threads must be greater than 0")
	}
	if c.MaxCompletionTokens <= 0 {
		return errors.New("max_completion_tokens must be greater than 0")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("request_timeout must be greater than 0")
	}
	return nil
}

func New(client *openai.Client, cfg Config, inputThreads []InputThread) *Runner {
	threads := make([]*Thread, len(inputThreads))
	for i := range inputThreads {
		threads[i] = &Thread{
			id:        inputThreads[i].ID,
			inputMsgs: inputThreads[i].Messages,
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
	sem := make(chan struct{}, r.cfg.MaxConcurrentThreads)

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
		ttfts []time.Duration
		tpots []time.Duration
	)

	result := Result{
		Duration: Duration(duration),
	}

	for _, t := range r.threads {
		result.Requests += t.requests
		if t.err != nil {
			// TODO: Should we gather the metrics from any successful chunks?
			result.FailedThreads++
			continue
		}
		for _, reqResult := range t.results {
			for chunkIdx, chunkResult := range reqResult.Chunks {
				result.PromptTokens += chunkResult.PromptTokens
				result.CachedPromptTokens += chunkResult.CachedPromptTokens
				result.CompletionTokens += chunkResult.CompletionTokens
				result.TotalTokens += chunkResult.TotalTokens

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

	result.TTFT = DurationGroup{Mean: Duration(mean(ttfts))}
	result.TPOT = DurationGroup{Mean: Duration(mean(tpots))}

	return result
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

		t.requests++
		stream, err := r.client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:     openai.GPT3Babbage002,
				MaxTokens: r.cfg.MaxCompletionTokens,
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

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	pd, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(pd)

	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}
