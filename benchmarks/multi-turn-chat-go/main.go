package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"multi-turn-chat-go/benchmark"
	"multi-turn-chat-go/tokenizer"
	"net/http"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var flags struct {
		config  string
		threads string
		format  string
	}

	flag.StringVar(&flags.config, "config", "", "Path to config file")
	flag.StringVar(&flags.threads, "threads", "", "Path to threads file")
	const (
		formatText = "text"
		formatJSON = "json"
	)
	flag.StringVar(&flags.format, "format", formatText, "Format of results")

	flag.Parse()

	if flags.config == "" {
		return errors.New("missing required flag: --config")
	}
	if flags.threads == "" {
		return errors.New("missing required flag: --threads")
	}

	switch flags.format {
	case "text", "json":
	default:
		return fmt.Errorf("invalid format: %q, must be %q or %q",
			flags.format, formatText, formatJSON)

	}

	var cfg Config
	if err := readJSON(flags.config, &cfg); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	openaiCfg := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	openaiCfg.BaseURL = os.Getenv("OPENAI_BASE_URL")
	if openaiCfg.BaseURL == "" {
		return fmt.Errorf("missing required environment variable: OPENAI_BASE_URL")
	}
	httpc := &http.Client{Timeout: time.Duration(cfg.RequestTimeout)}
	openaiCfg.HTTPClient = httpc
	client := openai.NewClientWithConfig(openaiCfg)

	var inputThreads []benchmark.InputThread
	if err := readJSON(flags.threads, &inputThreads); err != nil {
		return fmt.Errorf("reading input threads: %w", err)
	}

	tkn := &tokenizer.Tokenizer{
		Model: cfg.TokenizerModel,
		HTTPC: httpc,
		Port:  7000,
	}
	go func() {
		if err := tkn.Start(); err != nil {
			log.Fatalf("starting tokenizer: %v", err)
		}
	}()
	defer func() {
		if err := tkn.Stop(); err != nil {
			log.Printf("stopping tokenizer: %v", err)
		}
	}()

	waitCtx, cancelWaitCtx := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancelWaitCtx()
	if err := tkn.WaitForHealthy(waitCtx); err != nil {
		return fmt.Errorf("waiting for tokenizer to be healthy: %v", err)
	}

	runner := benchmark.New(client, tkn, cfg.Config, inputThreads)
	result, err := runner.Run()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	switch flags.format {
	case formatText:
		fmt.Println(result.String())
	case formatJSON:
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding output as json: %w", err)
		}
		fmt.Println(string(out))
	}

	return nil
}

func readJSON(path string, x interface{}) error {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening file %q: %w", path, err)
		}
		defer f.Close()
		r = f
	}

	if err := json.NewDecoder(r).Decode(x); err != nil {
		return fmt.Errorf("decoding as json: %w", err)
	}

	return nil
}

type Config struct {
	benchmark.Config
	RequestTimeout benchmark.Duration `json:"request_timeout"`
	TokenizerModel string             `json:"tokenizer_model"`
}

func (c Config) Validate() error {
	if c.RequestTimeout <= 0 {
		return errors.New("request_timeout must be greater than 0")
	}
	if c.TokenizerModel == "" {
		return errors.New("tokenizer_model must be specified")
	}
	return c.Config.Validate()
}
