package vllmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	HTTPClient *http.Client
}

type LoadAdapterRequest struct {
	LoraName string `json:"lora_name"`
	LoraPath string `json:"lora_path"`
}

// Load a LoRa adapter into the VLLM model server.
// See: https://docs.vllm.ai/en/latest/models/lora.html#dynamically-serving-lora-adapters
func (c *Client) LoadLoraAdapter(ctx context.Context, addr string, req LoadAdapterRequest) error {
	if err := c.post(ctx, addr, "/v1/load_lora_adapter", req, nil); err != nil {
		return err
	}
	return nil
}

type UnloadAdapterRequest struct {
	LoraName string `json:"lora_name"`
}

// Unload a LoRa adapter from the VLLM model server.
// See: https://docs.vllm.ai/en/latest/models/lora.html#dynamically-serving-lora-adapters
func (c *Client) UnloadLoraAdapter(ctx context.Context, addr string, req UnloadAdapterRequest) error {
	if err := c.post(ctx, addr, "/v1/unload_lora_adapter", req, nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) post(ctx context.Context, addr string, path string, req, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshalling body as json: %w", err)
	}

	url := addr + path
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending http request: POST %s: %w", url, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode > 299 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("unexpected status code: POST %s: %d: %s", url, httpResp.StatusCode, string(respBody))
	}

	if resp != nil {
		if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
			return fmt.Errorf("decoding response body: %w", err)
		}
	}

	return nil
}
