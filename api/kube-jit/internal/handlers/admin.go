package handlers

import (
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CleanExpiredRequests deletes requests where endDate < now and status is not Approved or Rejected
func CleanExpiredRequests(c *gin.Context) {
	sessionData, ok := checkLoggedIn(c)
	if !ok {
		return
	}
	isAdmin, _ := sessionData["isAdmin"].(bool)
	if !isAdmin {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: admin only"})
		return
	}

	now := time.Now()
	result := db.DB.
		Where("end_date < ? AND status = ?", now, "Requested").
		Delete(&models.RequestData{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Expired non-approved requests cleaned",
		"deleted": result.RowsAffected,
	})
}
