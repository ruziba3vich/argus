package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type (
	// Config -.
	Config struct {
		App           App
		Environment   string
		Server        Server
		Context       Context
		HTTP          HTTP
		GRPC          GRPC
		Log           Log
		PG            PG
		DB            DB
		RMQ           RMQ
		Redis         Redis
		Email         EmailConfig
		Token         Token
		OTLPCollector WebAddress
		MinIO         MinIO
		Multicard     Multicard
		Validation    Validation
		TelegramBot   TelegramBot
		Eskiz         Eskiz
	}

	Server struct {
		Host         string `json:"HOST"`
		Port         string `json:"PORT"`
		ReadTimeout  string `json:"readTimeout"`
		WriteTimeout string `json:"writeTimeout"`
		IdleTimeout  string `json:"idleTimeout"`
	}

	// App -.
	App struct {
		Name    string `env:"APP_NAME,required"`
		Version string `env:"APP_VERSION,required"`
	}

	// HTTP -.
	HTTP struct {
		Port string `env:"HTTP_PORT,required"`
	}

	Context struct {
		Timeout string
	}

	// GRPC -.
	GRPC struct {
		Port string `env:"GRPC_PORT" envDefault:"50051"`
	}

	// Log -.
	Log struct {
		Level string `env:"LOG_LEVEL,required"`
	}

	// PG -.
	PG struct {
		PoolMax int    `env:"PG_POOL_MAX,required"`
		URL     string `env:"PG_URL,required"`
	}

	DB struct {
		Host     string
		Port     string
		Name     string
		User     string
		Password string
		SSLMode  string
	}

	// RMQ -.
	RMQ struct {
		ServerExchange string `env:"RMQ_RPC_SERVER,required"`
		ClientExchange string `env:"RMQ_RPC_CLIENT,required"`
		URL            string `env:"RMQ_URL,required"`
	}

	// EmailConfig -.
	EmailConfig struct {
		From     string `env:"EMAIL_FROM,required"`
		Password string `env:"EMAIL_PASSWORD,required"`
		Host     string `env:"EMAIL_HOST,required"`
		Port     int    `env:"EMAIL_PORT,required"`
	}

	Redis struct {
		Host     string
		Port     string
		Password string
		Name     string
	}

	WebAddress struct {
		Host string
		Port string
	}

	Token struct {
		SigningKey                 string `env:"SIGNING_KEY"`
		AccessTokenExpirationTime  time.Duration
		RefreshTokenExpirationTime time.Duration
	}

	MinIO struct {
		Endpoint   string `env:"MINIO_ENDPOINT,required"`
		AccessKey  string `env:"MINIO_ACCESS_KEY,required"`
		SecretKey  string `env:"MINIO_SECRET_KEY,required"`
		BucketName string `env:"MINIO_BUCKET_NAME,required"`
	}

	// Multicard
	// MULTICARD_AGGR_APPLiCATION_ID=rhmt_test
	// MULTICARD_AGGR_SECRET=Pw18axeBFo8V7NamKHXX
	// MULTICARD_AGGR_API_URL_TEST=https://dev-mesh.multicard.uz/
	// MULTICARD_AGGR_API_URL_PROD=https://mesh.multicard.uz/
	// MULTICARD_AGGR_STORE_ID=6
	// MULTICARD_AGGR_CARD=8600303655375959
	// MULTICARD_AGGR_CARD_WALID_DATE=03/26
	MulticardAggr struct {
		ApplicationID string `env:"MULTICARD_AGGR_APPLiCATION_ID,required"`
		Secret        string `env:"MULTICARD_AGGR_SECRET,required"`
		APIURLTest    string `env:"MULTICARD_AGGR_API_URL_TEST,required"`
		APIURLProd    string `env:"MULTICARD_AGGR_API_URL_PROD,required"`
		StoreID       string `env:"MULTICARD_AGGR_STORE_ID,required"`
		Card          string `env:"MULTICARD_AGGR_CARD,required"`
		CardValidDate string `env:"MULTICARD_AGGR_CARD_WALID_DATE,required"`
	}

	MulticardCard struct {
		ApplicationID string `env:"MULTICARD_CARD_APPLiCATION_ID,required"`
		Secret        string `env:"MULTICARD_CARD_SECRET,required"`
		APIURLTest    string `env:"MULTICARD_CARD_API_URL_TEST,required"`
		APIURLProd    string `env:"MULTICARD_CARD_API_URL_PROD,required"`
		Card          string `env:"MULTICARD_CARD_CARD,required"`
		CardToken     string `env:"MULTICARD_CARD_CARD_TOKEN,required"`
	}

	Multicard struct {
		Aggr MulticardAggr
		Card MulticardCard
	}

	Validation struct {
		ApiKey  string `env:"VALIDATION_API_KEY,required"`
		BaseURL string `env:"VALIDATION_BASE_URL,required"`
	}

	// Telegram bot -.
	TelegramBot struct {
		Token    string `env:"TELEGRAM_BOT_TOKEN,required"`
		Username string `env:"TELEGRAM_BOT_USERNAME,required"`
	}

	// Eskiz
	Eskiz struct {
		Email      string `env:"ESKIZ_EMAIL,required"`
		SecretCode string `env:"ESKIZ_SECRET_CODE,required"`
		APIURL     string `env:"ESKIZ_API_URL,required"`
	}
)

func NewConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	var config Config

	// general configuration
	config.App.Name = getEnv("APP", "app")
	config.Environment = getEnv("ENVIRONMENT", "develop")
	config.Log.Level = getEnv("LOG_LEVEL", "debug")
	config.Context.Timeout = getEnv("CONTEXT_TIMEOUT", "30s")

	// server configuration
	config.Server.Host = getEnv("SERVER_HOST", "0.0.0.0")
	config.Server.Port = getEnv("SERVER_PORT", ":8080")
	config.Server.ReadTimeout = getEnv("SERVER_READ_TIMEOUT", "10s")
	config.Server.WriteTimeout = getEnv("SERVER_WRITE_TIMEOUT", "10s")
	config.Server.IdleTimeout = getEnv("SERVER_IDLE_TIMEOUT", "120s")

	// db configuration
	config.DB.Host = getEnv("POSTGRES_HOST", "108.181.201.147")
	config.DB.Port = getEnv("POSTGRES_PORT", "5433")
	config.DB.Name = getEnv("POSTGRES_DATABASE", "sugurta")
	config.DB.User = getEnv("POSTGRES_USER", "sugurta")
	config.DB.Password = getEnv("POSTGRES_PASSWORD", "v8Qe96csIhZZ")
	config.DB.SSLMode = getEnv("POSTGRES_SSLMODE", "disable")

	config.PG.PoolMax = cast.ToInt(getEnv("POSTGRES_POOL_MAX", "1"))

	// redis configuration
	config.Redis.Host = getEnv("REDIS_HOST", "108.181.201.147")
	config.Redis.Port = getEnv("REDIS_PORT", "6379")
	config.Redis.Password = getEnv("REDIS_PASSWORD", "97ZF8bKFpvwx")
	config.Redis.Name = getEnv("REDIS_DATABASE", "0")

	config.Email.From = getEnv("EMAIL_FROM", "your_email")
	config.Email.Password = getEnv("EMAIL_PASSWORD", "your_email")
	config.Email.Port = cast.ToInt(getEnv("EMAIL_PORT", "587"))
	config.Email.Host = getEnv("EMAIL_HOST", "smtp.gmail.com")

	config.Token.SigningKey = getEnv("SIGNING_KEY", "21d2d3rf324rdf34d2r34sw!@%!@N!I$@")
	config.Token.AccessTokenExpirationTime = time.Minute * 60 * 24 * 7
	config.Token.RefreshTokenExpirationTime = time.Minute * 60 * 24 * 7 * 30

	// otlp collector configuration
	config.OTLPCollector.Host = getEnv("OTLP_COLLECTOR_HOST", "localhost")
	config.OTLPCollector.Port = getEnv("OTLP_COLLECTOR_PORT", ":4317")

	// minio configuration
	config.MinIO.Endpoint = getEnv("MINIO_ENDPOINT", "localhost:9000")
	config.MinIO.AccessKey = getEnv("MINIO_ACCESS_KEY", "minioadmin")
	config.MinIO.SecretKey = getEnv("MINIO_SECRET_KEY", "minioadmin123")
	config.MinIO.BucketName = getEnv("MINIO_BUCKET_NAME", "sugurta")

	// multicard configuration
	config.Multicard.Aggr.ApplicationID = getEnv("MULTICARD_AGGR_APPLiCATION_ID", "application_id")
	config.Multicard.Aggr.Secret = getEnv("MULTICARD_AGGR_SECRET", "secret_key")
	config.Multicard.Aggr.APIURLTest = getEnv("MULTICARD_AGGR_API_URL_TEST", "https://dev-mesh.multicard.uz/")
	config.Multicard.Aggr.APIURLProd = getEnv("MULTICARD_AGGR_API_URL_PROD", "https://mesh.multicard.uz/")
	config.Multicard.Aggr.StoreID = getEnv("MULTICARD_AGGR_STORE_ID", "1")
	config.Multicard.Aggr.Card = getEnv("MULTICARD_AGGR_CARD", "1234567890123456")
	config.Multicard.Aggr.CardValidDate = getEnv("MULTICARD_AGGR_CARD_WALID_DATE", "01/29")

	config.Multicard.Card.ApplicationID = getEnv("MULTICARD_CARD_APPLiCATION_ID", "application_id")
	config.Multicard.Card.Secret = getEnv("MULTICARD_CARD_SECRET", "secret_key")
	config.Multicard.Card.APIURLTest = getEnv("MULTICARD_CARD_API_URL_TEST", "https://dev-mesh.multicard.uz/")
	config.Multicard.Card.APIURLProd = getEnv("MULTICARD_CARD_API_URL_PROD", "https://mesh.multicard.uz/")
	config.Multicard.Card.Card = getEnv("MULTICARD_CARD_CARD", "1234567890123456")
	config.Multicard.Card.CardToken = getEnv("MULTICARD_CARD_CARD_TOKEN", "card_token")

	config.Validation.ApiKey = getEnv("VALIDATION_API_KEY", "your_validation_api_key")
	config.Validation.BaseURL = getEnv("VALIDATION_BASE_URL", "https://api.validation.uz/")

	// telegram bot configuration
	config.TelegramBot.Token = getEnv("TELEGRAM_BOT_TOKEN", "your_telegram_bot_token")
	config.TelegramBot.Username = getEnv("TELEGRAM_BOT_USERNAME", "your_telegram_bot_username")

	// eskiz configuration
	config.Eskiz.Email = getEnv("ESKIZ_EMAIL", "your_eskiz_email")
	config.Eskiz.SecretCode = getEnv("ESKIZ_SECRET_CODE", "your_eskiz_secret_code")
	config.Eskiz.APIURL = getEnv("ESKIZ_API_URL", "https://notify.eskiz.uz/api/")

	// kafka configuration
	// config.Kafka.Address = strings.Split(getEnv("KAFKA_ADDRESS", "localhost:29092"), ",")
	// config.Kafka.Topic.InvestmentPaymentTransaction = getEnv("KAFKA_TOPIC_INVESTMENT_PAYMENT_TRANSACTION", "investment.payment.transaction")

	return &config, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

// NewConfig returns app config.
// func NewConfig() (*Config, error) {
// 	cfg := &Config{}

// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Println("No .env file found, relying on environment variables")
// 	}
// 	if err := env.Parse(cfg); err != nil {
// 		return nil, fmt.Errorf("config error: %w", err)
// 	}

// 	fmt.Println("config: ", cfg)

// 	return cfg, nil
// }
