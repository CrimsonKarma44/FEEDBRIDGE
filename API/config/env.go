package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type ENV struct {
	Database      DatabaseEnv
	Redis         RedisENV
	YoutubeAPIKey string
}

func (e *ENV) String() string {
	return "Loaded ENV" + ", Database: " + e.Database.String() + ", Redis: " + e.Redis.String()
}

func LoadENV() *ENV {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	return &ENV{
		YoutubeAPIKey: os.Getenv("YOUTUBE_API_KEY"),
		Database: DatabaseEnv{
			Host:     os.Getenv("DB_HOST"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			Db_Name:  os.Getenv("DB_NAME"),
			Ports:    os.Getenv("DB_PORT"),
		},
		Redis: RedisENV{
			Addr:     os.Getenv("REDIS_ADDR"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB: func() int {
				db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
				return db
			}(),
			Protocol: func() int {
				proto, _ := strconv.Atoi(os.Getenv("REDIS_PROTOCOL"))
				return proto
			}(),
		},
	}
}

type DatabaseEnv struct {
	Host     string
	User     string
	Password string
	Db_Name  string
	Ports    string
}

func (e *DatabaseEnv) String() string {
	return "Loaded ENV" + ", Db_Name: " + e.Db_Name
}

type RedisENV struct {
	Addr     string
	Password string
	DB       int
	Protocol int
}

func (e *RedisENV) String() string {
	return "Loaded ENV" + ", Addr: " + e.Addr
}
