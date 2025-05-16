package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

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

var originalDB *gorm.DB

// setupRouterAndDBMock initializes a Gin router, mocks the database, and sets up sessions.
func setupRouterAndDBMock(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	// Initialize a Nop logger for the handlers package (used by RequestLogger)
	// This assumes RequestLogger(c) uses the package-level logger initialized by InitLogger.
	InitLogger(zap.NewNop())
	// Initialize a Nop logger for the db package to suppress DB logs during tests
	// This assumes db package has an InitLogger function similar to handlers.
	db.InitLogger(zap.NewNop())

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup session middleware
	store := cookie.NewStore([]byte("test-secret-key"))
	router.Use(sessions.Sessions("kube_jit_test_session", store))

	// Mock GORM DB
	var sqlDB *sql.DB
	var mock sqlmock.Sqlmock
	var err error

	sqlDB, mock, err = sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB, // Pass the sqlmock connection to GORM
	}), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent), // Suppress GORM logging
	})
	if err != nil {
		sqlDB.Close() // Ensure sqlDB is closed if gorm.Open fails
		t.Fatalf("Failed to open GORM with mock DB: %v", err)
	}

	// Store the original db.DB instance only if it hasn't been stored yet (e.g. by a concurrent test)
	if originalDB == nil && db.DB != nil {
		originalDB = db.DB
	}

	db.DB = gormDB // Replace with mock DB instance

	return router, mock
}

// teardownDBMock restores the original database connection and closes the mock's underlying sql.DB.
func teardownDBMock(t *testing.T, mock sqlmock.Sqlmock) {
	if db.DB != nil && db.DB != originalDB { // Ensure we are closing the mocked DB, not the original one if already restored
		gormMockedDB := db.DB
		sqlDBFromMock, err := gormMockedDB.DB()
		if err == nil && sqlDBFromMock != nil {
			sqlDBFromMock.Close() // Close the actual *sql.DB from sqlmock
		}
	}

	db.DB = originalDB // Restore the original DB instance (which might be nil)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanExpiredRequests_Unauthorized_NotAdmin(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("isAdmin", false)
		s.Set("userID", "test-user")
		err := s.Save()
		assert.NoError(t, err)

		// Simulate the auth middleware populating the context for GetSessionData
		// Replace map[string]interface{} and its contents with the actual structure
		// expected by GetSessionData / CleanExpiredRequests.
		sessionContextData := map[string]interface{}{
			"userID":  "test-user",
			"isAdmin": false,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp models.SimpleMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Unauthorized: admin only", resp.Error)
	assert.NoError(t, mock.ExpectationsWereMet()) // Verify all sqlmock expectations
}

func TestCleanExpiredRequests_Unauthorized_IsAdminMissing(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		// "isAdmin" key is not set
		s.Set("userID", "test-user")
		err := s.Save()
		assert.NoError(t, err)

		// Simulate how auth middleware translates a missing "isAdmin" in the cookie session
		// to the "sessionData" context value. Assuming it defaults to false.
		sessionContextData := map[string]interface{}{
			"userID":  "test-user",
			"isAdmin": false,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp models.SimpleMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Unauthorized: admin only", resp.Error)
	assert.NoError(t, mock.ExpectationsWereMet()) // Verify all sqlmock expectations
}

func TestCleanExpiredRequests_Unauthorized_IsAdminNotBool(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("isAdmin", "not-a-boolean") // Set to a non-boolean type
		s.Set("userID", "test-user")
		err := s.Save()
		assert.NoError(t, err)

		// Simulate how auth middleware translates a non-boolean "isAdmin"
		// to the "sessionData" context value. Assuming it defaults to false.
		sessionContextData := map[string]interface{}{
			"userID":  "test-user",
			"isAdmin": false,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp models.SimpleMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Unauthorized: admin only", resp.Error)
	assert.NoError(t, mock.ExpectationsWereMet()) // Verify all sqlmock expectations
}

func TestCleanExpiredRequests_Success_DeletesRecords(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	expectedDeletedCount := int64(5)

	mock.ExpectBegin()
	// GORM uses the table name from the model, typically plural snake_case.
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "request_data" WHERE end_date < $1 AND status = $2`)).
		WithArgs(sqlmock.AnyArg(), "Requested"). // time.Now() is $1, "Requested" is $2
		WillReturnResult(sqlmock.NewResult(0, expectedDeletedCount))
	mock.ExpectCommit()

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("isAdmin", true)
		s.Set("userID", "admin-user")
		err := s.Save()
		assert.NoError(t, err)

		sessionContextData := map[string]interface{}{
			"userID":  "admin-user",
			"isAdmin": true,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp CleanExpiredResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Expired non-approved requests cleaned", resp.Message)
	assert.Equal(t, expectedDeletedCount, resp.Deleted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanExpiredRequests_Success_NoRecordsToDelete(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	expectedDeletedCount := int64(0)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "request_data" WHERE end_date < $1 AND status = $2`)).
		WithArgs(sqlmock.AnyArg(), "Requested").
		WillReturnResult(sqlmock.NewResult(0, expectedDeletedCount))
	mock.ExpectCommit()

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("isAdmin", true)
		s.Set("userID", "admin-user")
		err := s.Save()
		assert.NoError(t, err)

		sessionContextData := map[string]interface{}{
			"userID":  "admin-user",
			"isAdmin": true,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp CleanExpiredResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Expired non-approved requests cleaned", resp.Message)
	assert.Equal(t, expectedDeletedCount, resp.Deleted)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanExpiredRequests_DBError(t *testing.T) {
	router, mock := setupRouterAndDBMock(t)
	defer teardownDBMock(t, mock)

	dbQueryError := errors.New("simulated database error")

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "request_data" WHERE end_date < $1 AND status = $2`)).
		WithArgs(sqlmock.AnyArg(), "Requested").
		WillReturnError(dbQueryError)
	mock.ExpectRollback()

	router.POST("/admin/clean-expired", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("isAdmin", true)
		s.Set("userID", "admin-user")
		err := s.Save()
		assert.NoError(t, err)

		sessionContextData := map[string]interface{}{
			"userID":  "admin-user",
			"isAdmin": true,
		}
		c.Set("sessionData", sessionContextData)

		CleanExpiredRequests(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/admin/clean-expired", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp models.SimpleMessageResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Failed to clean expired requests", resp.Error)
	assert.NoError(t, mock.ExpectationsWereMet())
}
