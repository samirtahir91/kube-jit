package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"kube-jit/internal/db"
	"kube-jit/internal/models"
	"kube-jit/pkg/email"
	"kube-jit/pkg/k8s"

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

var originalDBRequestTest *gorm.DB
var originalK8sValidateNamespaces func(clusterName string, namespaces []string) (map[string]struct {
	GroupID   string
	GroupName string
}, error)
var originalK8sCreateK8sObject func(request models.RequestData, approverName string) error
var originalEmailSendMail func(to, subject, body string) error
var emailSent chan struct{}
var emailSentOnce sync.Once

// setupRequestTest configures a Gin router with a mocked DB and session management for request handler tests.
func setupRequestTest(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	var testLogger = zap.NewNop() // Use zap.NewDevelopment() for verbose logs during debugging
	InitLogger(testLogger)        // Initialize logger for handlers package
	db.InitLogger(testLogger)     // Initialize logger for db package
	k8s.InitLogger(testLogger)    // Initialize logger for k8s package
	// email.InitLogger(testLogger) // Assuming email package might also have an InitLogger

	mockDb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("Failed to open sqlmock database: %s", err)
	}

	dialector := postgres.New(postgres.Config{
		Conn:                 mockDb,
		PreferSimpleProtocol: true,
	})
	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info), // CHANGED TO Info
	})
	if err != nil {
		t.Fatalf("Failed to open gorm database: %s", err)
	}

	originalDBRequestTest = db.DB
	db.DB = gormDB

	// Store original functions for mocking
	originalK8sValidateNamespaces = k8s.ValidateNamespaces
	originalK8sCreateK8sObject = k8s.CreateK8sObject
	originalEmailSendMail = email.SendMail

	store := cookie.NewStore([]byte("test-secret-key-request"))
	r.Use(sessions.Sessions("kube_jit_session", store))

	// Middleware to inject logger
	r.Use(func(c *gin.Context) {
		c.Set("logger", testLogger)
		c.Next()
	})

	// Middleware to inject sessionData (simulating what auth middleware would do)
	// This will be customized per test case by setting session values directly
	// before making the request, or by having the test case's setupSession func do it.
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		sessionDataFromStore := s.Get("sessionData")
		var dataToSetInContext map[string]interface{}

		if sessionDataFromStore != nil {
			if m, ok := sessionDataFromStore.(map[string]interface{}); ok {
				dataToSetInContext = m
			} else if mAny, okAny := sessionDataFromStore.(map[string]any); okAny {
				dataToSetInContext = make(map[string]interface{})
				for k, v := range mAny {
					dataToSetInContext[k] = v
				}
			} else {
				// Fallback or error if type is unexpected
				dataToSetInContext = make(map[string]interface{})
				t.Logf("Warning: sessionData in store was not map[string]interface{} or map[string]any, but %T", sessionDataFromStore)
			}
		} else {
			dataToSetInContext = make(map[string]interface{})
		}
		c.Set("sessionData", dataToSetInContext)
		c.Next()
	})

	teardown := func() {
		db.DB = originalDBRequestTest
		k8s.ValidateNamespaces = originalK8sValidateNamespaces
		k8s.CreateK8sObject = originalK8sCreateK8sObject
		email.SendMail = originalEmailSendMail
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
		_ = mockDb.Close()
	}

	return r, mock, teardown
}

