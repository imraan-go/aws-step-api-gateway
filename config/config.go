package config

import (
	"log"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type (
	// Config -.
	Config struct {
		Debug     bool   `env:"DEBUG" mapstructure:"DEBUG"`
		AwsKey    string `env:"AWS_KEY"`
		AwsSecret string `env:"AWS_SECRET"`
		HTTP
		Database
	}
	HTTP struct {
		H2C         bool   `env:"H2C"`
		HTTPAddress string `env:"HTTP_ADDRESS" mapstructure:"HTTP_ADDRESS"`
	}

	GRPC struct {
		GrpcPort string `env:"GRPC_PORT"`
	}

	// DB -.
	Database struct {
		ApiKey          string `env:"API_KEY"`
		DbHost          string `env:"DBHOST" mapstructure:"DBHOST"`
		DbUser          string `env:"DBUSER" mapstructure:"DBUSER"`
		DbPass          string `env:"DBPASS" mapstructure:"DBPASS"`
		DbPort          string `env:"DBPORT" mapstructure:"DBPORT"`
		DbName          string `env:"DBNAME" mapstructure:"DBNAME"`
		DbSchema        string `env:"DBSCHEMA" mapstructure:"DBSCHEMA"`
		SetMaxOpenConns int    `env:"SETMAXOPENCONNS" mapstructure:"SETMAXOPENCONNS"`
	}
)

// Read properties from config.env file
// Command line enviroment variable will overwrite config.env properties
func NewConfig(configFile string) *Config {
	var config Config
	godotenv.Load(configFile)
	err := cleanenv.ReadEnv(&config)
	if err != nil {
		log.Fatal(err)
	}
	return &config
}
