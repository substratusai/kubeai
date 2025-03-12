package v1

import (
	"errors"
	"fmt"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Chat message roles defined by the OpenAI API.
const (
	// ChatMessageRoleSystem is used for developer-provided instructions (will be replaced by developer for o1 models and newer)
	ChatMessageRoleSystem = "system"
	// ChatMessageRoleUser is used for messages sent by an end user
	ChatMessageRoleUser = "user"
	// ChatMessageRoleAssistant is used for messages sent by the model in response to user messages
	ChatMessageRoleAssistant = "assistant"
	// ChatMessageRoleFunction is deprecated and replaced by tool
	ChatMessageRoleFunction = "function"
	// ChatMessageRoleTool is used for messages responding to a tool call
	ChatMessageRoleTool = "tool"
	// ChatMessageRoleDeveloper is used for developer-provided instructions that models should follow regardless of user messages
	ChatMessageRoleDeveloper = "developer"
)

// Hate represents content filter results for hate speech.
type Hate struct {
	// Filtered indicates whether content was filtered due to hate speech.
	// +required
	Filtered bool `json:"filtered"`

	// Severity indicates the severity level of the filtered content.
	// +optional
	Severity string `json:"severity,omitzero"`
}

// SelfHarm represents content filter results for self-harm content.
type SelfHarm struct {
	// Filtered indicates whether content was filtered due to self-harm references.
	// +required
	Filtered bool `json:"filtered"`

	// Severity indicates the severity level of the filtered content.
	// +optional
	Severity string `json:"severity,omitzero"`
}

// Sexual represents content filter results for sexual content.
type Sexual struct {
	// Filtered indicates whether content was filtered due to sexual content.
	// +required
	Filtered bool `json:"filtered"`

	// Severity indicates the severity level of the filtered content.
	// +optional
	Severity string `json:"severity,omitzero"`
}

// Violence represents content filter results for violent content.
type Violence struct {
	// Filtered indicates whether content was filtered due to violent content.
	// +required
	Filtered bool `json:"filtered"`

	// Severity indicates the severity level of the filtered content.
	// +optional
	Severity string `json:"severity,omitzero"`
}

// JailBreak represents content filter results for jailbreak attempts.
type JailBreak struct {
	// Filtered indicates whether content was filtered due to a detected jailbreak attempt.
	// +required
	Filtered bool `json:"filtered"`

	// Detected indicates whether a jailbreak attempt was detected.
	// +required
	Detected bool `json:"detected"`
}

// Profanity represents content filter results for profane content.
type Profanity struct {
	// Filtered indicates whether content was filtered due to profanity.
	// +required
	Filtered bool `json:"filtered"`

	// Detected indicates whether profanity was detected.
	// +required
	Detected bool `json:"detected"`
}

// ContentFilterResults contains the results of content filtering across different categories.
// This structure is used to indicate content that was flagged by OpenAI's content filters.
type ContentFilterResults struct {
	// Hate contains filtering results for hate speech content.
	// +optional
	Hate *Hate `json:"hate,omitzero"`

	// SelfHarm contains filtering results for self-harm content.
	// +optional
	SelfHarm *SelfHarm `json:"self_harm,omitzero"`

	// Sexual contains filtering results for sexual content.
	// +optional
	Sexual *Sexual `json:"sexual,omitzero"`

	// Violence contains filtering results for violent content.
	// +optional
	Violence *Violence `json:"violence,omitzero"`

	// JailBreak contains filtering results for jailbreak attempts.
	// +optional
	JailBreak *JailBreak `json:"jailbreak,omitzero"`

	// Profanity contains filtering results for profane content.
	// +optional
	Profanity *Profanity `json:"profanity,omitzero"`
}

// ImageURLDetail specifies the detail level of the image in a vision request.
// Learn more in the Vision guide at https://platform.openai.com/docs/guides/vision
type ImageURLDetail string

const (
	// ImageURLDetailHigh is used for high-detail image understanding
	ImageURLDetailHigh ImageURLDetail = "high"
	// ImageURLDetailLow is used for low-detail image understanding
	ImageURLDetailLow ImageURLDetail = "low"
	// ImageURLDetailAuto lets the model determine the appropriate detail level (default)
	ImageURLDetailAuto ImageURLDetail = "auto"
)

// ChatMessageImageURL represents an image input in a chat message.
// Learn more about image inputs at https://platform.openai.com/docs/guides/vision
type ChatMessageImageURL struct {
	// URL is either a URL of the image or the base64 encoded image data
	// +required
	URL string `json:"url"`

	// Detail specifies the detail level of the image
	// Default: "auto"
	// +optional
	Detail ImageURLDetail `json:"detail,omitzero"`
}

// ChatMessagePartType defines the types of content parts that can be included in a message.
type ChatMessagePartType string

const (
	// ChatMessagePartTypeText represents a text content part
	ChatMessagePartTypeText ChatMessagePartType = "text"
	// ChatMessagePartTypeImageURL represents an image content part
	ChatMessagePartTypeImageURL ChatMessagePartType = "image_url"
)

// ChatMessageContentPart represents a part of a message's content with a specific type.
// Used for multimodal messages that can contain text, images, or other content types.
type ChatMessageContentPart struct {
	// Type is the type of the content part (text, image_url, etc.)
	// +required
	Type ChatMessagePartType `json:"type,omitzero"`

	// Text contains the text content (used when Type is "text")
	// +optional
	Text string `json:"text,omitzero"`

	// ImageURL contains the image data (used when Type is "image_url")
	// +optional
	ImageURL *ChatMessageImageURL `json:"image_url,omitzero"`
}

// ChatMessageContent is a struct that represents the content of a chat message.
// It can be either a plain string or an array of content parts with defined types.
// This structure handles both simple text messages and multimodal content.
type ChatMessageContent struct {
	// String contains the message content as a plain string.
	// Should not be set when Array is set.
	// +optional
	String string

	// Array contains the message content as an array of content parts.
	// Should not be set when String is set.
	// +optional
	Array []ChatMessageContentPart
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *ChatMessageContent) UnmarshalJSON(data []byte) error {
	// Try unmarshaling as a string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.String = str
		c.Array = nil
		return nil
	}

	// If not a string, try as an array
	var arr []ChatMessageContentPart
	if err := json.Unmarshal(data, &arr); err == nil {
		c.String = ""
		c.Array = arr
		return nil
	}

	return fmt.Errorf("content must be either a string or an array of content parts")
}

