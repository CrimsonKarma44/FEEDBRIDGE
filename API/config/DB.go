package config

import (
	"fmt"
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func (db *DB) InitDB(values *DatabaseEnv) error {
	var err error
	db.DB, err = gorm.Open(postgres.Open(db.dns(values)), &gorm.Config{})
	if err != nil {

		return fmt.Errorf("error occured")
	}

	return nil
}

func (db *DB) migrateDB() error {
	err := db.DB.AutoMigrate()
	if err != nil {
		return err
	}
	return nil
}

func NewDB(values *DatabaseEnv) (*DB, error) {
	db := &DB{}
	err := db.InitDB(values)
	if err != nil {
		return nil, err
	}
	err = db.migrateDB()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func (app DB) dns(env *DatabaseEnv) string {
	if env.Host == "" || env.User == "" || env.Password == "" || env.Db_Name == "" {
		log.Fatalf("Missing required environment variables")
	}

	if env.Ports == "" {
		env.Ports = "5432"
	}

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		env.Host,
		env.User,
		env.Password,
		env.Db_Name,
		env.Ports,
	)
}