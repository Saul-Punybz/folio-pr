// Package config loads application configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds the full application configuration.
type Config struct {
	DB       DBConfig
	Server   ServerConfig
	S3       S3Config
	Ollama   OllamaConfig
	Telegram TelegramConfig
}

// DBConfig holds PostgreSQL connection parameters.
type DBConfig struct {
	Host    string
	Port    int
	User    string
	Pass    string
	DBName  string
	SSLMode string
}

// DSN returns a PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return "postgres://" + c.User + ":" + c.Pass +
		"@" + c.Host + ":" + strconv.Itoa(c.Port) +
		"/" + c.DBName + "?sslmode=" + c.SSLMode
}

// ServerConfig holds HTTP server parameters.
type ServerConfig struct {
	Port string
	Host string
}

// Addr returns the full listen address (host:port).
func (c ServerConfig) Addr() string {
	return c.Host + c.Port
}

// S3Config holds S3-compatible object storage parameters.
type S3Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
}

// OllamaConfig holds the Ollama LLM server parameters.
type OllamaConfig struct {
	Host          string
	InstructModel string
	EmbedModel    string
}

// TelegramConfig holds Telegram bot parameters.
type TelegramConfig struct {
	BotToken  string
	Allowlist string // format: "telegram_id:email,telegram_id:email"
}

// ParseAllowlist parses the TELEGRAM_ALLOWLIST string into a map of telegramID -> email.
func (c TelegramConfig) ParseAllowlist() map[int64]string {
	result := make(map[int64]string)
	if c.Allowlist == "" {
		return result
	}
	pairs := strings.Split(c.Allowlist, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		if err != nil {
			continue
		}
		result[id] = strings.TrimSpace(parts[1])
	}
	return result
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		DB: DBConfig{
			Host:    envOr("DB_HOST", "localhost"),
			Port:    envOrInt("DB_PORT", 5432),
			User:    envOr("DB_USER", "folio"),
			Pass:    envOr("DB_PASS", "folio"),
			DBName:  envOr("DB_NAME", "folio"),
			SSLMode: envOr("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			Port: envOr("SERVER_PORT", ":8080"),
			Host: envOr("SERVER_HOST", ""),
		},
		S3: S3Config{
			Endpoint:  envOr("S3_ENDPOINT", ""),
			Bucket:    envOr("S3_BUCKET", "folio-evidence"),
			AccessKey: envOr("S3_ACCESS_KEY", ""),
			SecretKey: envOr("S3_SECRET_KEY", ""),
			Region:    envOr("S3_REGION", "us-ashburn-1"),
		},
		Ollama: OllamaConfig{
			Host:          envOr("OLLAMA_HOST", "http://localhost:11434"),
			InstructModel: envOr("OLLAMA_INSTRUCT_MODEL", "llama3"),
			EmbedModel:    envOr("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		},
		Telegram: TelegramConfig{
			BotToken:  envOr("TELEGRAM_BOT_TOKEN", ""),
			Allowlist: envOr("TELEGRAM_ALLOWLIST", ""),
		},
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
