package config

import (
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	DB struct {
		Host     string `env:"DB_HOST" envDefault:"localhost"`
		Port     string `env:"DB_PORT" envDefault:"6379"`
		Password string `env:"DB_PASSWORD" envDefault:"password123"`
	}
	ProductClient struct {
		Host string `env:"PRODUCT_CATALOG_HOST" envDefault:"product-catalog-service"`
		Port string `env:"PRODUCT_CATALOG_PORT" envDefault:"8080"`
	}
	Server struct {
		Port string `env:"SERVER_PORT" envDefault:":8080"`
	}
}

func New() (*Config, error) {
	var config Config

	if err := cleanenv.ReadEnv(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
