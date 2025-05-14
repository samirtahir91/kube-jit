package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestServeOpenAPI3(t *testing.T) {
	gin.SetMode(gin.TestMode) // Set Gin to test mode

	router := gin.New()
	// Define a route that uses the handler
	router.GET("/openapi-spec", ServeOpenAPI3)

	t.Run("file_not_found_at_hardcoded_absolute_path", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "/openapi-spec", nil)
		assert.NoError(t, err)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code, "Expected HTTP 404 Not Found because /docs/openapi3.yaml is unlikely to exist")
	})
}
