package main

import (
	"kube-jit/internal/db"
	"kube-jit/internal/middleware"
	"kube-jit/internal/routes"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize database
	db.InitDB()

	r := gin.Default()

	// Setup middleware
	middleware.SetupMiddleware(r)

	// Setup routes
	routes.SetupRoutes(r)

	port := os.Getenv("LISTEN_PORT")
	log.Fatal(r.Run(":" + port))
}