// MarshalJSON implements the json.Marshaler interface.
func (c ChatMessageContent) MarshalJSON() ([]byte, error) {
	if c.String != "" && c.Array != nil {
		return nil, errors.New("ChatMessageContent: String and Array cannot be specified at the same time")
	}

	if c.Array != nil {
		return json.Marshal(c.Array)
	}

	return json.Marshal(c.String)
}

// ChatCompletionMessage represents a message in a chat conversation.
// Messages can be from different roles (system, user, assistant, tool, etc.)
// and can contain text, images, function calls, and other content types.
type ChatCompletionMessage struct {
	// Role is the role of the message author (system, user, assistant, tool, developer, etc.)
	// +required
	Role string `json:"role"`

	// Content contains the text content of the message, can be string or array format.
	// Required unless tool_calls or function_call is specified for assistant messages.
	// +optional
	Content *ChatMessageContent `json:"content"`

	// Refusal contains the refusal message if the model refuses to respond.
	// NOTE: When OpenAI responded with an assistant message, it responds with `refusal: null`.
	//       This API will omit the field in those cases (similar to what Ollama does).
	// +optional
	Refusal string `json:"refusal,omitzero"`

	// Name is an optional identifier for the participant.
	// Provides the model information to differentiate between participants of the same role.
	// +optional
	Name string `json:"name,omitzero"`

	// FunctionCall contains details about the function to call.
	// Deprecated: Use ToolCalls instead.
	// +optional
	FunctionCall *FunctionCall `json:"function_call,omitzero"`

	// ToolCalls contains the tool calls generated by the model, such as function calls.
	// This is used when Role="assistant".
	// +optional
	ToolCalls []ToolCall `json:"tool_calls,omitzero"`

	// ToolCallID is the ID of the tool call this message is responding to.
	// This is required when Role="tool".
	// +optional
	ToolCallID string `json:"tool_call_id,omitzero"`

	// Audio contains audio data when the model generates audio responses.
	// +optional
	Audio *AudioResponse `json:"audio,omitzero"`
}

