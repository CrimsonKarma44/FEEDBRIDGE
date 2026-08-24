package model

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type ENV struct {
	Token    string
	APIAddr  string
	Database DatabaseEnv
}

type DatabaseEnv struct {
	Host     string
	User     string
	Password string
	Name     string
	Port     string
}

func (d *DatabaseEnv) DSN() string {
	if d.Host == "" || d.User == "" || d.Password == "" || d.Name == "" {
		log.Fatal("missing required database environment variables")
	}
	if d.Port == "" {
		d.Port = "5432"
	}
	return "host=" + d.Host + " user=" + d.User + " password=" + d.Password +
		" dbname=" + d.Name + " port=" + d.Port + " sslmode=disable"
}

func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func UpdateEnv() (*ENV, error) {
	if err := godotenv.Load(); err != nil {
		return nil, err
	}

	return &ENV{
		Token:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		APIAddr: defaultString(os.Getenv("API_ADDR"), "localhost:50051"),
		Database: DatabaseEnv{
			Host:     os.Getenv("DB_HOST"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Name:     os.Getenv("DB_NAME"),
			Port:     os.Getenv("DB_PORT"),
		},
	}, nil
}