func TestSubmitRequest(t *testing.T) {
	// Define column names for sqlmock
	requestDataCols := []string{"id", "created_at", "updated_at", "deleted_at", "cluster_name", "role_name", "status", "user_id", "username", "users", "namespaces", "justification", "start_date", "end_date", "email", "approver_ids", "approver_names", "fully_approved", "notes"}
	_ = requestDataCols // Prevent "declared and not used" error if not immediately used
	requestNamespaceCols := []string{"id", "request_id", "namespace", "group_id", "group_name", "approved", "approver_id", "approver_name"}
	_ = requestNamespaceCols

	sampleTime := time.Now().Truncate(time.Second)

	testCases := []struct {
		name                      string
		setupSession              func(s sessions.Session)
		payload                   SubmitRequestPayload
		mockK8sValidateNamespaces func() // To set up the mock for k8s.ValidateNamespaces
		mockDB                    func(t *testing.T, mock sqlmock.Sqlmock, payload SubmitRequestPayload, expectedRequestID uint)
		mockEmail                 func() // To set up the mock for email.SendMail
		expectedStatus            int
		expectedBody              interface{} // Can be models.SimpleMessageResponse or other specific response
	}{
		{
			name: "Successful request submission",
			setupSession: func(s sessions.Session) {
				s.Set("sessionData", map[string]interface{}{
					"email":    "testuser@example.com",
					"id":       "testuser",
					"name":     "Test User",
					"provider": "github",
				})
			},
			payload: SubmitRequestPayload{
				Role:          models.Roles{Name: "view"},
				ClusterName:   models.Cluster{Name: "test-cluster"},
				UserID:        "testuser",
				Username:      "Test User",
				Users:         []string{"testuser@example.com"},
				Namespaces:    []string{"ns1", "ns2"},
				Justification: "Need access for testing",
				StartDate:     sampleTime,
				EndDate:       sampleTime.Add(1 * time.Hour),
			},
			mockK8sValidateNamespaces: func() {
				k8s.ValidateNamespaces = func(clusterName string, namespaces []string) (map[string]struct {
					GroupID   string
					GroupName string
				}, error) {
					assert.Equal(t, "test-cluster", clusterName)
					assert.ElementsMatch(t, []string{"ns1", "ns2"}, namespaces)
					return map[string]struct {
						GroupID   string
						GroupName string
					}{
						"ns1": {GroupID: "group1", GroupName: "Group One"},
						"ns2": {GroupID: "group2", GroupName: "Group Two"},
					}, nil
				}
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock, payload SubmitRequestPayload, expectedRequestID uint) {
				// Expect RequestData creation
				mock.ExpectBegin()
				mock.ExpectQuery(`INSERT INTO "request_data"`).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedRequestID))
				mock.ExpectCommit()

				// Regex for matching the specific INSERT statement for request_namespaces
				nsInsertRegex := `INSERT INTO "request_namespaces"`

				// Expect RequestNamespace creation for ns1 and ns2, in any order
				mock.ExpectBegin()
				mock.ExpectQuery(nsInsertRegex).
					WithArgs(expectedRequestID, "ns1", "group1", "Group One", false, "", "").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
				mock.ExpectCommit()

				mock.ExpectBegin()
				mock.ExpectQuery(nsInsertRegex).
					WithArgs(expectedRequestID, "ns2", "group2", "Group Two", false, "", "").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(102))
				mock.ExpectCommit()
			},
			mockEmail: func() {
				email.SendMail = func(to, subject, body string) error {
					assert.Equal(t, "testuser@example.com", to)
					assert.Contains(t, subject, "Your JIT request #1 has been submitted") // Assuming ID 1
					// Optionally, assert body contents
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   models.SimpleMessageResponse{Message: "Request submitted successfully"},
		},
		// Add more test cases:
		// - Invalid request data (binding error)
		// - k8s.ValidateNamespaces returns an error
		// - DB error on RequestData creation
		// - DB error on RequestNamespace creation
		// - Missing email in session (should still proceed but not send email)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, mock, teardown := setupRequestTest(t)
			defer teardown()

			// Setup mocks
			if tc.mockK8sValidateNamespaces != nil {
				tc.mockK8sValidateNamespaces()
			}
			if tc.mockEmail != nil {
				tc.mockEmail()
			}
			// The expectedRequestID for mockDB might need to be dynamic if not always 1
			if tc.mockDB != nil {
				tc.mockDB(t, mock, tc.payload, 1) // Assuming first request ID is 1 for simplicity
			}

			// Prepare request
			jsonPayload, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest(http.MethodPost, "/submit-request", bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")

			// Setup session for this specific test case
			// We need to create a temporary recorder and context to set session values
			// then extract the cookie to use in the actual request.
			sessionSetupRecorder := httptest.NewRecorder()
			_, tempEngine := gin.CreateTestContext(sessionSetupRecorder)
			tempEngine.Use(sessions.Sessions("kube_jit_session", cookie.NewStore([]byte("test-secret-key-request"))))
			tempEngine.GET("/setup_session_endpoint", func(c *gin.Context) {
				s := sessions.Default(c)
				if tc.setupSession != nil {
					tc.setupSession(s)
				}
				s.Save()
				c.Status(http.StatusOK)
			})
			tempSetupReq, _ := http.NewRequest(http.MethodGet, "/setup_session_endpoint", nil)
			tempEngine.ServeHTTP(sessionSetupRecorder, tempSetupReq)

			for _, cookie := range sessionSetupRecorder.Result().Cookies() {
				if cookie.Name == "kube_jit_session" { // Or your actual session cookie name if split
					req.AddCookie(cookie)
					break
				}
			}
			// If using split cookies, you'd add all relevant cookies here.

			// Serve request
			w := httptest.NewRecorder()
			r.POST("/submit-request", SubmitRequest) // Ensure the route is registered
			r.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != nil {
				expectedJSON, _ := json.Marshal(tc.expectedBody)
				assert.JSONEq(t, string(expectedJSON), w.Body.String())
			}
		})
	}
}

