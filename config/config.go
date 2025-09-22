package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
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
}

type Loader struct {
	Prefix   string
	Validate *validator.Validate
}

func NewLoader(prefix string) *Loader {
	v := validator.New()
	return &Loader{Prefix: prefix, Validate: v}
}

func (l *Loader) Load() (Config, error) {
	var cfg Config

	if err := loadDotEnv(); err != nil {
		log.Printf("dotenv: %v", err)
	}
	if err := envconfig.Process(l.Prefix, &cfg); err != nil {
		return cfg, fmt.Errorf("env load: %w", err)
	}

	if err := l.Validate.Struct(cfg); err != nil {
		return cfg, fmt.Errorf("config validation: %w", err)
	}

	log.Printf("config loaded env=%s logLevel=%s redisURL_set=%t",
		cfg.Env, cfg.LogLevel, cfg.RedisURL != "")

	return cfg, nil
}

func loadDotEnv() error {
	files := []string{".env"}

	if appEnv := strings.TrimSpace(os.Getenv("APP_ENV")); appEnv != "" {
		files = append(files, ".env."+appEnv)
	}
	if goEnv := strings.TrimSpace(os.Getenv("GO_ENV")); goEnv != "" && goEnv != os.Getenv("APP_ENV") {
		files = append(files, ".env."+goEnv)
	}

	var loadedAny bool
	for _, f := range files {
		if fileExists(f) {
			if err := godotenv.Overload(f); err != nil {
				log.Printf("dotenv: failed loading %s: %v", f, err)
				continue
			}
			loadedAny = true
		}
	}

	if !loadedAny {
		return fmt.Errorf("no .env files found (looked for: %s)", strings.Join(files, ", "))
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fetchSecret(key SecretKey) (string, error) {
	val := strings.TrimSpace(os.Getenv(string(key)))
	if val == "" {
		return "", fmt.Errorf("secret %q not found", key)
	}
	return val, nil
}

func FetchSecretByName(secret SecretKey) (string, error) {
	return fetchSecret(SecretKey(secret))
}
