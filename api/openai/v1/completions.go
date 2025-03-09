package v1

import easyjson "github.com/mailru/easyjson"

// CompletionRequest represents a request structure for completion API.
type CompletionRequest struct {
	// Model is the ID of the model to use. You can use the List models API to see all available models.
	// +required
	Model string `json:"model"`

	// Prompt is the prompt(s) to generate completions for, encoded as a string, array of strings,
	// array of tokens, or array of token arrays. Note that <|endoftext|> is the document separator
	// that the model sees during training, so if a prompt is not specified the model will generate
	// as if from the beginning of a new document.
	// +required
	Prompt any `json:"prompt"`

	// BestOf generates `best_of` completions server-side and returns the "best" (the one with the
	// highest log probability per token). Results cannot be streamed. When used with `n`, `best_of`
	// controls the number of candidate completions and `n` specifies how many to return – `best_of`
	// must be greater than `n`.
	// Default: 1
	// +optional
	BestOf *int `json:"best_of,omitempty"`

	// Echo determines whether to echo back the prompt in addition to the completion.
	// +optional
	Echo bool `json:"echo,omitempty"`

	// FrequencyPenalty is a number between -2.0 and 2.0. Positive values penalize new tokens based on
	// their existing frequency in the text so far, decreasing the model's likelihood to repeat the
	// same line verbatim.
	// +optional
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`

	// LogitBias modifies the likelihood of specified tokens appearing in the completion.
	// Accepts a JSON object that maps tokens (specified by their token ID in the GPT tokenizer)
	// to an associated bias value from -100 to 100. Values between -1 and 1 should decrease or
	// increase likelihood of selection; values like -100 or 100 should result in a ban or
	// exclusive selection of the relevant token.
	// +optional
	LogitBias map[string]int `json:"logit_bias,omitempty"`

	// Store can be set to true to store the output of this completion request for use in distillations and evals.
	// https://platform.openai.com/docs/api-reference/chat/create#chat-create-store
	// +optional
	Store bool `json:"store,omitempty"`

	// Metadata to store with the completion.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`

	// LogProbs includes the log probabilities on the `logprobs` most likely output tokens, as well
	// the chosen tokens. For example, if `logprobs` is 5, the API will return a list of the 5 most
	// likely tokens. The API will always return the `logprob` of the sampled token, so there may be
	// up to `logprobs+1` elements in the response. The maximum value for `logprobs` is 5.
	// +optional
	LogProbs *int `json:"logprobs,omitempty"`

	// MaxTokens is the maximum number of tokens that can be generated in the completion.
	// The token count of your prompt plus `max_tokens` cannot exceed the model's context length.
	// Default: 16
	// +optional
	MaxTokens int `json:"max_tokens,omitempty"`

	// N specifies how many completions to generate for each prompt.
	// Default: 1
	// +optional
	N *int `json:"n,omitempty"`

	// PresencePenalty is a number between -2.0 and 2.0. Positive values penalize new tokens based
	// on whether they appear in the text so far, increasing the model's likelihood to talk about
	// new topics.
	// +optional
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`

	// Seed can be specified to make a best effort to sample deterministically, such that
	// repeated requests with the same `seed` and parameters should return the same result.
	// Determinism is not guaranteed, and you should refer to the `system_fingerprint`
	// response parameter to monitor changes in the backend.
	// +optional
	Seed *int `json:"seed,omitempty"`

	// Stop specifies up to 4 sequences where the API will stop generating further tokens.
	// The returned text will not contain the stop sequence.
	// +optional
	Stop []string `json:"stop,omitempty"`

	// Stream determines whether to stream back partial progress. If set, tokens will be sent as
	// data-only server-sent events as they become available, with the stream terminated by a
	// `data: [DONE]` message.
	// +optional
	Stream bool `json:"stream,omitempty"`

	// Suffix is the suffix that comes after a completion of inserted text.
	// +optional
	Suffix string `json:"suffix,omitempty"`

	// Temperature controls the sampling temperature to use, between 0 and 2. Higher values
	// like 0.8 will make the output more random, while lower values like 0.2 will make it
	// more focused and deterministic. It is generally recommended to alter this or `top_p`
	// but not both.
	// Default: 1
	// +optional
	Temperature *float32 `json:"temperature,omitempty"`

	// TopP is an alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1 means only
	// the tokens comprising the top 10% probability mass are considered. It is generally
	// recommended to alter this or `temperature` but not both.
	// Default: 1
	// +optional
	TopP *float32 `json:"top_p,omitempty"`

	// User is a unique identifier representing your end-user, which can help OpenAI to monitor
	// and detect abuse.
	// +optional
	User string `json:"user,omitempty"`

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
	// Text is the generated completion text.
	// +required
	Text string `json:"text"`

	// Index is the index of the completion choice.
	// +required
	Index int `json:"index"`

	// FinishReason indicates the reason the model stopped generating tokens. This will be
	// `stop` if the model hit a natural stop point or a provided stop sequence,
	// `length` if the maximum number of tokens specified in the request was reached,
	// or `content_filter` if content was omitted due to a flag from content filters.
	// +required
	FinishReason string `json:"finish_reason"`

	// LogProbs contains log probability information if requested in the API call.
	// +optional
	LogProbs *LogprobResult `json:"logprobs,omitempty"`
}

// LogprobResult represents logprob result of Choice.
type LogprobResult struct {
	// Tokens contains the tokens generated in the completion.
	// +optional
	Tokens []string `json:"tokens,omitempty"`

	// TokenLogprobs contains the log probability for each token.
	// +optional
	TokenLogprobs []float32 `json:"token_logprobs,omitempty"`

	// TopLogprobs contains alternative tokens and their log probabilities.
	// +optional
	TopLogprobs []map[string]float32 `json:"top_logprobs,omitempty"`

	// TextOffset contains the character offset from the start of the text for each token.
	// +optional
	TextOffset []int `json:"text_offset,omitempty"`
}

// CompletionResponse represents a response structure for completion API.
type CompletionResponse struct {
	// ID is a unique identifier for the completion.
	// +required
	ID string `json:"id,omitempty"`

	// Object is the object type, which is always "text_completion".
	// +required
	Object string `json:"object"`

	// Created is the Unix timestamp (in seconds) of when the completion was created.
	// +required
	Created int64 `json:"created,omitempty"`

	// Model is the model used for completion.
	// +required
	Model string `json:"model"`

	// Choices contains the list of completion choices the model generated for the input prompt.
	// +required
	Choices []CompletionChoice `json:"choices"`

	// Usage provides usage statistics for the completion request.
	// +optional
	Usage *Usage `json:"usage,omitempty"`

	// SystemFingerprint represents the backend configuration that the model runs with.
	// Can be used with the seed parameter to understand when backend changes have been made
	// that might impact determinism.
	// +optional
	SystemFingerprint string `json:"system_fingerprint,omitempty"`

	// Preserve unknown fields to fully support the extended set of fields that backends such as vLLM support.
	easyjson.UnknownFieldsProxy
}
