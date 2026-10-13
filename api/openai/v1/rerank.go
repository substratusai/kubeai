package v1

import (
	"encoding/json/jsontext"
)

// RerankRequest represents a request to rerank a list of documents based on a query.
type RerankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	Model     string   `json:"model"`
	TopN      int      `json:"top_n,omitzero"`

	// Unknown fields should be preserved to fully support engines that accept
	// extended parameters.
	Unknown jsontext.Value `json:",embed"`
}

func (r *RerankRequest) GetModel() string  { return r.Model }
func (r *RerankRequest) SetModel(m string) { r.Model = m }

// RerankResult contains a single reranked document and its relevance score.
type RerankResult struct {
	DocumentIndex  int     `json:"document_index"`
	Document       string  `json:"document,omitzero"`
	RelevanceScore float32 `json:"relevance_score"`
}

// RerankResponse represents the response from a reranking request.
type RerankResponse struct {
	Object string         `json:"object"`
	Data   []RerankResult `json:"data"`
	Model  string         `json:"model"`

	// Unknown fields should be preserved to fully support engines that return
	// extended parameters.
	Unknown jsontext.Value `json:",embed"`
}
