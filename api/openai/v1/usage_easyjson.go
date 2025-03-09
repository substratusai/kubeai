// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.

package v1

import (
	json "encoding/json"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV1(in *jlexer.Lexer, out *Usage) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "prompt_tokens":
			out.PromptTokens = int(in.Int())
		case "completion_tokens":
			out.CompletionTokens = int(in.Int())
		case "total_tokens":
			out.TotalTokens = int(in.Int())
		case "prompt_tokens_details":
			if in.IsNull() {
				in.Skip()
				out.PromptTokensDetails = nil
			} else {
				if out.PromptTokensDetails == nil {
					out.PromptTokensDetails = new(PromptTokensDetails)
				}
				(*out.PromptTokensDetails).UnmarshalEasyJSON(in)
			}
		case "completion_tokens_details":
			if in.IsNull() {
				in.Skip()
				out.CompletionTokensDetails = nil
			} else {
				if out.CompletionTokensDetails == nil {
					out.CompletionTokensDetails = new(CompletionTokensDetails)
				}
				(*out.CompletionTokensDetails).UnmarshalEasyJSON(in)
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV1(out *jwriter.Writer, in Usage) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"prompt_tokens\":"
		out.RawString(prefix[1:])
		out.Int(int(in.PromptTokens))
	}
	{
		const prefix string = ",\"completion_tokens\":"
		out.RawString(prefix)
		out.Int(int(in.CompletionTokens))
	}
	{
		const prefix string = ",\"total_tokens\":"
		out.RawString(prefix)
		out.Int(int(in.TotalTokens))
	}
	if in.PromptTokensDetails != nil {
		const prefix string = ",\"prompt_tokens_details\":"
		out.RawString(prefix)
		(*in.PromptTokensDetails).MarshalEasyJSON(out)
	}
	if in.CompletionTokensDetails != nil {
		const prefix string = ",\"completion_tokens_details\":"
		out.RawString(prefix)
		(*in.CompletionTokensDetails).MarshalEasyJSON(out)
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v Usage) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV1(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v Usage) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV1(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *Usage) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV1(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *Usage) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV1(l, v)
}
func easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV11(in *jlexer.Lexer, out *PromptTokensDetails) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "audio_tokens":
			if in.IsNull() {
				in.Skip()
				out.AudioTokens = nil
			} else {
				if out.AudioTokens == nil {
					out.AudioTokens = new(int)
				}
				*out.AudioTokens = int(in.Int())
			}
		case "cached_tokens":
			if in.IsNull() {
				in.Skip()
				out.CachedTokens = nil
			} else {
				if out.CachedTokens == nil {
					out.CachedTokens = new(int)
				}
				*out.CachedTokens = int(in.Int())
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV11(out *jwriter.Writer, in PromptTokensDetails) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"audio_tokens\":"
		out.RawString(prefix[1:])
		if in.AudioTokens == nil {
			out.RawString("null")
		} else {
			out.Int(int(*in.AudioTokens))
		}
	}
	{
		const prefix string = ",\"cached_tokens\":"
		out.RawString(prefix)
		if in.CachedTokens == nil {
			out.RawString("null")
		} else {
			out.Int(int(*in.CachedTokens))
		}
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v PromptTokensDetails) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV11(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v PromptTokensDetails) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV11(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *PromptTokensDetails) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV11(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *PromptTokensDetails) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV11(l, v)
}
func easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV12(in *jlexer.Lexer, out *CompletionTokensDetails) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "audio_tokens":
			if in.IsNull() {
				in.Skip()
				out.AudioTokens = nil
			} else {
				if out.AudioTokens == nil {
					out.AudioTokens = new(int)
				}
				*out.AudioTokens = int(in.Int())
			}
		case "reasoning_tokens":
			if in.IsNull() {
				in.Skip()
				out.ReasoningTokens = nil
			} else {
				if out.ReasoningTokens == nil {
					out.ReasoningTokens = new(int)
				}
				*out.ReasoningTokens = int(in.Int())
			}
		case "accepted_prediction_tokens":
			if in.IsNull() {
				in.Skip()
				out.AcceptedPredictionTokens = nil
			} else {
				if out.AcceptedPredictionTokens == nil {
					out.AcceptedPredictionTokens = new(int)
				}
				*out.AcceptedPredictionTokens = int(in.Int())
			}
		case "rejected_prediction_tokens":
			if in.IsNull() {
				in.Skip()
				out.RejectedPredictionTokens = nil
			} else {
				if out.RejectedPredictionTokens == nil {
					out.RejectedPredictionTokens = new(int)
				}
				*out.RejectedPredictionTokens = int(in.Int())
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV12(out *jwriter.Writer, in CompletionTokensDetails) {
	out.RawByte('{')
	first := true
	_ = first
	if in.AudioTokens != nil {
		const prefix string = ",\"audio_tokens\":"
		first = false
		out.RawString(prefix[1:])
		out.Int(int(*in.AudioTokens))
	}
	if in.ReasoningTokens != nil {
		const prefix string = ",\"reasoning_tokens\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int(int(*in.ReasoningTokens))
	}
	if in.AcceptedPredictionTokens != nil {
		const prefix string = ",\"accepted_prediction_tokens\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int(int(*in.AcceptedPredictionTokens))
	}
	if in.RejectedPredictionTokens != nil {
		const prefix string = ",\"rejected_prediction_tokens\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int(int(*in.RejectedPredictionTokens))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v CompletionTokensDetails) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV12(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v CompletionTokensDetails) MarshalEasyJSON(w *jwriter.Writer) {
	easyjsonEea1550fEncodeGithubComSubstratusaiKubeaiApiOpenaiV12(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *CompletionTokensDetails) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV12(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *CompletionTokensDetails) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjsonEea1550fDecodeGithubComSubstratusaiKubeaiApiOpenaiV12(l, v)
}
