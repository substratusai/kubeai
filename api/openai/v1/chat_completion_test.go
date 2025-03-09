package v1_test

import (
	"encoding/json"
	"fmt"
	"testing"

	easyjson "github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
	v1 "github.com/substratusai/kubeai/api/openai/v1"
)

func TestChatCompletionRequestPrefix(t *testing.T) {
	cases := []struct {
		input            string
		n                int
		exp              string
		expErrorContains []string
	}{
		{`{}`, 0, "", []string{"missing", "messages"}},
		{`{}`, 0, "", []string{"missing", "messages"}},
		{`{"messages": []}`, 0, "", []string{"empty"}},
		{`{"messages": []}`, 9, "", []string{"empty"}},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "abc"}]}`, 9, "abc", nil},
		{`{"messages": [{"role": "user", "content": "abcefghijk"}]}`, 9, "abcefghij", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 1, "世", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 2, "世界", nil},
		{`{"messages": [{"role": "user", "content": "世界"}]}`, 3, "世界", nil},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, "", nil},
		{`{"messages": [{"role": "user", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "abc", nil},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 0, "", nil},
		{`{"messages": [{"role": "system", "content": "abc"}, {"role": "user", "content": "xyz"}]}`, 9, "xyz", nil},
		{`{"messages": [{"role": "system", "content": "abc"}]}`, 9, "", []string{"no", "user", "found"}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q %d", c.input, c.n), func(t *testing.T) {
			var body v1.ChatCompletionRequest
			require.NoError(t, easyjson.Unmarshal([]byte(c.input), &body))
			require.Equal(t, c.exp, body.Prefix(c.n))
		})
	}
}

