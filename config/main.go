package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/sujit-baniya/ip"
)

// AllConfig variable of type AppConfig
var AllConfig AppConfig

// AppConfig type AppConfig
type AppConfig struct {
	IsDevelopment bool   `envconfig:"IS_DEVELOPMENT"`
	Debug         bool   `envconfig:"DEBUG"`
	Env           string `envconfig:"APP_ENV"`
	Port          string `envconfig:"APP_PORT"`
	Secret        string `envconfig:"JWT_SECRET"`
	DB            *DBConfig
	GeoIP         *ip.GeoIpDB
	Server        ServerConfig `yaml:"server"`
	JWTSecret     string       `envconfig:"JWT_SECRET"`
	AppName			 string       `envconfig:"APP_NAME"`
}

// GetConfig Collects all configs
func GetConfig() AppConfig {
	err := godotenv.Load()
	if err != nil {
		log.Println("warning .env file not found, scanning from OS ENV")
	}

	AllConfig = AppConfig{}

	err = envconfig.Process("APP_PORT", &AllConfig)
	if err != nil {
		log.Fatal(err)
	}

	return AllConfig
}

// GetConfigByName Collects all configs
func GetConfigByName(key string) string {
	err := godotenv.Load()

	if err != nil {
		log.Fatal(err)
	}

	return os.Getenv(key)
}

// LoadTestEnv loads environment variables from .env.testing file
func LoadTestEnv() AppConfig {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	err = godotenv.Load(fmt.Sprintf("%s/.env.testing", cwd))
	if err != nil {
		log.Fatal(err)
	}
	return GetConfig()
}
