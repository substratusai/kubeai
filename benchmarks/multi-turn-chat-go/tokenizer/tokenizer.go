package tokenizer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type Tokenizer struct {
	Model string
	HTTPC *http.Client
	Port  int
	cmd   *exec.Cmd
}

func (t *Tokenizer) Start() error {
	// Run the process:
	// TOKENIZER_MODEL=gpt2 ./.venv/bin/fastapi run tokens.py --port 7000

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("runtime.Caller failed")
	}
	dir := filepath.Dir(filename)

	t.cmd = exec.Command("uvicorn",
		"--app-dir="+filepath.Join(dir), "tokens:app",
		"--log-level", "error", "--port", strconv.Itoa(t.Port),
	)
	// Copy env and add model env
	t.cmd.Env = append(os.Environ(), "TOKENIZER_MODEL="+t.Model)
	t.cmd.Stdout = os.Stdout
	t.cmd.Stderr = os.Stderr
	return t.cmd.Start()
}

func (t *Tokenizer) WaitForHealthy(ctx context.Context) error {
	// Wait until context is cancelled and try every 1 second.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			if err := t.sendHealthcheck(ctx); err != nil {
				log.Printf("Tokenzier healthcheck failed: %v", err)
				continue
			} else {
				return nil
			}
		}
	}
}

func (t *Tokenizer) sendHealthcheck(ctx context.Context) error {
	// curl localhost:7000/healthz
	url := fmt.Sprintf("http://localhost:%d/healthz", t.Port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	res, err := t.HTTPC.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck failed: %d", res.StatusCode)
	}
	return nil
}

func (t *Tokenizer) Stop() error {
	// Stop the process:
	// kill -9 <pid>
	return t.cmd.Process.Kill()
}

func (t *Tokenizer) CountTokens(text string) (int, error) {
	// POST localhost:7000/tokens {"text": "..."}
	payload := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	btys, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/tokens", t.Port), bytes.NewReader(btys))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := t.HTTPC.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	var response struct {
		NumTokens int `json:"num_tokens"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, err
	}

	return response.NumTokens, nil
}
