package v1

import (
	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionRequest struct {
	openai.ChatCompletionRequest
}

func (r *ChatCompletionRequest) GetModel() string {
	return r.Model
}

func (r *ChatCompletionRequest) SetModel(m string) {
	r.Model = m
}

func (r *ChatCompletionRequest) Prefix(n int) string {
	if len(r.Messages) == 0 {
		return ""
	}
	for _, m := range r.Messages {
		if m.Role == openai.ChatMessageRoleUser {
			return firstNChars(m.Content, n)
		}
	}
	return ""
}

type ChatCompletionResponse struct {
	openai.ChatCompletionResponse
}

type CompletionRequest struct {
	openai.CompletionRequest
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

type CompletionResponse struct {
	openai.CompletionResponse
}

// firstNChars returns the first n characters of a string.
// This function is needed because Go's string indexing is based on bytes, not runes.
func firstNChars(s string, n int) string {
	runes := []rune(s)
	return string(runes[:min(n, len(runes))])
}
