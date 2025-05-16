package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kube-jit/internal/db"
	"kube-jit/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var originalDBGetRecordsTest *gorm.DB

// setupGetRecordsTest configures a Gin router with a mocked DB and session management for GetRecords tests.
func setupGetRecordsTest(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	var testLogger = zap.NewNop() // Use zap.NewDevelopment() for verbose logs during debugging
	InitLogger(testLogger)
	db.InitLogger(testLogger)

	mockDb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to open sqlmock database: %s", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn:                 mockDb,
		PreferSimpleProtocol: true,
	})
	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent), // Suppress GORM logging
	})
	if err != nil {
		t.Fatalf("Failed to open gorm database: %s", err)
	}

	originalDBGetRecordsTest = db.DB
	db.DB = gormDB

	store := cookie.NewStore([]byte("test-secret-key-records-get"))
	r.Use(sessions.Sessions("kube_jit_session", store))

	r.Use(func(c *gin.Context) {
		c.Set("logger", testLogger)
		c.Next()
	})

	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		sessionDataFromStore := s.Get("sessionData")
		var dataToSetInContext map[string]interface{}

		if sessionDataFromStore != nil {
			if m, ok := sessionDataFromStore.(map[string]interface{}); ok {
				dataToSetInContext = m
			} else if mAny, okAny := sessionDataFromStore.(map[string]any); okAny {
				convertedMap := make(map[string]interface{})
				for k, v := range mAny {
					convertedMap[k] = v
				}
				dataToSetInContext = convertedMap
			} else {
				dataToSetInContext = make(map[string]interface{})
			}
		} else {
			dataToSetInContext = make(map[string]interface{})
		}
		c.Set("sessionData", dataToSetInContext)
		c.Next()
	})

	teardown := func() {
		db.DB = originalDBGetRecordsTest
		_ = mockDb.Close()
	}

	return r, mock, teardown
}