// ToolCall represents a tool that the model calls such as a function call.
type ToolCall struct {
	// Index is only used in chat completion chunk objects.
	// +optional
	Index *int `json:"index,omitzero"`

	// ID is the unique identifier for this tool call.
	// +required
	ID string `json:"id"`

	// Type specifies the type of the tool. Currently only "function" is supported.
	// +required
	Type ToolType `json:"type"`

	// Function contains details about the function that should be called.
	// +required
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function that the model calls with specific arguments.
type FunctionCall struct {
	// Name is the name of the function to call.
	// +required
	Name string `json:"name"`

	// Arguments is a string containing the arguments to pass to the function in JSON format.
	// Note: The model may not always generate valid JSON, and may hallucinate parameters
	// not defined by your function schema. Validate the arguments in your code before
	// calling your function.
	// +required
	Arguments string `json:"arguments"`
}

// ChatCompletionResponseFormatType defines the format types for model responses.
type ChatCompletionResponseFormatType string

const (
	// ChatCompletionResponseFormatTypeJSONObject enables JSON mode, which ensures the message is valid JSON
	ChatCompletionResponseFormatTypeJSONObject ChatCompletionResponseFormatType = "json_object"

	// ChatCompletionResponseFormatTypeJSONSchema enables Structured Outputs which ensures the model matches a JSON schema
	ChatCompletionResponseFormatTypeJSONSchema ChatCompletionResponseFormatType = "json_schema"

	// ChatCompletionResponseFormatTypeText specifies text format (default)
	ChatCompletionResponseFormatTypeText ChatCompletionResponseFormatType = "text"
)

// ChatCompletionResponseFormat specifies the format that the model must output.
// This can be used to request JSON or structured data from the model.
type ChatCompletionResponseFormat struct {
	// Type specifies the format type: "text", "json_object", or "json_schema"
	// +required
	Type ChatCompletionResponseFormatType `json:"type"`

	// JSONSchema contains schema information when Type is "json_schema"
	// +optional
	JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitzero"`
}

// ChatCompletionResponseFormatJSONSchema defines a JSON schema for structured model output.
// Learn more in the Structured Outputs guide: https://platform.openai.com/docs/guides/structured-outputs
type ChatCompletionResponseFormatJSONSchema struct {
	// Name is the name of the response format.
	// Must be a-z, A-Z, 0-9, or contain underscores and dashes, with a maximum length of 64.
	// +required
	Name string `json:"name"`

	// Description explains what the response format is for.
	// Used by the model to determine how to respond in the format.
	// +optional
	Description string `json:"description,omitzero"`

	// Schema is the schema for the response format, described as a JSON Schema object.
	// +required
	Schema interface{} `json:"schema"`

	// Strict enables strict schema adherence when generating the output.
	// If true, the model will always follow the exact schema defined.
	// +optional
	Strict bool `json:"strict,omitzero"`
}

// ChatCompletionRequest represents a request structure for chat completion API.
// Used to create a model response for a given chat conversation.
type ChatCompletionRequest struct {
	// Model is the ID of the model to use for completion.
	// See the model endpoint compatibility table for details on which models work with the Chat API.
	// +required
	Model string `json:"model"`

	// Messages is a list of messages comprising the conversation so far.
	// Different message types (modalities) are supported, like text, images, and audio.
	// +required
	Messages []ChatCompletionMessage `json:"messages,format:emitnull"`

	// MaxTokens is the maximum number of tokens to generate in the chat completion.
	// Deprecated: Use MaxCompletionTokens instead.
	// Should be a value `>= 1`.
	// +optional
	MaxTokens int `json:"max_tokens,omitzero"`

	// MaxCompletionTokens is an upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and reasoning tokens.
	// Should be a value `>= 1`.
	// +optional
	MaxCompletionTokens int `json:"max_completion_tokens,omitzero"`

	// Temperature controls randomness in the output. Values between 0 and 2.
	// Higher values like 0.8 make output more random, while lower values like 0.2 make it more focused and deterministic.
	// Default: 1.0
	// +optional
	Temperature *float32 `json:"temperature,omitzero"`

	// TopP is an alternative to sampling with temperature, called nucleus sampling.
	// The model considers the results of the tokens with top_p probability mass.
	// So 0.1 means only the tokens comprising the top 10% probability mass are considered.
	// Default: 1.0
	// +optional
	TopP *float32 `json:"top_p,omitzero"`

	// N specifies how many chat completion choices to generate for each input message.
	// Note that you will be charged based on the number of generated tokens across all choices.
	// Default: 1
	// +optional
	N *int `json:"n,omitzero"`

	// Stream enables partial message deltas to be sent as they're generated.
	// If true, tokens will be sent as data-only server-sent events as they become available.
	// +optional
	Stream bool `json:"stream,omitzero"`

	// Stop sequences are up to 4 sequences where the API will stop generating further tokens.
	// +optional
	Stop []string `json:"stop,omitzero"`

	// PresencePenalty is a number between -2.0 and 2.0.
	// Positive values penalize new tokens based on whether they appear in the text so far,
	// increasing the model's likelihood to talk about new topics.
	// Default: 0
	// +optional
	PresencePenalty *float32 `json:"presence_penalty,omitzero"`

	// ResponseFormat specifies the format that the model must output.
	// Can be used to request JSON or structured data from the model.
	// +optional
	ResponseFormat *ChatCompletionResponseFormat `json:"response_format,omitzero"`

	// Seed enables deterministic sampling for consistent outputs.
	// If specified, the system will make a best effort to sample deterministically,
	// such that repeated requests with the same seed and parameters should return the same result.
	// +optional
	Seed *int `json:"seed,omitzero"`

	// FrequencyPenalty is a number between -2.0 and 2.0.
	// Positive values penalize new tokens based on their existing frequency in the text so far,
	// decreasing the model's likelihood to repeat the same line verbatim.
	// Default: 0
	// +optional
	FrequencyPenalty *float32 `json:"frequency_penalty,omitzero"`

	// LogitBias modifies the likelihood of specified tokens appearing in the completion.
	// Maps tokens (specified by their token ID in the tokenizer) to an associated bias value from -100 to 100.
	// +optional
	LogitBias map[string]int `json:"logit_bias,omitzero"`

	// LogProbs indicates whether to return log probabilities of the output tokens.
	// If true, returns the log probabilities of each output token returned in the content of message.
	// +optional
	LogProbs bool `json:"logprobs,omitzero"`

	// TopLogProbs specifies the number of most likely tokens to return at each token position (0-20).
	// Requires logprobs to be true.
	// +optional
	TopLogProbs *int `json:"top_logprobs,omitzero"`

	// User is a unique identifier representing your end-user.
	// This helps OpenAI to monitor and detect abuse.
	// +optional
	User string `json:"user,omitzero"`

	// Functions is a list of functions the model may generate JSON inputs for.
	// Deprecated: Use Tools instead.
	// +optional
	Functions []FunctionDefinition `json:"functions,omitzero"`

	// FunctionCall controls which function is called by the model.
	// Deprecated: Use ToolChoice instead.
	// +optional
	FunctionCall interface{} `json:"function_call,omitzero"`

	// Tools is a list of tools the model may call.
	// Currently, only functions are supported as tools.
	// +optional
	Tools []Tool `json:"tools,omitzero"`

	// ToolChoice controls which (if any) tool is called by the model.
	// Can be "none", "auto", "required" or a specific tool choice object.
	// TODO: Update to be a string type (enum pattern).
	// +optional
	ToolChoice interface{} `json:"tool_choice,omitzero"`

	// StreamOptions configures options for streaming response.
	// Only set this when stream is true.
	// +optional
	StreamOptions *StreamOptions `json:"stream_options,omitzero"`

	// ParallelToolCalls enables parallel function calling during tool use.
	// Default: true
	// +optional
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitzero"`

	// Store determines whether to store the output for model distillation or evals products.
	// +optional
	Store bool `json:"store,omitzero"`

	// ReasoningEffort controls effort on reasoning for reasoning models.
	// Values can be "low", "medium", or "high". Reducing reasoning effort results in faster responses.
	// Default: "medium"
	// +optional
	ReasoningEffort string `json:"reasoning_effort,omitzero"`

	// Metadata is a set of 16 key-value pairs that can be attached to an object.
	// Useful for storing additional information in a structured format.
	// +optional
	Metadata map[string]string `json:"metadata,omitzero"`

	// ServiceTier specifies the latency tier for processing the request.
	// Can be "auto" or "default".
	// Default: "auto"
	// +optional
	ServiceTier string `json:"service_tier,omitzero"`

	// Modalities specifies output types that the model should generate for this request.
	// Most models generate text by default. Some models can also generate audio.
	// +optional
	Modalities []string `json:"modalities,omitzero"`

	// Prediction provides static content for faster responses.
	// Can improve response times when large parts of the model response are known ahead of time.
	// +optional
	Prediction *PredictionContent `json:"prediction,omitzero"`

	// Audio contains parameters for audio output.
	// Required when audio output is requested with modalities: ["audio"].
	// +optional
	Audio *AudioConfig `json:"audio,omitzero"`

	// Unknown fields should be preserved to fully support the extended set of fields that backends such as vLLM support.
	Unknown jsontext.Value `json:",unknown"`
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
		if m.Role == ChatMessageRoleUser {
			var s string
			if len(m.Content.Array) > 0 {
				for i := 0; i < len(m.Content.Array); i++ {
					s += m.Content.Array[i].Text
				}
			} else {
				s = m.Content.String
			}
			return firstNChars(s, n)
		}
	}
	return ""
}

