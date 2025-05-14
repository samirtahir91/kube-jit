package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetRecords godoc
// @Summary Get JIT requests for a user
// @Description Returns the latest JIT requests for a user with optional limit and date range.
// @Description Requires one or more cookies named kube_jit_session_<number> (e.g., kube_jit_session_0, kube_jit_session_1).
// @Description Pass split cookies in the Cookie header, for example:
// @Description     -H "Cookie: kube_jit_session_0=${cookie_0};kube_jit_session_1=${cookie_1}"
// @Description Note: Swagger UI cannot send custom Cookie headers due to browser security restrictions. Use curl for testing with split cookies:
// @Description Login required to test via browser, else test via curl
// @Tags records
// @Accept  json
// @Produce  json
// @Param   Cookie header string true "Session cookies (multiple allowed, names: kube_jit_session_0, kube_jit_session_1, etc.)"
// @Param   userID     query    string  false  "User ID"
// @Param   username   query    string  false  "Username"
// @Param   limit      query    int     false  "Limit"
// @Success 200 {array} models.RequestWithNamespaceApprovers
// @Failure 500 {object} models.SimpleMessageResponse
// @Router /history [get]
func GetRecords(c *gin.Context) {
	// Check if the user is logged in and get logger
	sessionData := GetSessionData(c)
	reqLogger := RequestLogger(c)

	isAdmin, _ := sessionData["isAdmin"].(bool)
	isPlatformApprover, _ := sessionData["isPlatformApprover"].(bool)
	userID := c.Query("userID")
	username := c.Query("username")
	limit := c.Query("limit")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		limitInt = 1
	}

	var requests []models.RequestData
	query := db.DB.Order("created_at desc").Limit(limitInt)
	if isAdmin || isPlatformApprover {
		if userID != "" {
			query = query.Where("user_id = ?", userID)
		}
		if username != "" {
			query = query.Where("username = ?", username)
		}
	} else {
		if userID != "" {
			query = query.Where("user_id = ? OR approver_ids @> ?", userID, fmt.Sprintf(`["%s"]`, userID))
		} else if username != "" {
			query = query.Where("username = ? OR approver_names @> ?", username, fmt.Sprintf(`["%s"]`, username))
		}
	}
	if startDate != "" && endDate != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}
	if err := query.Find(&requests).Error; err != nil {
		reqLogger.Error("Error fetching records in GetRecords", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch records"})
		return
	}

	var requestsWithApprovals []models.RequestWithNamespaceApprovers

	for _, req := range requests {
		var nsApprovals []models.NamespaceApprovalInfo
		if err := db.DB.Table("request_namespaces").
			Select("namespace, group_name, group_id, approved, approver_id, approver_name").
			Where("request_id = ?", req.ID).
			Find(&nsApprovals).Error; err != nil {
			reqLogger.Error("Error fetching namespace approvals for request", zap.Uint("requestID", req.ID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.SimpleMessageResponse{Error: "Failed to fetch namespace approvals"})
			return
		}
		if nsApprovals == nil {
			nsApprovals = []models.NamespaceApprovalInfo{} // Ensure empty slice instead of nil
		}
		requestsWithApprovals = append(requestsWithApprovals, models.RequestWithNamespaceApprovers{
			RequestData:        req,
			NamespaceApprovals: nsApprovals,
		})
	}

	if requestsWithApprovals == nil {
		requestsWithApprovals = []models.RequestWithNamespaceApprovers{}
	}

	c.JSON(http.StatusOK, requestsWithApprovals)
}