func TestGetRecords(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	// Sample data for request_data table
	sampleRecord1 := models.RequestData{
		GormModel:     models.GormModel{ID: 1, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		ClusterName:   "cluster-a",
		RoleName:      "role-x",
		Status:        "Approved",
		UserID:        "user1",
		Username:      "user1@example.com",
		Users:         []string{"user1@example.com"},
		Namespaces:    []string{"ns-a"}, // Assuming this field exists and is populated
		Justification: "Record 1 Justification",
		StartDate:     now.Add(-time.Hour),
		EndDate:       now.Add(time.Hour),
		ApproverIDs:   []string{"admin1"},
		ApproverNames: []string{"Admin One"},
		FullyApproved: true,
		Email:         "user1@example.com",
	}
	sampleRecord2 := models.RequestData{
		GormModel:     models.GormModel{ID: 2, CreatedAt: now, UpdatedAt: now},
		ClusterName:   "cluster-b",
		RoleName:      "role-y",
		Status:        "Requested",
		UserID:        "user2",
		Username:      "user2@example.com",
		Users:         []string{"user2@example.com"},
		Namespaces:    []string{"ns-b", "ns-c"},
		Justification: "Record 2 Justification",
		StartDate:     now,
		EndDate:       now.Add(2 * time.Hour),
		ApproverIDs:   []string{"user1"}, // user1 is an approver for this request
		ApproverNames: []string{"user1@example.com"},
		FullyApproved: false,
		Email:         "user2@example.com",
	}

	// Sample data for namespace_approvals table
	sampleNsApproval1ForRecord1 := models.NamespaceApprovalInfo{
		Namespace:    "ns-a",
		GroupName:    "Group Alpha",
		GroupID:      "groupA",
		Approved:     true,
		ApproverID:   "admin1",
		ApproverName: "Admin One",
	}
	sampleNsApproval1ForRecord2 := models.NamespaceApprovalInfo{
		Namespace:    "ns-b",
		GroupName:    "Group Beta",
		GroupID:      "groupB",
		Approved:     false,
		ApproverID:   "",
		ApproverName: "",
	}
	sampleNsApproval2ForRecord2 := models.NamespaceApprovalInfo{
		Namespace:    "ns-c",
		GroupName:    "Group Gamma",
		GroupID:      "groupC",
		Approved:     true,
		ApproverID:   "user1",
		ApproverName: "user1@example.com",
	}

	requestDataCols := []string{"id", "created_at", "updated_at", "deleted_at", "approver_ids", "approver_names", "cluster_name", "role_name", "status", "notes", "user_id", "username", "users", "namespaces", "justification", "start_date", "end_date", "fully_approved", "email"}
	nsApprovalCols := []string{"namespace", "group_name", "group_id", "approved", "approver_id", "approver_name"}

	testCases := []struct {
		name                string
		setupSession        func(t *testing.T, c *gin.Context)
		queryParams         string
		mockDB              func(t *testing.T, mock sqlmock.Sqlmock)
		expectedStatus      int
		expectedJSONBody    interface{}
		expectDBInteraction bool
	}{
		{
			name: "Admin user, no filters, default limit 1",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "", // Default limit 1
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				usersJSON1, _ := json.Marshal(sampleRecord1.Users)
				approverIDsJSON1, _ := json.Marshal(sampleRecord1.ApproverIDs)
				approverNamesJSON1, _ := json.Marshal(sampleRecord1.ApproverNames)
				namespacesJSON1, _ := json.Marshal(sampleRecord1.Namespaces)

				rowsRd := sqlmock.NewRows(requestDataCols).
					AddRow(sampleRecord1.ID, sampleRecord1.CreatedAt, sampleRecord1.UpdatedAt, sampleRecord1.DeletedAt, approverIDsJSON1, approverNamesJSON1, sampleRecord1.ClusterName, sampleRecord1.RoleName, sampleRecord1.Status, sampleRecord1.Notes, sampleRecord1.UserID, sampleRecord1.Username, usersJSON1, namespacesJSON1, sampleRecord1.Justification, sampleRecord1.StartDate, sampleRecord1.EndDate, sampleRecord1.FullyApproved, sampleRecord1.Email)
				mock.ExpectQuery(`SELECT \* FROM "request_data" ORDER BY created_at desc LIMIT \$1`).WithArgs(1).WillReturnRows(rowsRd)

				rowsNs := sqlmock.NewRows(nsApprovalCols).
					AddRow(sampleNsApproval1ForRecord1.Namespace, sampleNsApproval1ForRecord1.GroupName, sampleNsApproval1ForRecord1.GroupID, sampleNsApproval1ForRecord1.Approved, sampleNsApproval1ForRecord1.ApproverID, sampleNsApproval1ForRecord1.ApproverName)
				mock.ExpectQuery(`SELECT namespace, group_name, group_id, approved, approver_id, approver_name FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(sampleRecord1.ID).WillReturnRows(rowsNs)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: []models.RequestWithNamespaceApprovers{
				{RequestData: sampleRecord1, NamespaceApprovals: []models.NamespaceApprovalInfo{sampleNsApproval1ForRecord1}},
			},
			expectDBInteraction: true,
		},
		{
			name: "Platform approver, userID filter, limit 1",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isPlatformApprover": true}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "?userID=user2&limit=1",
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				usersJSON2, _ := json.Marshal(sampleRecord2.Users)
				approverIDsJSON2, _ := json.Marshal(sampleRecord2.ApproverIDs)
				approverNamesJSON2, _ := json.Marshal(sampleRecord2.ApproverNames)
				namespacesJSON2, _ := json.Marshal(sampleRecord2.Namespaces)

				rowsRd := sqlmock.NewRows(requestDataCols).
					AddRow(sampleRecord2.ID, sampleRecord2.CreatedAt, sampleRecord2.UpdatedAt, sampleRecord2.DeletedAt, approverIDsJSON2, approverNamesJSON2, sampleRecord2.ClusterName, sampleRecord2.RoleName, sampleRecord2.Status, sampleRecord2.Notes, sampleRecord2.UserID, sampleRecord2.Username, usersJSON2, namespacesJSON2, sampleRecord2.Justification, sampleRecord2.StartDate, sampleRecord2.EndDate, sampleRecord2.FullyApproved, sampleRecord2.Email)
				mock.ExpectQuery(`SELECT \* FROM "request_data" WHERE user_id = \$1 ORDER BY created_at desc LIMIT \$2`).
					WithArgs("user2", 1).WillReturnRows(rowsRd)

				rowsNs := sqlmock.NewRows(nsApprovalCols).
					AddRow(sampleNsApproval1ForRecord2.Namespace, sampleNsApproval1ForRecord2.GroupName, sampleNsApproval1ForRecord2.GroupID, sampleNsApproval1ForRecord2.Approved, sampleNsApproval1ForRecord2.ApproverID, sampleNsApproval1ForRecord2.ApproverName).
					AddRow(sampleNsApproval2ForRecord2.Namespace, sampleNsApproval2ForRecord2.GroupName, sampleNsApproval2ForRecord2.GroupID, sampleNsApproval2ForRecord2.Approved, sampleNsApproval2ForRecord2.ApproverID, sampleNsApproval2ForRecord2.ApproverName)
				mock.ExpectQuery(`SELECT namespace, group_name, group_id, approved, approver_id, approver_name FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(sampleRecord2.ID).WillReturnRows(rowsNs)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: []models.RequestWithNamespaceApprovers{
				{RequestData: sampleRecord2, NamespaceApprovals: []models.NamespaceApprovalInfo{sampleNsApproval1ForRecord2, sampleNsApproval2ForRecord2}},
			},
			expectDBInteraction: true,
		},
		{
			name: "Non-admin user, userID filter (own record)",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				// Session data for GetSessionData, not directly used by GetRecords's non-admin query logic unless query params are absent
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "userID": "user1", "username": "user1@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "?userID=user1&limit=1",
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				usersJSON1, _ := json.Marshal(sampleRecord1.Users)
				approverIDsJSON1, _ := json.Marshal(sampleRecord1.ApproverIDs)
				approverNamesJSON1, _ := json.Marshal(sampleRecord1.ApproverNames)
				namespacesJSON1, _ := json.Marshal(sampleRecord1.Namespaces)

				rowsRd := sqlmock.NewRows(requestDataCols).
					AddRow(sampleRecord1.ID, sampleRecord1.CreatedAt, sampleRecord1.UpdatedAt, sampleRecord1.DeletedAt, approverIDsJSON1, approverNamesJSON1, sampleRecord1.ClusterName, sampleRecord1.RoleName, sampleRecord1.Status, sampleRecord1.Notes, sampleRecord1.UserID, sampleRecord1.Username, usersJSON1, namespacesJSON1, sampleRecord1.Justification, sampleRecord1.StartDate, sampleRecord1.EndDate, sampleRecord1.FullyApproved, sampleRecord1.Email)
				// For non-admin, query is "user_id = ? OR approver_ids @> ?" - GORM doesn't seem to wrap this simple OR in parens
				mock.ExpectQuery(`SELECT \* FROM "request_data" WHERE user_id = \$1 OR approver_ids @> \$2 ORDER BY created_at desc LIMIT \$3`).
					WithArgs("user1", `["user1"]`, 1).WillReturnRows(rowsRd)

				rowsNs := sqlmock.NewRows(nsApprovalCols).
					AddRow(sampleNsApproval1ForRecord1.Namespace, sampleNsApproval1ForRecord1.GroupName, sampleNsApproval1ForRecord1.GroupID, sampleNsApproval1ForRecord1.Approved, sampleNsApproval1ForRecord1.ApproverID, sampleNsApproval1ForRecord1.ApproverName)
				mock.ExpectQuery(`SELECT namespace, group_name, group_id, approved, approver_id, approver_name FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(sampleRecord1.ID).WillReturnRows(rowsNs)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: []models.RequestWithNamespaceApprovers{
				{RequestData: sampleRecord1, NamespaceApprovals: []models.NamespaceApprovalInfo{sampleNsApproval1ForRecord1}},
			},
			expectDBInteraction: true,
		},
		{
			name: "Non-admin user, no query params (fetches any latest record as per current logic)",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "userID": "someUser", "username": "some@user.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "", // Default limit 1
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				// Expecting it to fetch sampleRecord2 as it's "later" (closer to 'now')
				usersJSON2, _ := json.Marshal(sampleRecord2.Users)
				approverIDsJSON2, _ := json.Marshal(sampleRecord2.ApproverIDs)
				approverNamesJSON2, _ := json.Marshal(sampleRecord2.ApproverNames)
				namespacesJSON2, _ := json.Marshal(sampleRecord2.Namespaces)

				rowsRd := sqlmock.NewRows(requestDataCols).
					AddRow(sampleRecord2.ID, sampleRecord2.CreatedAt, sampleRecord2.UpdatedAt, sampleRecord2.DeletedAt, approverIDsJSON2, approverNamesJSON2, sampleRecord2.ClusterName, sampleRecord2.RoleName, sampleRecord2.Status, sampleRecord2.Notes, sampleRecord2.UserID, sampleRecord2.Username, usersJSON2, namespacesJSON2, sampleRecord2.Justification, sampleRecord2.StartDate, sampleRecord2.EndDate, sampleRecord2.FullyApproved, sampleRecord2.Email)
				// Non-admin, no userID/username params -> no specific user filter, just ORDER and LIMIT
				mock.ExpectQuery(`SELECT \* FROM "request_data" ORDER BY created_at desc LIMIT \$1`).WithArgs(1).WillReturnRows(rowsRd)

				rowsNs := sqlmock.NewRows(nsApprovalCols).
					AddRow(sampleNsApproval1ForRecord2.Namespace, sampleNsApproval1ForRecord2.GroupName, sampleNsApproval1ForRecord2.GroupID, sampleNsApproval1ForRecord2.Approved, sampleNsApproval1ForRecord2.ApproverID, sampleNsApproval1ForRecord2.ApproverName).
					AddRow(sampleNsApproval2ForRecord2.Namespace, sampleNsApproval2ForRecord2.GroupName, sampleNsApproval2ForRecord2.GroupID, sampleNsApproval2ForRecord2.Approved, sampleNsApproval2ForRecord2.ApproverID, sampleNsApproval2ForRecord2.ApproverName)
				mock.ExpectQuery(`SELECT namespace, group_name, group_id, approved, approver_id, approver_name FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(sampleRecord2.ID).WillReturnRows(rowsNs)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: []models.RequestWithNamespaceApprovers{
				{RequestData: sampleRecord2, NamespaceApprovals: []models.NamespaceApprovalInfo{sampleNsApproval1ForRecord2, sampleNsApproval2ForRecord2}},
			},
			expectDBInteraction: true,
		},
		{
			name: "DB error when fetching request_data",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "",
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "request_data" ORDER BY created_at desc LIMIT \$1`).
					WithArgs(1).
					WillReturnError(errors.New("db query failed for request_data"))
			},
			expectedStatus:      http.StatusInternalServerError,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "Failed to fetch records"},
			expectDBInteraction: true,
		},
		{
			name: "DB error when fetching namespace_approvals",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "?limit=1",
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				usersJSON1, _ := json.Marshal(sampleRecord1.Users)
				approverIDsJSON1, _ := json.Marshal(sampleRecord1.ApproverIDs)
				approverNamesJSON1, _ := json.Marshal(sampleRecord1.ApproverNames)
				namespacesJSON1, _ := json.Marshal(sampleRecord1.Namespaces)
				rowsRd := sqlmock.NewRows(requestDataCols).
					AddRow(sampleRecord1.ID, sampleRecord1.CreatedAt, sampleRecord1.UpdatedAt, sampleRecord1.DeletedAt, approverIDsJSON1, approverNamesJSON1, sampleRecord1.ClusterName, sampleRecord1.RoleName, sampleRecord1.Status, sampleRecord1.Notes, sampleRecord1.UserID, sampleRecord1.Username, usersJSON1, namespacesJSON1, sampleRecord1.Justification, sampleRecord1.StartDate, sampleRecord1.EndDate, sampleRecord1.FullyApproved, sampleRecord1.Email)
				mock.ExpectQuery(`SELECT \* FROM "request_data" ORDER BY created_at desc LIMIT \$1`).WithArgs(1).WillReturnRows(rowsRd)

				mock.ExpectQuery(`SELECT namespace, group_name, group_id, approved, approver_id, approver_name FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(sampleRecord1.ID).WillReturnError(errors.New("db query failed for namespace_approvals"))
			},
			expectedStatus:      http.StatusInternalServerError,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "Failed to fetch namespace approvals"},
			expectDBInteraction: true,
		},
		{
			name: "No records found",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			queryParams: "?userID=nonexistentuser",
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				rowsRd := sqlmock.NewRows(requestDataCols) // No rows
				mock.ExpectQuery(`SELECT \* FROM "request_data" WHERE user_id = \$1 ORDER BY created_at desc LIMIT \$2`).
					WithArgs("nonexistentuser", 1).WillReturnRows(rowsRd)
				// No second query expected if first returns no results
			},
			expectedStatus:      http.StatusOK,
			expectedJSONBody:    []models.RequestWithNamespaceApprovers{}, // Expect empty array
			expectDBInteraction: true,
		},
	}

	var sessionSetupFunc func(t *testing.T, c *gin.Context)
	setSessionHandler := func(c *gin.Context) {
		if sessionSetupFunc != nil {
			sessionSetupFunc(t, c)
		}
		c.Status(http.StatusOK)
	}

	for _, tcLoop := range testCases {
		tc := tcLoop // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			r, mock, teardown := setupGetRecordsTest(t)
			defer teardown()

			sessionSetupFunc = tc.setupSession
			r.GET("/__set_session_for_test", setSessionHandler) // Endpoint to prime the session

			// Prime the session by making a request to the helper endpoint
			sessionSetupRecorder := httptest.NewRecorder()
			sessionSetupReq, _ := http.NewRequest(http.MethodGet, "/__set_session_for_test", nil)
			r.ServeHTTP(sessionSetupRecorder, sessionSetupReq)
			sessionCookies := sessionSetupRecorder.Result().Cookies() // Get cookies to use in actual request

			if tc.mockDB != nil {
				tc.mockDB(t, mock)
			}

			r.GET("/history", GetRecords) // Register the actual handler

			targetURL := "/history" + tc.queryParams
			w := httptest.NewRecorder()
			reqRecords, _ := http.NewRequest(http.MethodGet, targetURL, nil)
			for _, cookie := range sessionCookies { // Apply session cookies
				reqRecords.AddCookie(cookie)
			}
			r.ServeHTTP(w, reqRecords)

			assert.Equal(t, tc.expectedStatus, w.Code, "HTTP status code mismatch")
			if tc.expectedJSONBody != nil {
				if w.Code == http.StatusOK || (w.Code == http.StatusInternalServerError && tc.expectedStatus == http.StatusInternalServerError) { // Only check body for expected success or specific error structures
					expectedBodyBytes, err := json.Marshal(tc.expectedJSONBody)
					assert.NoError(t, err, "Failed to marshal expectedJSONBody")
					assert.JSONEq(t, string(expectedBodyBytes), w.Body.String(), "HTTP response body mismatch")
				}
			} else if w.Body.Len() > 0 && w.Code == http.StatusOK { // If no body expected but got one on OK
				assert.Fail(t, "Expected empty body but got: "+w.Body.String())
			}

			if tc.expectDBInteraction {
				assert.NoError(t, mock.ExpectationsWereMet(), "SQLmock expectations not met for a DB interaction test")
			} else {
				assert.NoError(t, mock.ExpectationsWereMet(), "SQLmock expectations not met for a non-DB interaction test (should be no pending query/exec expectations)")
			}
		})
	}
}
