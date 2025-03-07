package v1

import easyjson "github.com/mailru/easyjson"

// CompletionRequest represents a request structure for completion API.
type CompletionRequest struct {
	Model            string  `json:"model"`
	Prompt           any     `json:"prompt,omitempty"`
	BestOf           int     `json:"best_of,omitempty"`
	Echo             bool    `json:"echo,omitempty"`
	FrequencyPenalty float32 `json:"frequency_penalty,omitempty"`
	// LogitBias is must be a token id string (specified by their token ID in the tokenizer), not a word string.
	// incorrect: `"logit_bias":{"You": 6}`, correct: `"logit_bias":{"1639": 6}`
	// refs: https://platform.openai.com/docs/api-reference/completions/create#completions/create-logit_bias
	LogitBias map[string]int `json:"logit_bias,omitempty"`
	// Store can be set to true to store the output of this completion request for use in distillations and evals.
	// https://platform.openai.com/docs/api-reference/chat/create#chat-create-store
	Store bool `json:"store,omitempty"`
	// Metadata to store with the completion.
	Metadata        map[string]string `json:"metadata,omitempty"`
	LogProbs        int               `json:"logprobs,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	N               int               `json:"n,omitempty"`
	PresencePenalty float32           `json:"presence_penalty,omitempty"`
	Seed            *int              `json:"seed,omitempty"`
	Stop            []string          `json:"stop,omitempty"`
	Stream          bool              `json:"stream,omitempty"`
	Suffix          string            `json:"suffix,omitempty"`
	Temperature     float32           `json:"temperature,omitempty"`
	TopP            float32           `json:"top_p,omitempty"`
	User            string            `json:"user,omitempty"`

	// Preserve unknown fields to fully support the extended set of fields that backends such as vLLM support.
	easyjson.UnknownFieldsProxy
}

func (r *CompletionRequest) GetModel() string {
	return r.Model
}

func (r *CompletionRequest) SetModel(m string) {
	r.Model = m
}

func (r *CompletionRequest) Prefix(n int) string {
	return firstNChars(r.prompt0(), n)
}

func (r *CompletionRequest) prompt0() string {
	if p, ok := r.Prompt.(string); ok {
		return p
	}
	if ps, ok := r.Prompt.([]string); ok {
		if len(ps) == 0 {
			return ""
		}
		return ps[0]
	}

	// check if it is prompt is []string hidden under []any
	slice, isSlice := r.Prompt.([]any)
	if !isSlice {
		return ""
	}

	if len(slice) == 0 {
		return ""
	}
	if p, ok := slice[0].(string); ok {
		return p
	}

	return ""
}

// CompletionChoice represents one of possible completions.
type CompletionChoice struct {
	Text         string         `json:"text"`
	Index        int            `json:"index"`
	FinishReason string         `json:"finish_reason"`
	LogProbs     *LogprobResult `json:"logprobs,omitempty"`
}

// LogprobResult represents logprob result of Choice.
type LogprobResult struct {
	Tokens        []string             `json:"tokens,omitempty"`
	TokenLogprobs []float32            `json:"token_logprobs,omitempty"`
	TopLogprobs   []map[string]float32 `json:"top_logprobs,omitempty"`
	TextOffset    []int                `json:"text_offset,omitempty"`
}

// CompletionResponse represents a response structure for completion API.
type CompletionResponse struct {
	ID      string             `json:"id,omitempty"`
	Object  string             `json:"object"`
	Created int64              `json:"created,omitempty"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   *Usage             `json:"usage,omitempty"`

	// Preserve unknown fields to fully support the extended set of fields that backends such as vLLM support.
	easyjson.UnknownFieldsProxy
}
