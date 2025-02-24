package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

type Config struct {
	MaxConcurrentThreads int     `json:"max_concurrent_threads"`
	MaxCompletionTokens  int     `json:"max_completion_tokens"`
	Temperature          float32 `json:"temperature"`
	RequestModel         string  `json:"request_model"`
	Verbose              bool    `json:"verbose"`
}

type InputThread struct {
	ID       string                         `json:"id"`
	Messages []openai.ChatCompletionMessage `json:"messages"`
}

type Runner struct {
	client  *openai.Client
	cfg     Config
	threads []*thread
}

type Result struct {
	InputThreadCount       int        `json:"input_thread_count"`
	InputMessagesPerThread FloatGroup `json:"input_messages_per_thread"`

	// Duration of the entire run, waits for all threads to finish (includes stragglers).
	Duration         Duration      `json:"duration"`
	RequestCount     int           `json:"request_count"`
	RequestDuration  DurationGroup `json:"request_duration"`
	ChunksPerRequest FloatGroup    `json:"chunks_per_request"`
	FailedThreads    int           `json:"failed_threads"`
	// RunTotalThroughput of entire run, measured in requests per second.
	// This includes ramp-up and ramp-down timeframes.
	// TODO: Add another throughput measurement for steady-state.
	RunOutputThroughput float64       `json:"run_output_throughput"`
	RunTotalThroughput  float64       `json:"run_total_throughput"`
	TTFT                DurationGroup `json:"ttft"`
	ITL                 DurationGroup `json:"itl"`
	PromptTokens        int           `json:"prompt_tokens"`
	CachedPromptTokens  int           `json:"cached_prompt_tokens"`
	CompletionTokens    int           `json:"completion_tokens"`
	TotalTokens         int           `json:"total_tokens"`
}

func (r Result) String() string {
	return fmt.Sprintf(`======================= Input =======================
         Input thread count: %d
   Input msgs/thread (mean): %.2f
====================== Results ======================
                   Duration: %s
        Failed thread count: %d
              Request count: %d
    Request duration (mean): %s
  Chunks per request (mean): %.2f
              Prompt tokens: %d (%d cached)
          Completion tokens: %d
               Total tokens: %d
Output throughput (e2e run): %.2f tok/sec
 Total throughput (e2e run): %.2f tok/sec
                TTFT (mean): %s
                 ITL (mean): %s
=====================================================`,
		// Input //
		r.InputThreadCount,
		r.InputMessagesPerThread.Mean,
		// Results //
		time.Duration(r.Duration).Truncate(time.Second/10),
		r.FailedThreads,
		r.RequestCount,
		time.Duration(r.RequestDuration.Mean).Truncate(time.Millisecond/100),
		r.ChunksPerRequest.Mean,
		r.PromptTokens,
		r.CachedPromptTokens,
		r.CompletionTokens,
		r.TotalTokens,
		r.RunOutputThroughput,
		r.RunTotalThroughput,
		time.Duration(r.TTFT.Mean).Truncate(time.Millisecond/100),
		time.Duration(r.ITL.Mean).Truncate(time.Millisecond/100),
	)
}

type DurationGroup struct {
	Mean Duration `json:"mean"`
	// TODO: p95, etc.
}

type FloatGroup struct {
	Mean float64 `json:"mean"`
	// TODO: p95, etc.
}

type requestResult struct {
	duration           time.Duration
	ttChunks           []time.Duration
	promptTokens       int
	cachedPromptTokens int
	completionTokens   int
	totalTokens        int
}

type thread struct {
	id          string
	inputMsgs   []openai.ChatCompletionMessage
	currentMsgs []openai.ChatCompletionMessage
	requests    int
	results     []requestResult
	err         error
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

func New(client *openai.Client, cfg Config, inputThreads []InputThread) *Runner {
	threads := make([]*thread, len(inputThreads))
	for i := range inputThreads {
		threads[i] = &thread{
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
				if r.cfg.Verbose {
					log.Printf("Thread[%d/%d]: Finished", i+1, tLen)
				}
			}
		}()
	}

	wg.Wait()

	duration := time.Since(t0)

	log.Println("Run completed, starting summarization...")
	return r.summarizeResults(duration)
}