// ToolType defines the type of tool that the model can use.
type ToolType string

const (
	// ToolTypeFunction represents a function tool type
	// Currently, only function is supported as a tool type
	ToolTypeFunction ToolType = "function"
)

// Tool represents a tool the model may call, such as a function.
// A max of 128 tools are supported.
type Tool struct {
	// Type specifies the type of the tool. Currently only "function" is supported.
	// +required
	Type ToolType `json:"type"`

	// Function contains the definition of the function that can be called.
	// +required
	Function *FunctionDefinition `json:"function,omitzero"`
}

// ToolChoice specifies a particular tool the model should use.
// Used to force the model to call a specific function.
type ToolChoice struct {
	// Type specifies the type of the tool. Currently only "function" is supported.
	// +required
	Type ToolType `json:"type"`

	// Function contains information about the specific function to call.
	// +required
	Function ToolFunction `json:"function,omitzero"`
}

// ToolFunction specifies a named function to call.
type ToolFunction struct {
	// Name is the name of the function to call.
	// +required
	Name string `json:"name"`
}

// FunctionDefinition defines a function that the model can call.
type FunctionDefinition struct {
	// Name is the name of the function to be called.
	// Must be a-z, A-Z, 0-9, or contain underscores and dashes, with a maximum length of 64.
	// +required
	Name string `json:"name"`

	// Description explains what the function does and when it should be called.
	// Used by the model to determine when and how to call the function.
	// +optional
	Description string `json:"description,omitzero"`

	// Strict enables strict schema adherence when generating the function call.
	// If true, the model will follow the exact schema defined in the parameters field.
	// +optional
	Strict bool `json:"strict,omitzero"`

	// Parameters is an object describing the function parameters as a JSON Schema object.
	// +required
	Parameters any `json:"parameters"`
}

