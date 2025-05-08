package handlers

import (
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CleanExpiredResponse represents the response for CleanExpiredRequests
type CleanExpiredResponse struct {
	Message string `json:"message"`
	Deleted int64  `json:"deleted"`
}

// CleanExpiredRequests godoc
// @Summary Clean up expired non-approved JIT requests
// @Description Deletes JIT requests where endDate is in the past and status is "Requested" (not Approved or Rejected). Admin only.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies.
// @Tags admin
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Success 200 {object} handlers.CleanExpiredResponse "Expired non-approved requests cleaned"
// @Failure 401 {object} models.SimpleMessageResponse "Unauthorized: admin only"
// @Failure 500 {object} models.SimpleMessageResponse "Failed to clean expired requests"
// @Router /admin/clean-expired [post]
func CleanExpiredRequests(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	isAdmin, _ := sessionData["isAdmin"].(bool)
	if !isAdmin {
		reqLogger.Warn("Unauthorized access attempt to CleanExpiredRequests")
		c.JSON(http.StatusUnauthorized, models.SimpleMessageResponse{Error: "Unauthorized: admin only"})
		return
	}

	now := time.Now()
	result := db.DB.
		Where("end_date < ? AND status = ?", now, "Requested").
		Delete(&models.RequestData{})

	if result.Error != nil {
		reqLogger.Error("Failed to clean expired non-approved requests", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to clean expired requests"})
		return
	}

	reqLogger.Info("Expired non-approved requests cleaned",
		zap.Int64("deleted", result.RowsAffected),
	)
	c.JSON(http.StatusOK, CleanExpiredResponse{
		Message: "Expired non-approved requests cleaned",
		Deleted: result.RowsAffected,
	})
}
