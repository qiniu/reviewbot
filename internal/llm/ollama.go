package llm

import (
	"errors"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

var ErrServerURLRequired = errors.New("server URL is required")
var ErrModelRequired = errors.New("model is required")

func initOllamaClient(config Config) (llms.Model, error) {
	if config.ServerURL == "" {
		return nil, ErrServerURLRequired
	}
	if config.Model == "" {
		return nil, ErrModelRequired
	}
	opts := []ollama.Option{
		ollama.WithServerURL(config.ServerURL),
		ollama.WithModel(config.Model),
	}
	m, err := ollama.New(opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}