// Deprecated: use FunctionDefinition instead.
type FunctionDefine = FunctionDefinition

// TopLogProbs represents one of the most likely tokens at a given position with its probability information.
type TopLogProbs struct {
	// Token is the token text.
	// +required
	Token string `json:"token"`

	// LogProb is the log probability of this token.
	// +required
	LogProb float64 `json:"logprob"`

	// Bytes is a list of integers representing the UTF-8 bytes representation of the token.
	// Can be null if there is no bytes representation for the token.
	// +optional
	Bytes []int `json:"bytes"`
}

// LogProb represents the probability information for a token.
type LogProb struct {
	// Token is the token text.
	// +required
	Token string `json:"token"`

	// LogProb is the log probability of this token.
	// If the token is within the top 20 most likely tokens, this is its actual log probability.
	// Otherwise, the value -9999.0 is used to signify that the token is very unlikely.
	// +required
	LogProb float64 `json:"logprob"`

	// Bytes is a list of integers representing the UTF-8 bytes representation of the token.
	// Useful when characters are represented by multiple tokens and their byte representations
	// must be combined to generate the correct text representation.
	// Can be null if there is no bytes representation for the token.
	// +optional
	Bytes []int `json:"bytes,omitzero"`

	// TopLogProbs is a list of the most likely tokens and their log probability at this token position.
	// In rare cases, there may be fewer than the number of requested top_logprobs returned.
	// +required
	TopLogProbs []TopLogProbs `json:"top_logprobs"`
}

