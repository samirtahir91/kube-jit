package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
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

var originalDBRecordsTestGetPending *gorm.DB

// setupGetPendingApprovalsTest configures a Gin router with a mocked DB and session management for GetPendingApprovals tests.
func setupGetPendingApprovalsTest(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
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
		Logger: gormlogger.Default.LogMode(gormlogger.Silent), // Use gormlogger.Info for verbose GORM logs
	})
	if err != nil {
		t.Fatalf("Failed to open gorm database: %s", err)
	}

	originalDBRecordsTestGetPending = db.DB
	db.DB = gormDB

	store := cookie.NewStore([]byte("test-secret-key-records"))
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
		db.DB = originalDBRecordsTestGetPending
		// mockDb.Close() error is checked implicitly by sqlmock if expectations are not met.
		// Explicitly checking err here can be noisy if other expectations failed first.
		_ = mockDb.Close()
	}

	return r, mock, teardown
}

func TestGetPendingApprovals(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Consistent time for tests

	// Expected data for admin/platform approver paths
	// GormModel.ID is set to match what sqlmock will provide (1-based index from loop).
	// Other GormModel fields (CreatedAt, UpdatedAt) are left as zero time; if the handler/GORM sets them,
	// this expectation needs to match. Based on current logs, they are zero.
	expectedAdminRequests := []models.RequestData{
		{GormModel: models.GormModel{ID: 1}, ClusterName: "cluster-a", RoleName: "role-x", Status: "Requested", UserID: "user1", Username: "user1@example.com", Justification: "Admin request 1", StartDate: now, EndDate: now.Add(24 * time.Hour), Users: []string{"user1@example.com"}},
		{GormModel: models.GormModel{ID: 2}, ClusterName: "cluster-b", RoleName: "role-y", Status: "Requested", UserID: "user2", Username: "user2@example.com", Justification: "Admin request 2", StartDate: now, EndDate: now.Add(48 * time.Hour), Users: []string{"user2@example.com"}},
	}

	usersReq1JSON, _ := json.Marshal([]string{"reqUser1@example.com"})
	nonAdminRow1 := PendingRequestRow{
		ID: 10, ClusterName: "cluster-c", RoleName: "role-z", UserID: "reqUser1", Username: "reqUser1@example.com", Justification: "Needs access", StartDate: now, EndDate: now.Add(time.Hour), CreatedAt: now, Users: []string{"reqUser1@example.com"},
		Namespace: "ns1", GroupID: "group1", GroupName: "Group One", Approved: false, // Status will be "" as not selected in JOIN query
	}
	nonAdminRow1Ns2 := PendingRequestRow{ // Same request, different namespace part
		ID: 10, ClusterName: "cluster-c", RoleName: "role-z", UserID: "reqUser1", Username: "reqUser1@example.com", Justification: "Needs access", StartDate: now, EndDate: now.Add(time.Hour), CreatedAt: now, Users: []string{"reqUser1@example.com"},
		Namespace: "ns1-b", GroupID: "group1", GroupName: "Group One", Approved: false,
	}
	usersReq2JSON, _ := json.Marshal([]string{"reqUser2@example.com"})
	nonAdminRow2 := PendingRequestRow{
		ID: 11, ClusterName: "cluster-d", RoleName: "role-w", UserID: "reqUser2", Username: "reqUser2@example.com", Justification: "More access", StartDate: now, EndDate: now.Add(2 * time.Hour), CreatedAt: now, Users: []string{"reqUser2@example.com"},
		Namespace: "ns2", GroupID: "group2", GroupName: "Group Two", Approved: false,
	}

	// For []any approver groups test
	usersReq3JSON, _ := json.Marshal([]string{"reqUser3@example.com"})
	nonAdminRow3AnyGroup := PendingRequestRow{
		ID: 12, ClusterName: "cluster-e", RoleName: "role-v", UserID: "reqUser3", Username: "reqUser3@example.com", Justification: "Access for any group type", StartDate: now, EndDate: now.Add(3 * time.Hour), CreatedAt: now, Users: []string{"reqUser3@example.com"},
		Namespace: "ns3", GroupID: "groupX", GroupName: "Group X", Approved: false,
	}

	testCases := []struct {
		name                string
		setupSession        func(t *testing.T, c *gin.Context) // setupSession still takes a *gin.Context
		mockDB              func(t *testing.T, mock sqlmock.Sqlmock)
		expectedStatus      int
		expectedJSONBody    interface{}
		expectDBInteraction bool
	}{
		{
			name: "Admin user successfully fetches all pending requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true, "isPlatformApprover": false, "userID": "adminUser", "username": "admin@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				// Define all columns that GORM might select for models.RequestData
				// This list should match all fields in models.RequestData and its embedded models.GormModel
				cols := []string{"id", "created_at", "updated_at", "deleted_at", "approver_ids", "approver_names", "cluster_name", "role_name", "status", "notes", "user_id", "username", "users", "namespaces", "justification", "start_date", "end_date", "fully_approved", "email"}
				rows := sqlmock.NewRows(cols)
				for _, req := range expectedAdminRequests { // Use the version with IDs
					usersJSON, _ := json.Marshal(req.Users)
					approverIDsJSON, _ := json.Marshal(req.ApproverIDs)
					approverNamesJSON, _ := json.Marshal(req.ApproverNames)
					namespacesJSON, _ := json.Marshal(req.Namespaces)

					rows.AddRow(req.ID, req.CreatedAt, req.UpdatedAt, req.DeletedAt, approverIDsJSON, approverNamesJSON, req.ClusterName, req.RoleName, req.Status, req.Notes, req.UserID, req.Username, usersJSON, namespacesJSON, req.Justification, req.StartDate, req.EndDate, req.FullyApproved, req.Email)
				}
				mock.ExpectQuery(`SELECT .* FROM "request_data" WHERE status = \$1`). // GORM generates specific cols, but .* is fine for mock
													WithArgs("Requested").
													WillReturnRows(rows)
			},
			expectedStatus:      http.StatusOK,
			expectedJSONBody:    gin.H{"pendingRequests": expectedAdminRequests}, // Use the version with IDs
			expectDBInteraction: true,
		},
		{
			name: "Platform approver user successfully fetches all pending requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": true, "userID": "platformUser", "username": "platform@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) { // Same mock as admin
				cols := []string{"id", "created_at", "updated_at", "deleted_at", "approver_ids", "approver_names", "cluster_name", "role_name", "status", "notes", "user_id", "username", "users", "namespaces", "justification", "start_date", "end_date", "fully_approved", "email"}
				rows := sqlmock.NewRows(cols)
				for _, req := range expectedAdminRequests {
					usersJSON, _ := json.Marshal(req.Users)
					approverIDsJSON, _ := json.Marshal(req.ApproverIDs)
					approverNamesJSON, _ := json.Marshal(req.ApproverNames)
					namespacesJSON, _ := json.Marshal(req.Namespaces)
					rows.AddRow(req.ID, req.CreatedAt, req.UpdatedAt, req.DeletedAt, approverIDsJSON, approverNamesJSON, req.ClusterName, req.RoleName, req.Status, req.Notes, req.UserID, req.Username, usersJSON, namespacesJSON, req.Justification, req.StartDate, req.EndDate, req.FullyApproved, req.Email)
				}
				mock.ExpectQuery(`SELECT .* FROM "request_data" WHERE status = \$1`).
					WithArgs("Requested").
					WillReturnRows(rows)
			},
			expectedStatus:      http.StatusOK,
			expectedJSONBody:    gin.H{"pendingRequests": expectedAdminRequests},
			expectDBInteraction: true,
		},
		{
			name: "Non-admin user with approver groups (models.Team) fetches specific pending requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				approverGroups := []models.Team{{ID: "group1", Name: "Group One"}, {ID: "group2", Name: "Group Two"}}
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "approverGroups": approverGroups, "userID": "normalUser", "username": "normal@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				cols := []string{"id", "cluster_name", "role_name", "user_id", "username", "justification", "start_date", "end_date", "created_at", "users", "namespace", "group_id", "group_name", "approved"}
				rows := sqlmock.NewRows(cols).
					AddRow(nonAdminRow1.ID, nonAdminRow1.ClusterName, nonAdminRow1.RoleName, nonAdminRow1.UserID, nonAdminRow1.Username, nonAdminRow1.Justification, nonAdminRow1.StartDate, nonAdminRow1.EndDate, nonAdminRow1.CreatedAt, usersReq1JSON, nonAdminRow1.Namespace, nonAdminRow1.GroupID, nonAdminRow1.GroupName, nonAdminRow1.Approved).
					AddRow(nonAdminRow1Ns2.ID, nonAdminRow1Ns2.ClusterName, nonAdminRow1Ns2.RoleName, nonAdminRow1Ns2.UserID, nonAdminRow1Ns2.Username, nonAdminRow1Ns2.Justification, nonAdminRow1Ns2.StartDate, nonAdminRow1Ns2.EndDate, nonAdminRow1Ns2.CreatedAt, usersReq1JSON, nonAdminRow1Ns2.Namespace, nonAdminRow1Ns2.GroupID, nonAdminRow1Ns2.GroupName, nonAdminRow1Ns2.Approved).
					AddRow(nonAdminRow2.ID, nonAdminRow2.ClusterName, nonAdminRow2.RoleName, nonAdminRow2.UserID, nonAdminRow2.Username, nonAdminRow2.Justification, nonAdminRow2.StartDate, nonAdminRow2.EndDate, nonAdminRow2.CreatedAt, usersReq2JSON, nonAdminRow2.Namespace, nonAdminRow2.GroupID, nonAdminRow2.GroupName, nonAdminRow2.Approved)

				expectedQuery := regexp.QuoteMeta(`SELECT request_data.id, request_data.cluster_name, request_data.role_name, request_data.user_id, request_data.username, request_data.justification, request_data.start_date, request_data.end_date, request_data.created_at, request_data.users, request_namespaces.namespace, request_namespaces.group_id, request_namespaces.group_name, request_namespaces.approved FROM "request_data" JOIN request_namespaces ON request_namespaces.request_id = request_data.id WHERE request_namespaces.group_id IN ($1,$2) AND request_data.status = $3 AND request_namespaces.approved = false`)
				mock.ExpectQuery(expectedQuery).
					WithArgs("group1", "group2", "Requested").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: gin.H{"pendingRequests": []PendingRequest{
				{ID: nonAdminRow1.ID, ClusterName: nonAdminRow1.ClusterName, RoleName: nonAdminRow1.RoleName, Status: "", UserID: nonAdminRow1.UserID, Users: nonAdminRow1.Users, Username: nonAdminRow1.Username, Justification: nonAdminRow1.Justification, StartDate: nonAdminRow1.StartDate, EndDate: nonAdminRow1.EndDate, CreatedAt: nonAdminRow1.CreatedAt, Namespaces: []string{nonAdminRow1.Namespace, nonAdminRow1Ns2.Namespace}, GroupIDs: []string{nonAdminRow1.GroupID, nonAdminRow1Ns2.GroupID}, GroupNames: []string{nonAdminRow1.GroupName, nonAdminRow1Ns2.GroupName}, ApprovedList: []bool{nonAdminRow1.Approved, nonAdminRow1Ns2.Approved}},
				{ID: nonAdminRow2.ID, ClusterName: nonAdminRow2.ClusterName, RoleName: nonAdminRow2.RoleName, Status: "", UserID: nonAdminRow2.UserID, Users: nonAdminRow2.Users, Username: nonAdminRow2.Username, Justification: nonAdminRow2.Justification, StartDate: nonAdminRow2.StartDate, EndDate: nonAdminRow2.EndDate, CreatedAt: nonAdminRow2.CreatedAt, Namespaces: []string{nonAdminRow2.Namespace}, GroupIDs: []string{nonAdminRow2.GroupID}, GroupNames: []string{nonAdminRow2.GroupName}, ApprovedList: []bool{nonAdminRow2.Approved}},
			}},
			expectDBInteraction: true,
		},
		{
			name: "Non-admin user with approver groups (any type) fetches specific pending requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				approverGroups := []any{
					map[string]any{"id": "groupX", "name": "Group X"},
				}
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "approverGroups": approverGroups, "userID": "anyGroupUser", "username": "anygroup@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				cols := []string{"id", "cluster_name", "role_name", "user_id", "username", "justification", "start_date", "end_date", "created_at", "users", "namespace", "group_id", "group_name", "approved"}
				rows := sqlmock.NewRows(cols).
					AddRow(nonAdminRow3AnyGroup.ID, nonAdminRow3AnyGroup.ClusterName, nonAdminRow3AnyGroup.RoleName, nonAdminRow3AnyGroup.UserID, nonAdminRow3AnyGroup.Username, nonAdminRow3AnyGroup.Justification, nonAdminRow3AnyGroup.StartDate, nonAdminRow3AnyGroup.EndDate, nonAdminRow3AnyGroup.CreatedAt, usersReq3JSON, nonAdminRow3AnyGroup.Namespace, nonAdminRow3AnyGroup.GroupID, nonAdminRow3AnyGroup.GroupName, nonAdminRow3AnyGroup.Approved)

				expectedQuery := regexp.QuoteMeta(`SELECT request_data.id, request_data.cluster_name, request_data.role_name, request_data.user_id, request_data.username, request_data.justification, request_data.start_date, request_data.end_date, request_data.created_at, request_data.users, request_namespaces.namespace, request_namespaces.group_id, request_namespaces.group_name, request_namespaces.approved FROM "request_data" JOIN request_namespaces ON request_namespaces.request_id = request_data.id WHERE request_namespaces.group_id IN ($1) AND request_data.status = $2 AND request_namespaces.approved = false`)
				mock.ExpectQuery(expectedQuery).
					WithArgs("groupX", "Requested").
					WillReturnRows(rows)
			},
			expectedStatus: http.StatusOK,
			expectedJSONBody: gin.H{"pendingRequests": []PendingRequest{
				{ID: nonAdminRow3AnyGroup.ID, ClusterName: nonAdminRow3AnyGroup.ClusterName, RoleName: nonAdminRow3AnyGroup.RoleName, Status: "", UserID: nonAdminRow3AnyGroup.UserID, Users: nonAdminRow3AnyGroup.Users, Username: nonAdminRow3AnyGroup.Username, Justification: nonAdminRow3AnyGroup.Justification, StartDate: nonAdminRow3AnyGroup.StartDate, EndDate: nonAdminRow3AnyGroup.EndDate, CreatedAt: nonAdminRow3AnyGroup.CreatedAt, Namespaces: []string{nonAdminRow3AnyGroup.Namespace}, GroupIDs: []string{nonAdminRow3AnyGroup.GroupID}, GroupNames: []string{nonAdminRow3AnyGroup.GroupName}, ApprovedList: []bool{nonAdminRow3AnyGroup.Approved}},
			}},
			expectDBInteraction: true,
		},
		{
			name: "Non-admin user with approver groups but no matching pending requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				approverGroups := []models.Team{{ID: "groupNoMatch", Name: "Group No Match"}}
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "approverGroups": approverGroups, "userID": "noMatchUser", "username": "nomatch@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				cols := []string{"id", "cluster_name", "role_name", "user_id", "username", "justification", "start_date", "end_date", "created_at", "users", "namespace", "group_id", "group_name", "approved"}
				rows := sqlmock.NewRows(cols) // No rows added

				expectedQuery := regexp.QuoteMeta(`SELECT request_data.id, request_data.cluster_name, request_data.role_name, request_data.user_id, request_data.username, request_data.justification, request_data.start_date, request_data.end_date, request_data.created_at, request_data.users, request_namespaces.namespace, request_namespaces.group_id, request_namespaces.group_name, request_namespaces.approved FROM "request_data" JOIN request_namespaces ON request_namespaces.request_id = request_data.id WHERE request_namespaces.group_id IN ($1) AND request_data.status = $2 AND request_namespaces.approved = false`)
				mock.ExpectQuery(expectedQuery).
					WithArgs("groupNoMatch", "Requested").
					WillReturnRows(rows)
			},
			expectedStatus:      http.StatusOK,
			expectedJSONBody:    gin.H{"pendingRequests": []PendingRequest{}},
			expectDBInteraction: true,
		},
		{
			name: "Unauthorized if non-admin and no approver groups in session",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "userID": "noGroupUser", "username": "nogroup@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				// No DB interaction expected
			},
			expectedStatus:      http.StatusUnauthorized,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "Unauthorized: no approver groups in session"},
			expectDBInteraction: false,
		},
		{
			name: "Unauthorized if non-admin and empty approver groups in session",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "approverGroups": []models.Team{}, "userID": "emptyGroupUser", "username": "emptygroup@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				// No DB interaction expected
			},
			expectedStatus:      http.StatusUnauthorized,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "Unauthorized: no approver groups in session"},
			expectDBInteraction: false,
		},
		{
			name: "DB error when admin fetches requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				sessionData := map[string]any{"isAdmin": true, "isPlatformApprover": false, "userID": "adminUser", "username": "admin@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .* FROM "request_data" WHERE status = \$1`).
					WithArgs("Requested").
					WillReturnError(errors.New("db query failed for admin"))
			},
			expectedStatus:      http.StatusInternalServerError,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "db query failed for admin"},
			expectDBInteraction: true,
		},
		{
			name: "DB error when non-admin fetches requests",
			setupSession: func(t *testing.T, c *gin.Context) {
				session := sessions.Default(c)
				approverGroups := []models.Team{{ID: "group1"}}
				sessionData := map[string]any{"isAdmin": false, "isPlatformApprover": false, "approverGroups": approverGroups, "userID": "normalUserErr", "username": "normalerr@example.com"}
				session.Set("sessionData", sessionData)
				assert.NoError(t, session.Save())
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				expectedQuery := regexp.QuoteMeta(`SELECT request_data.id, request_data.cluster_name, request_data.role_name, request_data.user_id, request_data.username, request_data.justification, request_data.start_date, request_data.end_date, request_data.created_at, request_data.users, request_namespaces.namespace, request_namespaces.group_id, request_namespaces.group_name, request_namespaces.approved FROM "request_data" JOIN request_namespaces ON request_namespaces.request_id = request_data.id WHERE request_namespaces.group_id IN ($1) AND request_data.status = $2 AND request_namespaces.approved = false`)
				mock.ExpectQuery(expectedQuery).
					WithArgs("group1", "Requested").
					WillReturnError(errors.New("db query failed for non-admin"))
			},
			expectedStatus:      http.StatusInternalServerError,
			expectedJSONBody:    models.SimpleMessageResponse{Error: "db query failed for non-admin"},
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
			r, mock, teardown := setupGetPendingApprovalsTest(t)
			defer teardown()

			sessionSetupFunc = tc.setupSession
			r.GET("/__set_session_for_test", setSessionHandler)

			sessionSetupRecorder := httptest.NewRecorder()
			sessionSetupReq, _ := http.NewRequest(http.MethodGet, "/__set_session_for_test", nil)
			r.ServeHTTP(sessionSetupRecorder, sessionSetupReq)
			sessionCookies := sessionSetupRecorder.Result().Cookies()

			if tc.mockDB != nil {
				tc.mockDB(t, mock)
			}

			r.GET("/approvals", GetPendingApprovals)

			w := httptest.NewRecorder()
			reqApprovals, _ := http.NewRequest(http.MethodGet, "/approvals", nil)
			for _, cookie := range sessionCookies {
				reqApprovals.AddCookie(cookie)
			}
			r.ServeHTTP(w, reqApprovals)

			assert.Equal(t, tc.expectedStatus, w.Code, "HTTP status code mismatch")
			if tc.expectedJSONBody != nil {
				expectedBodyBytes, err := json.Marshal(tc.expectedJSONBody)
				assert.NoError(t, err, "Failed to marshal expectedJSONBody")
				assert.JSONEq(t, string(expectedBodyBytes), w.Body.String(), "HTTP response body mismatch")
			}

			// Check expectations after the handler has run and before teardown.
			// This ensures all DB interactions expected by tc.mockDB were met.
			// The teardown will handle mockDb.Close().
			if tc.expectDBInteraction {
				assert.NoError(t, mock.ExpectationsWereMet(), "SQLmock expectations not met for a DB interaction test")
			} else {
				// If no DB interaction is expected, still check ExpectationsWereMet.
				// This ensures no *unexpected* DB calls were made.
				assert.NoError(t, mock.ExpectationsWereMet(), "SQLmock expectations not met for a non-DB interaction test (should be no pending query/exec expectations)")
			}
		})
	}
}
