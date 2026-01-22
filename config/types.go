package config

import (
	"time"

	"github.com/go-playground/validator/v10"
)

type SecretKey string

const (
	SecretGithubPrivateKey SecretKey = "APP_GITHUB_PRIVATE_KEY"
	SecretGithubClientID   SecretKey = "GITHUB_CLIENT_ID"
	SecretOpenAIKey        SecretKey = "OPENAI_API_KEY"
)

type Config struct {
	// App
	Env           string        `split_words:"true" default:"prod" validate:"oneof=dev staging prod"`
	LogLevel      string        `split_words:"true" default:"info" validate:"oneof=debug info warn error"`
	ShutdownGrace time.Duration `split_words:"true" default:"15s" validate:"gt=0"`

	// Redis
	RedisURL string `split_words:"true" validate:"required"`

	// GitHub
	GithubPrivateKey string `envconfig:"APP_GITHUB_PRIVATE_KEY" validate:"required"`
	GithubClientID   string `split_words:"true" validate:"required"`

	// OPENAI
	OpenaiApiKey string `split_words:"true" validate:"required"`

	// Performance tuning
	WorkerCount       int           `split_words:"true" default:"5" validate:"gt=0"`
	GithubConcurrency int           `split_words:"true" default:"10" validate:"gt=0"`
	GithubRateLimit   int           `split_words:"true" default:"80" validate:"gt=0"`
	OpenaiRateLimit   int           `split_words:"true" default:"50" validate:"gt=0"`
	CacheSize         int           `split_words:"true" default:"1000" validate:"gt=0"`
	MessageTimeout    time.Duration `split_words:"true" default:"5m" validate:"gt=0"`

	// Redis tuning
	RedisStreamMaxLen int           `split_words:"true" default:"1000" validate:"gt=0"`
	RedisBlockTimeout time.Duration `split_words:"true" default:"1s" validate:"gt=0"`
	RedisBatchSize    int           `split_words:"true" default:"10" validate:"gt=0"`
	BackoffMin        time.Duration `split_words:"true" default:"100ms" validate:"gt=0"`
	BackoffMax        time.Duration `split_words:"true" default:"3s" validate:"gt=0"`
	HTTPClientTimeout time.Duration `split_words:"true" default:"30s" validate:"gt=0"`
	RedisConnTimeout  time.Duration `split_words:"true" default:"3s" validate:"gt=0"`
	MaxRetries        int           `split_words:"true" default:"3" validate:"gt=0"`
}

type Loader struct {
	Prefix   string
	Validate *validator.Validate
}