// LogProbs is the top-level structure containing the log probability information.
type LogProbs struct {
	// Content is a list of message content tokens with log probability information.
	// +required
	Content []LogProb `json:"content"`
}

// FinishReason indicates why the model stopped generating tokens.
type FinishReason string

const (
	// FinishReasonStop indicates the model hit a natural stop point or a provided stop sequence.
	FinishReasonStop FinishReason = "stop"

	// FinishReasonLength indicates incomplete model output due to max_tokens parameter or token limit.
	FinishReasonLength FinishReason = "length"

	// FinishReasonFunctionCall indicates the model decided to call a function.
	// Deprecated: Use FinishReasonToolCalls instead.
	FinishReasonFunctionCall FinishReason = "function_call"

	// FinishReasonToolCalls indicates the model decided to call tools.
	FinishReasonToolCalls FinishReason = "tool_calls"

	// FinishReasonContentFilter indicates content was omitted due to a flag from content filters.
	FinishReasonContentFilter FinishReason = "content_filter"
)

// AudioResponse contains data about an audio response from the model.
// Learn more at https://platform.openai.com/docs/guides/audio
type AudioResponse struct {
	// ID is a unique identifier for this audio response.
	// +required
	ID string `json:"id"`

	// ExpiresAt is the Unix timestamp (in seconds) for when this audio response will no longer be accessible.
	// After this time, the audio response will not be available for use in multi-turn conversations.
	// +required
	ExpiresAt int64 `json:"expires_at"`

	// Data contains base64 encoded audio bytes generated by the model, in the format
	// specified in the request.
	// +required
	Data string `json:"data"`

	// Transcript is the text transcript of the audio generated by the model.
	// +required
	Transcript string `json:"transcript"`
}

// AudioConfig represents the parameters for audio output.
// Required when audio output is requested with modalities: ["audio"].
// Learn more at https://platform.openai.com/docs/guides/audio
type AudioConfig struct {
	// Voice specifies the voice the model uses to respond.
	// +required
	Voice string `json:"voice"`

	// Format specifies the output audio format.
	// Must be one of "wav", "mp3", "flac", "opus", or "pcm16".
	// +required
	Format string `json:"format"`
}

// StreamOptions represents options for streaming response.
// Only used when the stream parameter is set to true.
type StreamOptions struct {
	// IncludeUsage determines whether to include usage statistics in the streaming response.
	// If true, an additional chunk will be streamed before the final [DONE] message,
	// containing token usage statistics for the entire request.
	// All other chunks will include a usage field with a null value.
	// +optional
	IncludeUsage bool `json:"include_usage,omitzero"`
}

// ToolChoiceOption represents a named tool choice option for forcing a specific tool call.
// Used to force the model to call a specific function.
type ToolChoiceOption struct {
	// Type specifies the type of the tool. Currently only "function" is supported.
	// +required
	Type string `json:"type"`

	// Function contains information about the specific function to call.
	// +required
	Function ToolFunction `json:"function"`
}

// ChatCompletionToolChoiceString is a string enum for tool choice options.
type ChatCompletionToolChoiceString string

