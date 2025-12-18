package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	WebSocket WebSocketConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Email     EmailConfig
	App       AppConfig
	Payment   PaymentConfig
	Worker    WorkerConfig
}

type ServerConfig struct {
	Host string
	Port string
	Env  string
}
type WebSocketConfig struct {
	AllowedOrigins  string
	ReadBufferSize  int
	WriteBufferSize int
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

type AppConfig struct {
	BcryptCost         int
	RateLimitRequests  int
	RateLimitWindow    time.Duration
	CORSAllowedOrigins []string
}

type PaymentConfig struct {
	CompanyBTCWallet   string
	CompanyETHWallet   string
	CompanyUSDTWallet  string
	CompanyBankName    string
	CompanyBankAccount string
	CompanyBankIBAN    string
	CompanyBankSWIFT   string
}

type WorkerConfig struct {
	RateUpdateInterval time.Duration
	RateUpdateTimeout  time.Duration
	RateUpdateRetries  int
	RateRetryBackoff   time.Duration
}

func (c *Config) GetDSN() string {
	if c.Database.Host == "postgres" {
		return fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s sslmode=%s",
			c.Database.Host,
			c.Database.User,
			c.Database.Password,
			c.Database.DBName,
			c.Database.SSLMode,
		)
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
			Env:  getEnv("ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "exchange"),
			Password: getEnv("DB_PASSWORD", "exchange_password"),
			DBName:   getEnv("DB_NAME", "exchange_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret:             getEnv("JWT_SECRET", ""),
			AccessTokenExpiry:  parseDuration(getEnv("JWT_ACCESS_TOKEN_EXPIRY", "15m"), 15*time.Minute),
			RefreshTokenExpiry: parseDuration(getEnv("JWT_REFRESH_TOKEN_EXPIRY", "168h"), 168*time.Hour),
		},
		Email: EmailConfig{
			SMTPHost:     getEnv("SMTP_HOST", ""),
			SMTPPort:     parseInt(getEnv("SMTP_PORT", "587"), 587),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			SMTPFrom:     getEnv("SMTP_FROM", "noreply@caspianex.com"),
		},
		App: AppConfig{
			BcryptCost:         parseInt(getEnv("BCRYPT_COST", "10"), 10),
			RateLimitRequests:  parseInt(getEnv("RATE_LIMIT_REQUESTS", "100"), 100),
			RateLimitWindow:    parseDuration(getEnv("RATE_LIMIT_WINDOW", "1m"), 1*time.Minute),
			CORSAllowedOrigins: parseStringSlice(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		},
		Payment: PaymentConfig{
			CompanyBTCWallet:   getEnv("COMPANY_BTC_WALLET", ""),
			CompanyETHWallet:   getEnv("COMPANY_ETH_WALLET", ""),
			CompanyUSDTWallet:  getEnv("COMPANY_USDT_WALLET", ""),
			CompanyBankName:    getEnv("COMPANY_BANK_NAME", "Kaspi Bank"),
			CompanyBankAccount: getEnv("COMPANY_BANK_ACCOUNT", ""),
			CompanyBankIBAN:    getEnv("COMPANY_BANK_IBAN", ""),
			CompanyBankSWIFT:   getEnv("COMPANY_BANK_SWIFT", ""),
		},
		WebSocket: WebSocketConfig{
			AllowedOrigins:  getEnv("WEBSOCKET_ALLOWED_ORIGINS", "http://localhost:5173"),
			ReadBufferSize:  parseInt(getEnv("WEBSOCKET_READ_BUFFER_SIZE", "1024"), 1024),
			WriteBufferSize: parseInt(getEnv("WEBSOCKET_WRITE_BUFFER_SIZE", "1024"), 1024),
		},
		Worker: WorkerConfig{
			RateUpdateInterval: parseDuration(getEnv("WORKER_RATE_UPDATE_INTERVAL", "2m"), 2*time.Minute),
			RateUpdateTimeout:  parseDuration(getEnv("WORKER_RATE_UPDATE_TIMEOUT", "30s"), 30*time.Second),
			RateUpdateRetries:  parseInt(getEnv("WORKER_RATE_UPDATE_RETRIES", "3"), 3),
			RateRetryBackoff:   parseDuration(getEnv("WORKER_RATE_RETRY_BACKOFF", "5s"), 5*time.Second),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(value string, defaultValue int) int {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return defaultValue
}

func parseDuration(value string, defaultValue time.Duration) time.Duration {
	if d, err := time.ParseDuration(value); err == nil {
		return d
	}
	return defaultValue
}

func parseStringSlice(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
