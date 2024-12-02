package llm

import (
	"errors"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

var ErrAPIKeyRequired = errors.New("API key is required")

func initOpenAIClient(config Config) (llms.Model, error) {
	if config.APIKey == "" {
		return nil, ErrAPIKeyRequired
	}
	opts := []openai.Option{
		openai.WithModel(config.Model),
		openai.WithToken(config.APIKey),
		openai.WithBaseURL(config.ServerURL),
	}
	m, err := openai.New(opts...)
	if err != nil {
		return nil, err
	}
	return m, nil
}