func TestChatCompletionRequest_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		req  *v1.ChatCompletionRequest
	}{
		{
			name: "empty request",
			json: `{"model":"","messages":null}`,
			req:  &v1.ChatCompletionRequest{},
		},
		{
			name: "extra field",
			json: `{"model":"","messages":null,"eexxttrraa":"val"}`,
			req:  &v1.ChatCompletionRequest{},
		},
		{
			name: "single content message",
			json: `{"model":"test-model","messages":[{"role":"user","content":"test-prefix"}],"extra":"val"}`,
			req: &v1.ChatCompletionRequest{
				Model: "test-model",
				Messages: []v1.ChatCompletionMessage{
					{
						Role:    "user",
						Content: &v1.ChatMessageContent{String: "test-prefix"},
					},
				},
			},
		},
		{
			name: "multi content message",
			json: `{
	"model":"test-model",
	"messages":[
		{
			"role":"user",
			"content": [
		        {"type": "text", "text": "What's in this image?"},
		        {
		            "type": "image_url",
		            "image_url": {
		            	"url": "https://example.com/image.jpg"
		            }
		        }
			]
		}
	],
	"extra":"val"
}`,
			req: &v1.ChatCompletionRequest{
				Model: "test-model",
				Messages: []v1.ChatCompletionMessage{
					{
						Role: "user",
						Content: &v1.ChatMessageContent{
							Array: []v1.ChatMessageContentPart{
								{Type: "text", Text: "What's in this image?"},
								{
									Type: "image_url",
									ImageURL: &v1.ChatMessageImageURL{
										URL: "https://example.com/image.jpg",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "all fields set",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "system",
				"content": "You are a helpful assistant."
			},
			{
				"role": "user",
				"name": "John",
				"content": "Hello!"
			},
			{
				"role": "assistant",
				"content": "Hi there!",
				"tool_calls": [
					{
						"id": "call_123",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
						}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_123",
				"content": "{\"temperature\":18,\"unit\":\"celsius\",\"description\":\"Partly cloudy\"}"
			}
		],
		"max_tokens": 100,
		"max_completion_tokens": 150,
		"temperature": 0.7,
		"top_p": 0.95,
		"n": 1,
		"stream": true,
		"stop": ["END", "STOP"],
		"presence_penalty": 0.5,
		"response_format": {
			"type": "json_object"
		},
		"seed": 42,
		"frequency_penalty": 0.8,
		"logit_bias": {"50256": -100},
		"logprobs": true,
		"top_logprobs": 3,
		"user": "user123",
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get the current weather in a location",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {
								"type": "string",
								"description": "The city and state, e.g. San Francisco, CA"
							},
							"unit": {
								"type": "string",
								"enum": ["celsius", "fahrenheit"]
							}
						},
						"required": ["location"]
					}
				}
			}
		],
		"tool_choice": {
			"type": "function",
			"function": {
				"name": "get_weather"
			}
		},
		"stream_options": {
			"include_usage": true
		},
		"parallel_tool_calls": true,
		"store": true,
		"reasoning_effort": "high",
		"metadata": {
			"user_id": "abc123",
			"conversation_id": "conv_456"
		}
	}`,
			req: &v1.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []v1.ChatCompletionMessage{
					{
						Role:    "system",
						Content: &v1.ChatMessageContent{String: "You are a helpful assistant."},
					},
					{
						Role:    "user",
						Name:    "John",
						Content: &v1.ChatMessageContent{String: "Hello!"},
					},
					{
						Role:    "assistant",
						Content: &v1.ChatMessageContent{String: "Hi there!"},
						ToolCalls: []v1.ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: v1.FunctionCall{
									Name:      "get_weather",
									Arguments: "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}",
								},
							},
						},
					},
					{
						Role:       "tool",
						ToolCallID: "call_123",
						Content:    &v1.ChatMessageContent{String: "{\"temperature\":18,\"unit\":\"celsius\",\"description\":\"Partly cloudy\"}"},
					},
				},
				MaxTokens:           100,
				MaxCompletionTokens: 150,
				Temperature:         0.7,
				TopP:                0.95,
				N:                   1,
				Stream:              true,
				Stop:                []string{"END", "STOP"},
				PresencePenalty:     0.5,
				ResponseFormat: &v1.ChatCompletionResponseFormat{
					Type: "json_object",
				},
				Seed:             func() *int { i := 42; return &i }(),
				FrequencyPenalty: 0.8,
				LogitBias:        map[string]int{"50256": -100},
				LogProbs:         true,
				TopLogProbs:      3,
				User:             "user123",
				Tools: []v1.Tool{
					{
						Type: "function",
						Function: &v1.FunctionDefinition{
							Name:        "get_weather",
							Description: "Get the current weather in a location",
							Parameters: map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "The city and state, e.g. San Francisco, CA",
									},
									"unit": map[string]interface{}{
										"type": "string",
										"enum": []interface{}{"celsius", "fahrenheit"},
									},
								},
								"required": []interface{}{"location"},
							},
						},
					},
				},
				ToolChoice: map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": "get_weather",
					},
				},
				StreamOptions: &v1.StreamOptions{
					IncludeUsage: true,
				},
				ParallelToolCalls: true,
				Store:             true,
				ReasoningEffort:   "high",
				Metadata: map[string]string{
					"user_id":         "abc123",
					"conversation_id": "conv_456",
				},
			},
		},
		{
			name: "auto tool choice",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user",
				"content": "What's the weather like in San Francisco?"
			}
		],
		"tool_choice": "auto",
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get the current weather in a location",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {
								"type": "string",
								"description": "The city and state, e.g. San Francisco, CA"
							}
						},
						"required": ["location"]
					}
				}
			}
		]
	}`,
			req: nil,
		},
		{
			name: "none tool choice",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user",
				"content": "Just respond normally without using any tools."
			}
		],
		"tool_choice": "none",
		"tools": [
			{
				"type": "function",
				"function": {
					"name": "get_weather",
					"description": "Get the current weather in a location",
					"parameters": {
						"type": "object",
						"properties": {
							"location": {
								"type": "string",
								"description": "The city and state, e.g. San Francisco, CA"
							}
						},
						"required": ["location"]
					}
				}
			}
		]
	}`,
		},
		{
			name: "multiple tool calls",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user",
				"content": "What's the weather like in San Francisco and New York?"
			},
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_sf123",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"location\":\"San Francisco\"}"
						}
					},
					{
						"id": "call_ny456",
						"type": "function",
						"function": {
							"name": "get_weather",
							"arguments": "{\"location\":\"New York\"}"
						}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_sf123",
				"content": "{\"temperature\":18,\"unit\":\"celsius\",\"description\":\"Sunny\"}"
			},
			{
				"role": "tool",
				"tool_call_id": "call_ny456",
				"content": "{\"temperature\":12,\"unit\":\"celsius\",\"description\":\"Cloudy\"}"
			}
		]
	}`,
		},
		{
			name: "request with vision model",
			json: `{
		"model": "gpt-4-vision",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "text",
						"text": "What's in this image?"
					},
					{
						"type": "image_url",
						"image_url": {
							"url": "https://example.com/image.jpg",
							"detail": "high"
						}
					},
					{
						"type": "image_url",
						"image_url": {
							"url": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEASABIAAD...",
							"detail": "low"
						}
					}
				]
			}
		],
		"max_tokens": 300
	}`,
		},
		{
			name: "request with image captions",
			json: `{
		"model": "gpt-4-vision",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "text",
						"text": "Describe these images"
					},
					{
						"type": "image_url",
						"image_url": {
							"url": "https://example.com/image1.jpg",
							"detail": "high"
						}
					},
					{
						"type": "image_url",
						"image_url": {
							"url": "https://example.com/image2.jpg"
						}
					}
				]
			}
		],
		"response_format": {"type": "text"}
	}`,
		},
		{
			name: "function calling request",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user",
				"content": "What's the weather in Boston and San Francisco?"
			}
		],
		"functions": [
			{
				"name": "get_weather",
				"description": "Get the current weather in a location",
				"parameters": {
					"type": "object",
					"properties": {
						"location": {
							"type": "string",
							"description": "The city and state, e.g. San Francisco, CA"
						}
					},
					"required": ["location"]
				}
			}
		]
	}`,
		},
		{
			name: "logprobs with top_logprobs",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user",
				"content": "Generate a random sentence."
			}
		],
		"logprobs": true,
		"top_logprobs": 5
	}`,
		},
		{
			name: "request with system_fingerprint",
			json: `{
		"model": "gpt-4-0125-preview",
		"messages": [
			{
				"role": "user", 
				"content": "Hello"
			}
		],
		"system_fingerprint": "fp_44709d6fcb"
	}`,
		},
		{
			name: "request with dimensions in response format",
			json: `{
		"model": "gpt-4",
		"messages": [
			{
				"role": "user", 
				"content": "Generate something"
			}
		],
		"response_format": {
			"type": "json_object",
			"json_schema": {
				"name": "something",
				"schema": {
					"abc": {
						"type": "array",
						"items": {"type": "number"},
						"dimensions": 1536
					}
				}
			}
		}
	}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var req v1.ChatCompletionRequest
			err := easyjson.Unmarshal([]byte(c.json), &req)
			require.NoError(t, err, "unmarshal error")

			if c.req != nil {
				proxy := req.UnknownFieldsProxy
				req.UnknownFieldsProxy = easyjson.UnknownFieldsProxy{}
				// Assert on equality without the unknown fields.
				require.EqualValues(t, *c.req, req, "expected struct values")
				req.UnknownFieldsProxy = proxy
			}

			jsn, err := easyjson.Marshal(req)
			require.NoError(t, err, "marshal error")
			require.JSONEq(t, c.json, string(jsn), "expected round-trip JSON")
		})
	}
}

func TestChatCompletionRequest_InvalidMessageContent(t *testing.T) {
	validArray := v1.ChatMessageContent{
		Array: []v1.ChatMessageContentPart{
			{Type: "text", Text: "test"},
			{Type: "image_url", ImageURL: &v1.ChatMessageImageURL{
				URL: "https://example.com/image.jpg",
			}},
		},
	}
	_, err := easyjson.Marshal(validArray)
	require.NoError(t, err)

	validString := v1.ChatMessageContent{
		String: "test",
	}
	_, err = easyjson.Marshal(validString)
	require.NoError(t, err)

	invalidBoth := v1.ChatMessageContent{
		Array: []v1.ChatMessageContentPart{
			{Type: "text", Text: "test"},
		},
		String: "test",
	}
	_, err = easyjson.Marshal(invalidBoth)
	require.ErrorContains(t, err, "String and Array cannot be specified at the same time")
}

func TestChatCompletionResponse_JSON(t *testing.T) {
	cases := []struct {
		name string
		json string
		resp *v1.ChatCompletionResponse
	}{
		{
			name: "empty response",
			json: `{"object":"","model":"","choices":null}`,
			resp: &v1.ChatCompletionResponse{},
		},
		{
			name: "basic response",
			json: `{
				"id": "chatcmpl-123",
				"object": "chat.completion",
				"created": 1677858242,
				"model": "gpt-3.5-turbo-0613",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "This is a test response."
						},
						"finish_reason": "stop"
					}
				]
			}`,
			resp: &v1.ChatCompletionResponse{
				ID:      "chatcmpl-123",
				Object:  "chat.completion",
				Created: 1677858242,
				Model:   "gpt-3.5-turbo-0613",
				Choices: []v1.ChatCompletionChoice{
					{
						Index: 0,
						Message: v1.ChatCompletionMessage{
							Role:    "assistant",
							Content: &v1.ChatMessageContent{String: "This is a test response."},
						},
						FinishReason: func() *v1.FinishReason { r := v1.FinishReasonStop; return &r }(),
					},
				},
			},
		},
		{
			name: "with function call",
			json: `{
				"id": "chatcmpl-456",
				"object": "chat.completion",
				"created": 1699807964,
				"model": "gpt-4-1106-preview",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "",
							"tool_calls": [
								{
									"id": "call_abc123",
									"type": "function",
									"function": {
										"name": "get_weather",
										"arguments": "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}"
									}
								}
							]
						},
						"finish_reason": "tool_calls"
					}
				],
				"usage": {
					"prompt_tokens": 82,
					"completion_tokens": 29,
					"total_tokens": 111
				},
				"system_fingerprint": "fp_44709d6fcb"
			}`,
			resp: &v1.ChatCompletionResponse{
				ID:      "chatcmpl-456",
				Object:  "chat.completion",
				Created: 1699807964,
				Model:   "gpt-4-1106-preview",
				Choices: []v1.ChatCompletionChoice{
					{
						Index: 0,
						Message: v1.ChatCompletionMessage{
							Role:    "assistant",
							Content: &v1.ChatMessageContent{String: ""},
							ToolCalls: []v1.ToolCall{
								{
									ID:   "call_abc123",
									Type: "function",
									Function: v1.FunctionCall{
										Name:      "get_weather",
										Arguments: "{\"location\":\"San Francisco\",\"unit\":\"celsius\"}",
									},
								},
							},
						},
						FinishReason: func() *v1.FinishReason { r := v1.FinishReasonToolCalls; return &r }(),
					},
				},
				Usage: &v1.Usage{
					PromptTokens:     82,
					CompletionTokens: 29,
					TotalTokens:      111,
				},
				SystemFingerprint: "fp_44709d6fcb",
			},
		},
		{
			name: "response with filter results",
			json: `{
				"id": "chatcmpl-789",
				"object": "chat.completion",
				"created": 1697725363,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "I cannot provide information about that topic."
						},
						"finish_reason": "stop",
						"content_filter_results": {
							"hate": {
								"filtered": true,
								"severity": "high"
							},
							"self_harm": {
								"filtered": false,
								"severity": "low"
							}
						}
					}
				],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 12,
					"total_tokens": 32
				},
				"prompt_filter_results": [
					{
						"index": 0,
						"content_filter_results": {
							"hate": {
								"filtered": false,
								"severity": "low"
							},
							"self_harm": {
								"filtered": false,
								"severity": "low"
							}
						}
					}
				]
			}`,
			resp: &v1.ChatCompletionResponse{
				ID:      "chatcmpl-789",
				Object:  "chat.completion",
				Created: 1697725363,
				Model:   "gpt-4",
				Choices: []v1.ChatCompletionChoice{
					{
						Index: 0,
						Message: v1.ChatCompletionMessage{
							Role:    "assistant",
							Content: &v1.ChatMessageContent{String: "I cannot provide information about that topic."},
						},
						FinishReason: func() *v1.FinishReason { r := v1.FinishReasonStop; return &r }(),
						ContentFilterResults: &v1.ContentFilterResults{
							Hate: &v1.Hate{
								Filtered: true,
								Severity: "high",
							},
							SelfHarm: &v1.SelfHarm{
								Filtered: false,
								Severity: "low",
							},
						},
					},
				},
				Usage: &v1.Usage{
					PromptTokens:     20,
					CompletionTokens: 12,
					TotalTokens:      32,
				},
				PromptFilterResults: []v1.PromptFilterResult{
					{
						Index: 0,
						ContentFilterResults: v1.ContentFilterResults{
							Hate: &v1.Hate{
								Filtered: false,
								Severity: "low",
							},
							SelfHarm: &v1.SelfHarm{
								Filtered: false,
								Severity: "low",
							},
						},
					},
				},
			},
		},
		{
			name: "response with logprobs",
			json: `{
				"id": "chatcmpl-logprobs123",
				"object": "chat.completion",
				"created": 1700000000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "Hello world"
						},
						"logprobs": {
							"content": [
								{
									"token": "Hello",
									"logprob": -0.5,
									"top_logprobs": [
										{
											"token": "Hello",
											"logprob": -0.5
										},
										{
											"token": "Hi",
											"logprob": -1.2
										}
									]
								},
								{
									"token": " world",
									"logprob": -0.8,
									"top_logprobs": [
										{
											"token": " world",
											"logprob": -0.8
										},
										{
											"token": " there",
											"logprob": -1.5
										}
									]
								}
							]
						},
						"finish_reason": "stop"
					}
				]
			}`,
		},
		{
			name: "multiple choices response",
			json: `{
				"id": "chatcmpl-multiplechoices",
				"object": "chat.completion",
				"created": 1700010000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "First response option"
						},
						"finish_reason": "stop"
					},
					{
						"index": 1,
						"message": {
							"role": "assistant",
							"content": "Second response option"
						},
						"finish_reason": "stop"
					},
					{
						"index": 2,
						"message": {
							"role": "assistant",
							"content": "Third response option"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 15,
					"completion_tokens": 30,
					"total_tokens": 45
				}
			}`,
		},
		{
			name: "response with length finish reason",
			json: `{
				"id": "chatcmpl-length123",
				"object": "chat.completion",
				"created": 1700020000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "This is a response that reached the maximum token limit..."
						},
						"finish_reason": "length"
					}
				],
				"usage": {
					"prompt_tokens": 10,
					"completion_tokens": 100,
					"total_tokens": 110
				}
			}`,
		},
		{
			name: "response with content moderation",
			json: `{
				"id": "chatcmpl-moderation123",
				"object": "chat.completion",
				"created": 1700030000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": ""
						},
						"finish_reason": "content_filter",
						"content_filter_results": {
							"hate": {
								"filtered": true,
								"severity": "high"
							},
							"self_harm": {
								"filtered": false,
								"severity": "low"
							},
							"sexual": {
								"filtered": false,
								"severity": "low"
							},
							"violence": {
								"filtered": true,
								"severity": "medium"
							}
						}
					}
				],
				"usage": {
					"prompt_tokens": 25,
					"completion_tokens": 0,
					"total_tokens": 25
				}
			}`,
		},
		{
			name: "response with JSON format",
			json: `{
				"id": "chatcmpl-json123",
				"object": "chat.completion",
				"created": 1700040000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "{\"name\":\"John Doe\",\"age\":30,\"city\":\"New York\",\"skills\":[\"programming\",\"design\",\"writing\"]}"
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 25,
					"total_tokens": 45
				}
			}`,
		},
		{
			name: "response with function call (deprecated format)",
			json: `{
				"id": "chatcmpl-function123",
				"object": "chat.completion",
				"created": 1700050000,
				"model": "gpt-4",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "",
							"function_call": {
								"name": "get_weather",
								"arguments": "{\"location\":\"Paris\",\"unit\":\"celsius\"}"
							}
						},
						"finish_reason": "function_call"
					}
				],
				"usage": {
					"prompt_tokens": 30,
					"completion_tokens": 20,
					"total_tokens": 50
				}
			}`,
		},
		{
			name: "response with vision content",
			json: `{
				"id": "chatcmpl-vision123",
				"object": "chat.completion",
				"created": 1700060000,
				"model": "gpt-4-vision",
				"choices": [
					{
						"index": 0,
						"message": {
							"role": "assistant",
							"content": "The image shows a scenic landscape with mountains and a lake."
						},
						"finish_reason": "stop"
					}
				],
				"usage": {
					"prompt_tokens": 150,
					"completion_tokens": 15,
					"total_tokens": 165
				}
			}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.True(t, json.Valid([]byte(c.json)), "test case should be valid json")

			var resp v1.ChatCompletionResponse
			err := easyjson.Unmarshal([]byte(c.json), &resp)
			require.NoError(t, err, "unmarshal error")

			if c.resp != nil {
				proxy := resp.UnknownFieldsProxy
				resp.UnknownFieldsProxy = easyjson.UnknownFieldsProxy{}
				// Assert on equality without the unknown fields.
				require.EqualValues(t, *c.resp, resp, "expected struct values")
				resp.UnknownFieldsProxy = proxy
			}

			jsn, err := easyjson.Marshal(resp)
			require.NoError(t, err, "marshal error")
			require.JSONEq(t, c.json, string(jsn), "expected round-trip JSON")
		})
	}
}
