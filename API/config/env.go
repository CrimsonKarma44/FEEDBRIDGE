package config

import (
	"os"
	"github.com/joho/godotenv"
	"log"
)

type DatabaseEnv struct {
	Host     string
	User     string
	Password string
	Db_Name  string
	Ports    string
}

func LoadEnv() *DatabaseEnv {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	return &DatabaseEnv{
		Host:     os.Getenv("DB_HOST"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Db_Name:  os.Getenv("DB_NAME"),
		Ports:    os.Getenv("DB_PORT"),
	}
}
