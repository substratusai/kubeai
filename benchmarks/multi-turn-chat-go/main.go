package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"multi-turn-chat-go/benchmark"
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

	var runnerCfg benchmark.Config
	if err := readJSON(flags.config, &runnerCfg); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	if err := runnerCfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	openaiCfg := openai.DefaultConfig(os.Getenv("OPENAI_API_KEY"))
	openaiCfg.BaseURL = os.Getenv("OPENAI_BASE_URL")
	if openaiCfg.BaseURL == "" {
		return fmt.Errorf("missing required environment variable: OPENAI_BASE_URL")
	}
	openaiCfg.HTTPClient = &http.Client{Timeout: time.Duration(runnerCfg.RequestTimeout)}
	client := openai.NewClientWithConfig(openaiCfg)

	var inputThreads []benchmark.InputThread
	if err := readJSON(flags.threads, &inputThreads); err != nil {
		return fmt.Errorf("reading input threads: %w", err)
	}

	runner := benchmark.New(client, runnerCfg, inputThreads)
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