const (
	// ToolChoiceNone means the model will not call any tool and generates a message.
	// This is the default when no tools are present.
	ToolChoiceNone ChatCompletionToolChoiceString = "none"

	// ToolChoiceAuto means the model can choose between generating a message or calling tools.
	// This is the default if tools are present.
	ToolChoiceAuto ChatCompletionToolChoiceString = "auto"

	// ToolChoiceRequired means the model must call one or more tools.
	ToolChoiceRequired ChatCompletionToolChoiceString = "required"
)

// PredictionContent represents static predicted output content for faster responses.
// Used with Predicted Outputs to improve response times when large parts
// of the model response are known ahead of time.
type PredictionContent struct {
	// Type is the type of predicted content. Currently always "content".
	// +required
	Type string `json:"type"`

	// Content is the content that should be matched when generating a model response.
	// If generated tokens would match this content, the entire model response can be returned much more quickly.
	// Can be string or array of content parts.
	// +required
	Content interface{} `json:"content"`
}

// ChatCompletionChoice represents a single completion choice generated by the model.
type ChatCompletionChoice struct {
	// Index is the index of the choice in the list of choices.
	// +required
	Index int `json:"index"`

	// Message is the chat completion message generated by the model.
	// +required
	Message ChatCompletionMessage `json:"message"`

	// FinishReason indicates why the model stopped generating tokens:
	// - "stop": API returned complete message or a message terminated by a stop sequence
	// - "length": Incomplete model output due to max_tokens parameter or token limit
	// - "function_call": The model decided to call a function (deprecated)
	// - "tool_calls": The model decided to call tools
	// - "content_filter": Omitted content due to a flag from content filters
	// - null: API response still in progress or incomplete
	// +optional
	FinishReason *FinishReason `json:"finish_reason,omitzero"`

	// LogProbs contains log probability information for the choice.
	// Only present if logprobs was set to true in the request.
	// NOTE: OpenAI will respond with `"logprobs": null`. This API will omit null logprobs.
	// +optional
	LogProbs *LogProbs `json:"logprobs,omitzero"`

	// ContentFilterResults contains any content filtering applied to this choice.
	// +optional
	ContentFilterResults *ContentFilterResults `json:"content_filter_results,omitzero"`
}

// ChatCompletionResponse represents a response structure for chat completion API.
// Returned by model based on the provided input.
type ChatCompletionResponse struct {
	// ID is a unique identifier for the chat completion.
	// +required
	ID string `json:"id"`

	// Object is the object type, which is always "chat.completion".
	// +required
	Object string `json:"object"`

	// Created is the Unix timestamp (in seconds) of when the chat completion was created.
	// +required
	Created int64 `json:"created"`

	// Model is the model used for the chat completion.
	// +required
	Model string `json:"model"`

	// Choices is a list of chat completion choices. Can be more than one if n>1.
	// +required
	Choices []ChatCompletionChoice `json:"choices"`

	// Usage provides token usage statistics for the completion request.
	// +optional
	Usage *Usage `json:"usage,omitzero"`

	// SystemFingerprint represents the backend configuration that the model runs with.
	// Can be used with the seed parameter to understand when backend changes have been made
	// that might impact determinism.
	// +optional
	SystemFingerprint string `json:"system_fingerprint,omitzero"`

	// ServiceTier indicates the service tier used for processing the request.
	// Can be "scale" or "default".
	// +optional
	ServiceTier string `json:"service_tier,omitzero"`

	// PromptFilterResults contains any content filtering applied to the prompts.
	// +optional
	PromptFilterResults []PromptFilterResult `json:"prompt_filter_results,omitzero"`

	// Unknown fields should be preserved to fully support the extended set of fields that backends such as vLLM support.
	Unknown jsontext.Value `json:",unknown"`
}

// PromptFilterResult contains information about content filtering applied to a particular prompt.
type PromptFilterResult struct {
	// Index is the index of the prompt that was filtered.
	// +required
	Index int `json:"index"`

	// ContentFilterResults contains details about the content filtering applied.
	// +required
	ContentFilterResults ContentFilterResults `json:"content_filter_results"`
}
