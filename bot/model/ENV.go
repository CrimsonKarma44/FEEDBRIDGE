package model

import (
	"os"

	"github.com/joho/godotenv"
)

type ENV struct {
	Token string `env:"TOKEN"`
}

func UpdateEnv() (*ENV, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	return &ENV{
		Token: os.Getenv("TELEGRAM_BOT_TOKEN"),
	}, nil
}
