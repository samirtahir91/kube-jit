package handlers

import (
	"github.com/gin-gonic/gin"
)

// ServeOpenAPI3 serves the OpenAPI 3 YAML spec
func ServeOpenAPI3(c *gin.Context) {
	c.File("/docs/openapi3.yaml")
}
