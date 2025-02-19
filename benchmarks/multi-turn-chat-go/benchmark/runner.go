package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"multi-turn-chat-go/tokenizer"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Config struct {
	MaxConcurrentThreads int    `json:"max_concurrent_threads"`
	MaxCompletionTokens  int    `json:"max_completion_tokens"`
	RequestModel         string `json:"request_model"`
}

type Runner struct {
	client    *openai.Client
	tokenizer *tokenizer.Tokenizer
	cfg       Config
	threads   []*Thread
}

type Result struct {
	// Duration of the entire run, waits for all threads to finish (includes stragglers).
	Duration      Duration `json:"duration"`
	Requests      int      `json:"requests"`
	FailedThreads int      `json:"failed_threads"`
	// RunTotalThroughput of entire run, measured in requests per second.
	// This includes ramp-up and ramp-down timeframes.
	// TODO: Add another throughput measurement for steady-state.
	RunOutputThroughput float64       `json:"run_output_throughput"`
	RunTotalThroughput  float64       `json:"run_total_throughput"`
	TTFT                DurationGroup `json:"ttft"`
	TPOT                DurationGroup `json:"tpot"`
	PromptTokens        int           `json:"prompt_tokens"`
	CachedPromptTokens  int           `json:"cached_prompt_tokens"`
	CompletionTokens    int           `json:"completion_tokens"`
	TotalTokens         int           `json:"total_tokens"`
}

func (r Result) String() string {
	return fmt.Sprintf(`====================== Results ======================
                   Duration: %s
        Failed Thread Count: %d
                   Requests: %d
              Prompt Tokens: %d (%d cached)
          Completion Tokens: %d
               Total Tokens: %d
Output Throughput (e2e run): %f tok/sec
 Total Throughput (e2e run): %f tok/sec
                TTFT (mean): %s
                TPOT (mean): %s
=====================================================`,
		time.Duration(r.Duration),
		r.FailedThreads,
		r.Requests,
		r.PromptTokens,
		r.CachedPromptTokens,
		r.CompletionTokens,
		r.TotalTokens,
		r.RunOutputThroughput,
		r.RunTotalThroughput,
		time.Duration(r.TTFT.Mean),
		time.Duration(r.TPOT.Mean),
	)
}

type DurationGroup struct {
	Mean Duration `json:"mean"`
	// TODO: p95, etc.
}

type RequestResult struct {
	Duration           time.Duration
	Chunks             []ChunkResult
	PromptTokens       int
	CachedPromptTokens int
	CompletionTokens   int
	TotalTokens        int
}

type ChunkResult struct {
	Text     string
	Duration time.Duration
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
		return errors.New("max_concurrent_threads (--max-concurrent-threads) must be greater than 0")
	}
	if c.MaxCompletionTokens <= 0 {
		return errors.New("max_completion_tokens (--max-completion-tokens) must be greater than 0")
	}
	if c.RequestModel == "" {
		return errors.New("request_model (--request-model) must be specified")
	}
	return nil
}

func New(client *openai.Client, tokenizer *tokenizer.Tokenizer, cfg Config, inputThreads []InputThread) *Runner {
	threads := make([]*Thread, len(inputThreads))
	for i := range inputThreads {
		threads[i] = &Thread{
			id:        inputThreads[i].ID,
			inputMsgs: inputThreads[i].Messages,
		}
	}
	return &Runner{
		client:    client,
		tokenizer: tokenizer,
		cfg:       cfg,
		threads:   threads,
	}
}

func (r *Runner) Run() (Result, error) {
	log.Println("Starting run...")
	// Run up to cfg.Concurrency threads at a time, until all threads are ran.

	var wg sync.WaitGroup
	sem := make(chan struct{}, r.cfg.MaxConcurrentThreads)

	t0 := time.Now()
	tLen := len(r.threads)
	for i, t := range r.threads {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			if err := r.RunThread(t); err != nil {
				log.Printf("Thread[%d/%d]: Failed: %v\n", i+1, tLen, err)
				t.err = err
			} else {
				log.Printf("Thread[%d/%d]: Finished", i+1, tLen)
			}
		}()
	}

	wg.Wait()

	duration := time.Since(t0)

	log.Printf("Run completed after %s, starting summarization...", duration)
	return r.summarizeResults(duration)
}

func (r *Runner) summarizeResults(duration time.Duration) (Result, error) {
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
			//var sumReqCompletionTokens int
			//var sumText string
			for chunkIdx, chunkResult := range reqResult.Chunks {

				//completionTokens, err := r.tokenizer.CountTokens(chunkResult.Text)
				//if err != nil {
				//	return Result{}, fmt.Errorf("countNumTokens (thread index %d, request index %d, chunk index %d): %w", tIdx, reqIdx, chunkIdx, err)
				//}
				//sumReqCompletionTokens += completionTokens
				//sumText += chunkResult.Text

				if chunkIdx == 0 {
					ttfts = append(ttfts, chunkResult.Duration)
				} //else {
				//for i := 0; i < completionTokens; i++ {
				//	// If 3 tokens are generated in a chunk that took 3 seconds to be returned,
				//	// count each token's generation time as 1 second.
				//	tpots = append(tpots, chunkResult.Duration/time.Duration(completionTokens))
				//}
				//}
			}
			//if sumReqCompletionTokens != reqResult.CompletionTokens {
			//	log.Fatalf("FATAL: Calculated completion token count (%d) does not match usage report (%d): %q - tokenizer model likely does not match request model",
			//		sumReqCompletionTokens, reqResult.CompletionTokens, sumText)
			//}
			result.PromptTokens += reqResult.PromptTokens
			result.CachedPromptTokens += reqResult.CachedPromptTokens
			result.CompletionTokens += reqResult.CompletionTokens
			result.TotalTokens += reqResult.TotalTokens
			tpots = append(tpots, reqResult.Duration/time.Duration(reqResult.CompletionTokens))
		}
	}

	result.TTFT = DurationGroup{Mean: Duration(mean(ttfts))}
	result.TPOT = DurationGroup{Mean: Duration(mean(tpots))}

	result.RunOutputThroughput = float64(result.CompletionTokens) / float64(time.Duration(result.Duration).Seconds())
	result.RunTotalThroughput = float64(result.TotalTokens) / float64(time.Duration(result.Duration).Seconds())

	return result, nil
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
				Model:     r.cfg.RequestModel,
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

			if len(response.Choices) > 0 {
				chunkResult := ChunkResult{
					Duration: time.Since(tChunk0),
					Text:     response.Choices[0].Delta.Content,
				}
				result.Chunks = append(result.Chunks, chunkResult)
			}

			if response.Usage != nil {
				if result.TotalTokens != 0 {
					panic("observed multiple usage reports for a single request - expected one in the final server-sent-event")
				}
				result.PromptTokens = response.Usage.PromptTokens
				result.CompletionTokens = response.Usage.CompletionTokens
				result.TotalTokens = response.Usage.TotalTokens
				if response.Usage.PromptTokensDetails != nil {
					result.CachedPromptTokens = response.Usage.PromptTokensDetails.CachedTokens
				}
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