func (r *Runner) summarizeResults(duration time.Duration) (Result, error) {
	var (
		ttfts        []time.Duration
		reqDurations []time.Duration
	)

	result := Result{
		Duration: Duration(duration),
	}

	var (
		totalChunks              int
		totalITLTime             time.Duration
		totalITLCompletionTokens float64
		totalInputMessageCount   int
	)
	for _, t := range r.threads {
		result.RequestCount += t.requests
		totalInputMessageCount += len(t.inputMsgs)
		if t.err != nil {
			// TODO: Should we gather the metrics from any successful chunks?
			result.FailedThreads++
			continue
		}
		for _, reqResult := range t.results {
			var itlChunkCount int
			for chunkIdx, ttChunk := range reqResult.ttChunks {
				totalChunks++
				if chunkIdx == 0 {
					ttfts = append(ttfts, ttChunk)
				} else {
					totalITLTime += ttChunk
					itlChunkCount++
				}
			}
			result.PromptTokens += reqResult.promptTokens
			result.CachedPromptTokens += reqResult.cachedPromptTokens
			result.CompletionTokens += reqResult.completionTokens
			result.TotalTokens += reqResult.totalTokens
			reqDurations = append(reqDurations, reqResult.duration)

			// Distribution of tokens returned by the LLM in each chunk
			// is assumed to be roughly equal. We compute a factor that represents
			// the percentage of completion tokens that should be considered in
			// the ITL calculation.
			itlFactor := float64(itlChunkCount) / float64(itlChunkCount+1)
			totalITLCompletionTokens += itlFactor * float64(reqResult.completionTokens)
		}
	}

	result.TTFT = DurationGroup{Mean: Duration(mean(ttfts))}
	result.ITL = DurationGroup{Mean: Duration(float64(totalITLTime) / totalITLCompletionTokens)}
	result.RequestDuration = DurationGroup{Mean: Duration(mean(reqDurations))}
	result.ChunksPerRequest.Mean = float64(totalChunks) / float64(result.RequestCount)
	result.InputThreadCount = len(r.threads)
	result.InputMessagesPerThread.Mean = float64(totalInputMessageCount) / float64(result.InputThreadCount)

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

func (r *Runner) RunThread(t *thread) error {
	// Load all messages up but not including the first user message.
	// (Usually this is just a single system message).
	for _, msg := range t.inputMsgs {
		if msg.Role == openai.ChatMessageRoleUser {
			break
		}
		t.currentMsgs = append(t.currentMsgs, msg)
	}

	// Start the multi-turn conversation.
	// Append the next input message.
	// Append the response from the LLM.
	// Iterate until all user messages are processed.
	for _, msg := range t.inputMsgs {
		t.currentMsgs = append(t.currentMsgs, msg)

		var result requestResult

		t0 := time.Now()

		t.requests++
		if r.cfg.Verbose {
			json.NewEncoder(os.Stderr).Encode(t.currentMsgs)
		}
		stream, err := r.client.CreateChatCompletionStream(
			context.Background(),
			openai.ChatCompletionRequest{
				Model:       r.cfg.RequestModel,
				MaxTokens:   r.cfg.MaxCompletionTokens,
				Stream:      true,
				Messages:    t.currentMsgs,
				Temperature: r.cfg.Temperature,
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
		var responseText string
		var responseRole string
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("stream: %v", err)
			}

			if len(response.Choices) > 0 &&
				(response.Choices[0].FinishReason == openai.FinishReasonNull ||
					response.Choices[0].FinishReason == "") {
				//fmt.Printf("Response chunk[%d]: %q - %v\n", i, response.Choices[0].Delta.Content, response.Usage)
				result.ttChunks = append(result.ttChunks, time.Since(tChunk0))
				responseText += response.Choices[0].Delta.Content
				if responseRole == "" {
					responseRole = response.Choices[0].Delta.Role
				}
			}

			if response.Usage != nil {
				result.promptTokens = response.Usage.PromptTokens
				result.completionTokens = response.Usage.CompletionTokens
				result.totalTokens = response.Usage.TotalTokens
				if response.Usage.PromptTokensDetails != nil {
					result.cachedPromptTokens = response.Usage.PromptTokensDetails.CachedTokens
				}
			}

			tChunk0 = time.Now()
			i++
		}
		t.currentMsgs = append(t.currentMsgs, openai.ChatCompletionMessage{
			Role:    responseRole,
			Content: responseText,
		})

		result.duration = time.Since(t0)
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
