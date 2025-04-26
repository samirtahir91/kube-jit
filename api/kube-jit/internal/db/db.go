package db

import (
	"fmt"
	"log"
	"os"

	"kube-jit/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	var err error

	// Read environment variables
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")
	sslmode := os.Getenv("DB_SSLMODE")
	timezone := os.Getenv("DB_TIMEZONE")
	connect_timeout := os.Getenv("DB_CONN_TIMEOUT")

	// Construct DSN
	dsn := fmt.Sprintf("host=%s connect_timeout=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, connect_timeout, user, password, dbname, port, sslmode, timezone)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Auto migrate the schema
	log.Println("Migrating database schema...")
	err = DB.AutoMigrate(&models.RequestData{})
	if err != nil {
		log.Fatalf("Error migrating database: %v", err)
	}
	log.Println("Database schema migrated successfully")
}
