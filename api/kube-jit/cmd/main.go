// @title Kube-JIT API
// @version 1.0
// @description Self-service Kubernetes RBAC with GitHub Teams.
// @BasePath /kube-jit-api
package main

import (
	"encoding/gob"
	"flag"
	"kube-jit/internal/db"
	"kube-jit/internal/handlers"
	"kube-jit/internal/middleware"
	"kube-jit/internal/routes"
	"kube-jit/pkg/k8s"
	"kube-jit/pkg/utils"
	"os"
	"regexp"
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

	// Register gob types for db
	gob.Register(map[string]any{})

	// Initialize zap logger in all packages
	handlers.InitLogger(logger)
	db.InitLogger(logger)
	middleware.InitLogger(logger)
	k8s.InitLogger(logger)
	utils.InitLogger(logger)

	// Initialize Kubernetes client and cache
	k8s.InitK8sConfig()

	// Initialize database
	db.InitDB()

	r := gin.New()

	// Skip only authenticated routes and healthz (not oauth, client_id, build-sha, logout)
	rxAuthenticated := regexp.MustCompile(`^/kube-jit-api/(healthz|approving-groups|roles-and-clusters|github/profile|google/profile|azure/profile|submit-request|history|approvals|approve-reject|permissions|admin/clean-expired)$`)
	r.Use(ginzap.GinzapWithConfig(logger, &ginzap.Config{
		UTC:             true,
		TimeFormat:      time.RFC3339,
		SkipPathRegexps: []*regexp.Regexp{rxAuthenticated},
	}))
	r.Use(ginzap.RecoveryWithZap(logger, true))

	// Setup middleware
	middleware.SetupMiddleware(r)

	// Setup routes
	routes.SetupRoutes(r)

	port := utils.MustGetEnv("LISTEN_PORT")
	logger.Info("Starting server", zap.String("port", port))
	if err := r.Run(":" + port); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
