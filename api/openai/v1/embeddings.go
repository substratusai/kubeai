package v1

import (
	"errors"

	"github.com/go-json-experiment/json/jsontext"
)

var ErrVectorLengthMismatch = errors.New("vector length mismatch")

// Embedding is a special format of data representation that can be easily utilized by machine
// learning models and algorithms. The embedding is an information dense representation of the
// semantic meaning of a piece of text. Each embedding is a vector of floating point numbers,
// such that the distance between two embeddings in the vector space is correlated with semantic similarity
// between two inputs in the original format. For example, if two texts are similar,
// then their vector representations should also be similar.
type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingEncodingFormat is the format of the embeddings data.
// Currently, only "float" and "base64" are supported, however, "base64" is not officially documented.
// If not specified OpenAI will use "float".
type EmbeddingEncodingFormat string

const (
	EmbeddingEncodingFormatFloat  EmbeddingEncodingFormat = "float"
	EmbeddingEncodingFormatBase64 EmbeddingEncodingFormat = "base64"
)

type EmbeddingRequest struct {
	Input          any                     `json:"input"`
	Model          string                  `json:"model"`
	User           string                  `json:"user,omitzero"`
	EncodingFormat EmbeddingEncodingFormat `json:"encoding_format,omitzero"`
	// Dimensions The number of dimensions the resulting output embeddings should have.
	// Only supported in text-embedding-3 and later models.
	Dimensions int `json:"dimensions,omitzero"`

	// Unknown fields should be preserved to fully support the extended set of fields that backends such as vLLM support.
	Unknown jsontext.Value `json:",unknown"`
}

func (r *EmbeddingRequest) GetModel() string {
	return r.Model
}

func (r *EmbeddingRequest) SetModel(m string) {
	r.Model = m
}

// EmbeddingResponse is the response from a Create embeddings request.
type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  *Usage      `json:"usage,omitzero"`

	// Unknown fields should be preserved to fully support the extended set of fields that backends such as vLLM support.
	Unknown jsontext.Value `json:",unknown"`
}
