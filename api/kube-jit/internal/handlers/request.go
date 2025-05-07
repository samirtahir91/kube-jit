package handlers

import (
	"fmt"
	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/email"
	"kube-jit/pkg/k8s"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SubmitRequest creates the new jit record in postgress
// It validates the request data and checks if the user is logged in
// It also validates the namespaces and sends an email notification
// It returns a success message or an error message
func SubmitRequest(c *gin.Context) {
	// Check if the user is logged in
	logger := c.MustGet("logger").(*zap.Logger)
	sessionData := GetSessionData(c)

	// Check if the email is present in the session data
	emailAddress, _ := sessionData["email"].(string)

	// Process the request data
	var requestData struct {
		Role          models.Roles   `json:"role"`
		ClusterName   models.Cluster `json:"cluster"`
		UserID        string         `json:"requestorId"`
		Username      string         `json:"requestorName"`
		Users         []string       `json:"users"`
		Namespaces    []string       `json:"namespaces"`
		Justification string         `json:"justification"`
		StartDate     time.Time      `json:"startDate"`
		EndDate       time.Time      `json:"endDate"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate namespaces and fetch group IDs and names
	namespaceGroups, err := k8s.ValidateNamespaces(requestData.ClusterName.Name, requestData.Namespaces)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Namespace validation failed: %v", err)})
		return
	}

	// Create a new RequestData in database
	dbRequestData := models.RequestData{
		ClusterName:   requestData.ClusterName.Name,
		RoleName:      requestData.Role.Name,
		Status:        "Requested",
		UserID:        requestData.UserID,
		Username:      requestData.Username,
		Users:         requestData.Users,
		Namespaces:    requestData.Namespaces,
		Justification: requestData.Justification,
		StartDate:     requestData.StartDate,
		EndDate:       requestData.EndDate,
		Email:         emailAddress,
	}

	// Insert the request data into the database
	if err := db.DB.Create(&dbRequestData).Error; err != nil {
		logger.Error("Error inserting data in SubmitRequest", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (database error)"})
		return
	}

	// Insert namespaces into the request_namespaces table
	for namespace, groupInfo := range namespaceGroups {
		namespaceEntry := models.RequestNamespace{
			RequestID: dbRequestData.ID,
			Namespace: namespace,
			GroupID:   groupInfo.GroupID,
			GroupName: groupInfo.GroupName,
			Approved:  false,
		}
		if err := db.DB.Create(&namespaceEntry).Error; err != nil {
			logger.Error("Error inserting namespace data in SubmitRequest", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit request (namespace error)"})
			return
		}
	}

	// Send submission email
	if dbRequestData.Email != "" {
		body := email.BuildRequestEmail(email.EmailRequestDetails{
			Username:      dbRequestData.Username,
			ClusterName:   dbRequestData.ClusterName,
			Namespaces:    dbRequestData.Namespaces,
			RoleName:      dbRequestData.RoleName,
			Justification: dbRequestData.Justification,
			StartDate:     dbRequestData.StartDate,
			EndDate:       dbRequestData.EndDate,
			Status:        "submitted",
			Message:       "",
		})
		go func() {
			if err := email.SendMail(dbRequestData.Email, fmt.Sprintf("Your JIT request #%d has been submitted", dbRequestData.ID), body); err != nil {
				logger.Warn("Failed to send submission email", zap.String("email", dbRequestData.Email), zap.Error(err))
			}
		}()
	}

	// Respond with success message
	c.JSON(http.StatusOK, gin.H{"message": "Request submitted successfully"})
}

// ApproveOrRejectRequests approves pending requests in db - status = Approved
// or rejects them - status = Rejected
// It creates the k8s object for each request if status is Approved
// It updates the status of the requests in the database
func ApproveOrRejectRequests(c *gin.Context) {
	// Check if the user is logged in
	sessionData := GetSessionData(c)

	// Check if the user is an admin or platform approver
	isAdmin, _ := sessionData["isAdmin"].(bool)
	isPlatformApprover, _ := sessionData["isPlatformApprover"].(bool)

	var approverGroups []string
	if !isAdmin && !isPlatformApprover {
		// Only non-admins need approverGroups
		rawApproverGroups, ok := sessionData["approverGroups"]
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
			return
		}
		// Handle both []models.Team and []interface{} (from session serialization)
		if rawGroups, ok := rawApproverGroups.([]models.Team); ok {
			for _, group := range rawGroups {
				approverGroups = append(approverGroups, group.ID)
			}
		} else if rawGroups, ok := rawApproverGroups.([]any); ok {
			for _, group := range rawGroups {
				if groupMap, ok := group.(map[string]any); ok {
					if id, ok := groupMap["id"].(string); ok {
						approverGroups = append(approverGroups, id)
					}
				}
			}
		}
		if len(approverGroups) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: no approver groups in session"})
			return
		}
	}

	if isAdmin || isPlatformApprover {
		// Admin/Platform Approver: expects Namespaces []string
		type AdminApproveRequest struct {
			Requests     []models.RequestData `json:"requests"`
			ApproverID   string               `json:"approverID"`
			ApproverName string               `json:"approverName"`
			Status       string               `json:"status"`
		}
		var req AdminApproveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		for _, r := range req.Requests {
			processApproval(r.ID, r, req.ApproverID, req.ApproverName, req.Status, nil, c)
		}
		c.JSON(http.StatusOK, gin.H{"message": "Admin/Platform requests processed successfully"})
		return
	} else {
		// Non-admin: expects Namespace string
		type UserApproveRequest struct {
			Requests []struct {
				ID            uint      `json:"id"`
				ApproverName  string    `json:"approverName"`
				ClusterName   string    `json:"clusterName"`
				RoleName      string    `json:"roleName"`
				Status        string    `json:"status"`
				UserID        string    `json:"userID"`
				Username      string    `json:"username"`
				Users         []string  `json:"users"`
				Justification string    `json:"justification"`
				StartDate     time.Time `json:"startDate"`
				EndDate       time.Time `json:"endDate"`
				FullyApproved bool      `gorm:"default:false"`
				Namespace     string    `json:"namespace"`
			} `json:"requests"`
			ApproverID   string `json:"approverID"`
			ApproverName string `json:"approverName"`
			Status       string `json:"status"`
		}
		var req UserApproveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		for _, r := range req.Requests {
			namespaces := []string{r.Namespace}
			// Convert to models.RequestData for downstream compatibility
			requestData := models.RequestData{
				ClusterName:   r.ClusterName,
				RoleName:      r.RoleName,
				Status:        r.Status,
				UserID:        r.UserID,
				Username:      r.Username,
				Users:         r.Users,
				Namespaces:    namespaces,
				Justification: r.Justification,
				StartDate:     r.StartDate,
				EndDate:       r.EndDate,
				FullyApproved: r.FullyApproved,
			}
			processApproval(r.ID, requestData, req.ApproverID, req.ApproverName, req.Status, approverGroups, c)
		}
		c.JSON(http.StatusOK, gin.H{"message": "User requests processed successfully"})
		return
	}
}

// Helper function to process approval logic for each request
// It updates the request status and approver information in the database
// It also creates the k8s object if all namespaces are approved
// It sends an email notification to the user if the request is approved
func processApproval(
	requestID uint,
	requestData models.RequestData,
	approverID string,
	approverName string,
	status string,
	approverGroups []string,
	c *gin.Context,
) {
	// Fetch namespaces for the request
	var dbNamespaces []models.RequestNamespace
	if err := db.DB.Where("request_id = ?", requestID).Find(&dbNamespaces).Error; err != nil {
		logger.Error("Error fetching namespaces for request", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch namespaces"})
		return
	}

	// Approve all if admin (approverGroups == nil), else check group
	for i := range dbNamespaces {
		ns := &dbNamespaces[i]
		if approverGroups == nil || contains(approverGroups, ns.GroupID) {
			if status == "Approved" {
				ns.Approved = true
			} else if status == "Rejected" {
				ns.Approved = false
			}
			ns.ApproverID = approverID
			ns.ApproverName = approverName
			if err := db.DB.Save(ns).Error; err != nil {
				logger.Error("Error updating namespace approval", zap.Uint("requestID", requestID), zap.String("namespace", ns.Namespace), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update namespace approval"})
				return
			}
		} else {
			logger.Info("Skipping namespace - approver does not have permissions",
				zap.String("namespace", ns.Namespace),
				zap.String("groupID", ns.GroupID),
				zap.String("approverID", approverID),
			)
		}
	}

	// Check if all namespaces for the request are approved
	allApproved := true
	for _, ns := range dbNamespaces {
		if !ns.Approved {
			allApproved = false
			break
		}
	}

	// Only set status to "Approved" if all namespaces are approved, otherwise keep as "Requested"
	finalStatus := status
	if status == "Approved" && !allApproved {
		finalStatus = "Requested"
	}

	logger.Debug("Approval status check",
		zap.Bool("allApproved", allApproved),
		zap.String("status", status),
		zap.String("finalStatus", finalStatus),
		zap.Uint("requestID", requestID),
	)

	if allApproved && status == "Approved" {
		var namespacesToSpec []string
		for _, ns := range dbNamespaces {
			namespacesToSpec = append(namespacesToSpec, ns.Namespace)
		}
		requestData.Namespaces = namespacesToSpec
		requestData.ID = requestID
		if err := k8s.CreateK8sObject(requestData, approverName); err != nil {
			logger.Error("Error creating k8s object for request", zap.Uint("requestID", requestID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create k8s object"})
			return
		}
	}

	// Fetch the request record
	var req models.RequestData
	if err := db.DB.First(&req, requestID).Error; err != nil {
		logger.Error("Error fetching request for update", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request"})
		return
	}

	// Append approver if not already present
	if !contains(req.ApproverIDs, approverID) {
		req.ApproverIDs = append(req.ApproverIDs, approverID)
	}
	if !contains(req.ApproverNames, approverName) {
		req.ApproverNames = append(req.ApproverNames, approverName)
	}

	// Update the request status and approvers using struct update
	req.Status = finalStatus
	req.FullyApproved = allApproved

	if err := db.DB.Model(&req).Select("Status", "ApproverIDs", "ApproverNames", "FullyApproved").Updates(req).Error; err != nil {
		logger.Error("Error updating request after approval", zap.Uint("requestID", requestID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update request"})
		return
	}

	if req.Email != "" {
		body := email.BuildRequestEmail(email.EmailRequestDetails{
			Username:      req.Username,
			ClusterName:   req.ClusterName,
			Namespaces:    req.Namespaces,
			RoleName:      req.RoleName,
			Justification: req.Justification,
			StartDate:     req.StartDate,
			EndDate:       req.EndDate,
			Status:        req.Status,
			Message:       "", // Reserved for controller messages
		})
		go func() {
			if err := email.SendMail(req.Email, fmt.Sprintf("Your JIT request #%d is now %s", req.ID, req.Status), body); err != nil {
				logger.Warn("Failed to send status change email", zap.String("email", req.Email), zap.Error(err))
			}
		}()
	}
}
