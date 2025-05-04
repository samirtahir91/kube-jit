package db

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

	// Enable GORM debug mode if DB_DEBUG=true
	if os.Getenv("DB_DEBUG") == "true" {
		DB = DB.Debug()
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}

	// Read connection pool settings from environment variables
	maxOpenConns, _ := strconv.Atoi(getEnv("DB_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(getEnv("DB_MAX_IDLE_CONNS", "10"))
	connMaxLifetime, _ := time.ParseDuration(getEnv("DB_CONN_MAX_LIFETIME", "5m"))
	connMaxIdleTime, _ := time.ParseDuration(getEnv("DB_CONN_MAX_IDLE_TIME", "10m"))

	sqlDB.SetMaxOpenConns(maxOpenConns)       // Maximum number of open connections
	sqlDB.SetMaxIdleConns(maxIdleConns)       // Maximum number of idle connections
	sqlDB.SetConnMaxLifetime(connMaxLifetime) // Maximum lifetime of a connection
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime) // Maximum idle time for a connection

	// Auto migrate the schema
	log.Println("Migrating database schema...")
	err = DB.AutoMigrate(&models.RequestData{}, &models.RequestNamespace{})
	if err != nil {
		log.Fatalf("Error migrating database: %v", err)
	}

	log.Println("Database schema migrated successfully")
}

// getEnv reads an environment variable or returns a default value if not set
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
