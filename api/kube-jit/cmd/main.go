package main

import (
	"flag"
	"kube-jit/internal/db"
	"kube-jit/internal/middleware"
	"kube-jit/internal/routes"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
)

var logger *zap.Logger

func main() {
	// Read DEBUG_LOG from env var
	debugLog, logVarErr := strconv.ParseBool(os.Getenv("DEBUG_LOG"))
	if logVarErr != nil {
		debugLog = false
	}

	var zapCfg zap.Config
	if debugLog {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	// Optional: allow zap to bind flags if you want CLI overrides
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	flag.Parse()

	var err error
	logger, err = zapCfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Initialize zap logger in all packages
	db.InitLogger(logger)
	middleware.InitLogger(logger)
	k8s.InitLogger(logger)
	utils.InitLogger(logger)

	// Initialize Kubernetes client and cache
	k8s.InitK8sConfig()

	// Initialize database
	db.InitDB()

	r := gin.New()
	r.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	r.Use(ginzap.RecoveryWithZap(logger, true))

	// Setup middleware
	middleware.SetupMiddleware(r)

	// Setup routes
	routes.SetupRoutes(r)

	port := os.Getenv("LISTEN_PORT")
	logger.Info("Starting server", zap.String("port", port))
	if err := r.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
