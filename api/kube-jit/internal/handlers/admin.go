package handlers

import (
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CleanExpiredRequests deletes requests where endDate < now and status is Requested (not Approved or Rejected)
func CleanExpiredRequests(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	isAdmin, _ := sessionData["isAdmin"].(bool)
	if !isAdmin {
		reqLogger.Warn("Unauthorized access attempt to CleanExpiredRequests")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: admin only"})
		return
	}

	now := time.Now()
	result := db.DB.
		Where("end_date < ? AND status = ?", now, "Requested").
		Delete(&models.RequestData{})

	if result.Error != nil {
		reqLogger.Error("Failed to clean expired non-approved requests", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clean expired requests"})
		return
	}

	reqLogger.Info("Expired non-approved requests cleaned",
		zap.Int64("deleted", result.RowsAffected),
	)
	c.JSON(http.StatusOK, gin.H{
		"message": "Expired non-approved requests cleaned",
		"deleted": result.RowsAffected,
	})
}