func TestApproveOrRejectRequests(t *testing.T) {
	// Define column names for sqlmock
	requestDataCols := []string{"id", "created_at", "updated_at", "deleted_at", "cluster_name", "role_name", "status", "user_id", "username", "users", "namespaces", "justification", "start_date", "end_date", "email", "approver_ids", "approver_names", "fully_approved", "notes"}
	requestNamespaceCols := []string{"id", "request_id", "namespace", "group_id", "group_name", "approved", "approver_id", "approver_name"}

	sampleTime := time.Now().Truncate(time.Second)

	var emailSent chan struct{}

	testCases := []struct {
		name                   string
		setupSession           func(s sessions.Session)
		payload                interface{} // AdminApproveRequest or UserApproveRequest
		mockDB                 func(t *testing.T, mock sqlmock.Sqlmock)
		mockK8sCreateK8sObject func() // To set up the mock for k8s.CreateK8sObject
		mockEmail              func() // To set up the mock for email.SendMail
		expectedStatus         int
		expectedBody           interface{}
	}{
		{
			name: "Admin successfully approves a request",
			setupSession: func(s sessions.Session) {
				s.Set("sessionData", map[string]interface{}{
					"isAdmin":            true,
					"id":                 "admin001",
					"name":               "Admin User",
					"email":              "requestor@example.com", // Email of the original requestor for notification
					"isPlatformApprover": false,
				})
			},
			payload: AdminApproveRequest{
				ApproverID:   "admin001",
				ApproverName: "Admin User",
				Status:       "Approved",
				Requests: []models.RequestData{
					{
						GormModel:     models.GormModel{ID: 1},
						ClusterName:   "test-cluster",
						RoleName:      "view",
						UserID:        "user123",
						Username:      "User OneTwoThree",
						Users:         []string{"user123@example.com"},
						Namespaces:    []string{"ns-a", "ns-b"}, // These are from original request
						Justification: "Admin approval test",
						StartDate:     sampleTime,
						EndDate:       sampleTime.Add(2 * time.Hour),
						Email:         "requestor@example.com",
					},
				},
			},
			mockDB: func(t *testing.T, mock sqlmock.Sqlmock) {
				requestID := uint(1)
				nsIDA := uint(10) // Assuming ID for ns-a
				nsIDB := uint(11) // Assuming ID for ns-b

				// 1. Expect fetch namespaces for the request
				nsRows := sqlmock.NewRows(requestNamespaceCols).
					AddRow(nsIDA, requestID, "ns-a", "groupA", "Group A", false, "", "").
					AddRow(nsIDB, requestID, "ns-b", "groupB", "Group B", false, "", "")
				mock.ExpectQuery(`SELECT \* FROM "request_namespaces" WHERE request_id = \$1`).
					WithArgs(requestID).
					WillReturnRows(nsRows)

				// 2. Expect save for each namespace (Approved=true, approver details set)
				// GORM is NOT including updated_at in this specific update.
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "request_namespaces" SET "request_id"=\$1,"namespace"=\$2,"group_id"=\$3,"group_name"=\$4,"approved"=\$5,"approver_id"=\$6,"approver_name"=\$7 WHERE "id" = \$8`).
					WithArgs(requestID, "ns-a", "groupA", "Group A", true, "admin001", "Admin User", nsIDA).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()

				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "request_namespaces" SET "request_id"=\$1,"namespace"=\$2,"group_id"=\$3,"group_name"=\$4,"approved"=\$5,"approver_id"=\$6,"approver_name"=\$7 WHERE "id" = \$8`).
					WithArgs(requestID, "ns-b", "groupB", "Group B", true, "admin001", "Admin User", nsIDB).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()

				// 3. Expect fetch request record (to append approver IDs/Names)
				// Ensure ApproverIDs and ApproverNames are initialized if they could be nil (e.g. as []byte("null") or actual empty JSON array)
				// For simplicity, assuming they are initially empty JSON arrays `[]` which GORM handles as `[]string{}`
				initialApproverIDsJSON, _ := json.Marshal([]string{})
				initialApproverNamesJSON, _ := json.Marshal([]string{})

				reqDataRow := sqlmock.NewRows(requestDataCols).
					AddRow(requestID, sampleTime, sampleTime, nil, "test-cluster", "view", "Requested", "user123", "User OneTwoThree", `["user123@example.com"]`, `["ns-a","ns-b"]`, "Admin approval test", sampleTime, sampleTime.Add(2*time.Hour), "requestor@example.com", initialApproverIDsJSON, initialApproverNamesJSON, false, "")
				// Removed ASC from ORDER BY clause to match GORM's actual query
				mock.ExpectQuery(`SELECT \* FROM "request_data" WHERE "request_data"."id" = \$1 ORDER BY "request_data"."id" LIMIT \$2`).
					WithArgs(requestID, 1).
					WillReturnRows(reqDataRow)

				// 4. Expect update request status, approvers, fully_approved
				// GORM's actual order: "updated_at", "approver_ids", "approver_names", "status", "fully_approved"
				// For JSON fields, GORM/driver might send string representation of JSON
				expectedApproverIDsStr := `["admin001"]`
				expectedApproverNamesStr := `["Admin User"]`
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE "request_data" SET "updated_at"=\$1,"approver_ids"=\$2,"approver_names"=\$3,"status"=\$4,"fully_approved"=\$5 WHERE "id" = \$6`).
					WithArgs(sqlmock.AnyArg(), expectedApproverIDsStr, expectedApproverNamesStr, "Approved", true, requestID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			mockK8sCreateK8sObject: func() {
				k8s.CreateK8sObject = func(request models.RequestData, approverName string) error {
					assert.Equal(t, uint(1), request.ID)
					assert.Equal(t, "Admin User", approverName)
					assert.ElementsMatch(t, []string{"ns-a", "ns-b"}, request.Namespaces) // Namespaces should be the ones from dbNamespaces
					return nil
				}
			},
			mockEmail: func() {
				emailSent = make(chan struct{})
				emailSentOnce = sync.Once{}
				email.SendMail = func(to, subject, body string) error {
					assert.Equal(t, "requestor@example.com", to)
					assert.Contains(t, subject, "Your JIT request #1 is now Approved")
					emailSentOnce.Do(func() { close(emailSent) })
					return nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   models.SimpleMessageResponse{Message: "Admin/Platform requests processed successfully"},
		},
		// Add more test cases:
		// - Admin rejects a request
		// - Platform approver approves/rejects
		// - Non-admin approves a namespace (authorized)
		// - Non-admin tries to approve (unauthorized for group)
		// - Non-admin rejects a namespace
		// - Request becomes partially approved (not all namespaces approved by one user)
		// - Request becomes fully approved after multiple non-admin approvals
		// - Invalid payload
		// - DB errors
		// - k8s.CreateK8sObject error
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, mock, teardown := setupRequestTest(t)
			defer teardown()

			if tc.mockDB != nil {
				tc.mockDB(t, mock)
			}
			if tc.mockK8sCreateK8sObject != nil {
				tc.mockK8sCreateK8sObject()
			}
			if tc.mockEmail != nil {
				tc.mockEmail()
			}

			jsonPayload, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest(http.MethodPost, "/approve-reject", bytes.NewBuffer(jsonPayload))
			req.Header.Set("Content-Type", "application/json")

			sessionSetupRecorder := httptest.NewRecorder()
			_, tempEngine := gin.CreateTestContext(sessionSetupRecorder)
			tempEngine.Use(sessions.Sessions("kube_jit_session", cookie.NewStore([]byte("test-secret-key-request"))))
			tempEngine.GET("/setup_session_endpoint_approve", func(c *gin.Context) {
				s := sessions.Default(c)
				if tc.setupSession != nil {
					tc.setupSession(s)
				}
				s.Save()
				c.Status(http.StatusOK)
			})
			tempSetupReq, _ := http.NewRequest(http.MethodGet, "/setup_session_endpoint_approve", nil)
			tempEngine.ServeHTTP(sessionSetupRecorder, tempSetupReq)

			for _, cookie := range sessionSetupRecorder.Result().Cookies() {
				// Adapt if using split cookies
				if cookie.Name == "kube_jit_session" {
					req.AddCookie(cookie)
					break
				}
			}

			// Serve request
			w := httptest.NewRecorder()
			r.POST("/approve-reject", ApproveOrRejectRequests)
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != nil {
				expectedJSON, _ := json.Marshal(tc.expectedBody)
				assert.JSONEq(t, string(expectedJSON), w.Body.String())
			}

			// Wait for email goroutine (if this test expects an email)
			if tc.mockEmail != nil {
				select {
				case <-emailSent:
					// Email sent
				case <-time.After(time.Second):
					t.Error("email.SendMail was not called")
				}
			}
		})
	}
}
