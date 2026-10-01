package testdb

import (
	"testing"
)

func TestOpen_MigratesEmptySchema(t *testing.T) {
	db := Open(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatal(err)
	}
}
