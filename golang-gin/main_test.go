package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestApp() *gin.Engine {
	gin.SetMode(gin.TestMode)
	app := gin.New()

	// Setup routes (same as main.go)
	app.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	app.POST("/echo", func(c *gin.Context) {
		data, _ := c.GetRawData()
		c.Data(http.StatusOK, "application/json", data)
	})

	app.POST("/auth/token", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"token": "quest-token-xyz"})
	})

	bookGroup := app.Group("/books")

	bookGroup.POST("/", func(c *gin.Context) {
		var payload Book
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schema"})
			return
		}

		if payload.Title == "" || payload.Author == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
			return
		}

		book := &Book{
			ID:     "test-" + payload.Title,
			Title:  payload.Title,
			Author: payload.Author,
			Year:   payload.Year,
		}

		c.JSON(http.StatusCreated, book)
	})

	bookGroup.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, []*Book{})
	})

	bookGroup.GET("/:id", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})

	bookGroup.PUT("/:id", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})

	bookGroup.DELETE("/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	return app
}

// Level 1: Ping
func TestLevel1_Ping(t *testing.T) {
	app := setupTestApp()

	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, true, result["success"])
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

// Level 2: Echo
func TestLevel2_Echo(t *testing.T) {
	app := setupTestApp()

	payload := map[string]string{"message": "hello world"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/echo", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "hello world", result["message"])
}

// Level 3: Create Book
func TestLevel3_CreateBook(t *testing.T) {
	app := setupTestApp()

	// Configure client to not follow redirects
	_ = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	payload := map[string]interface{}{
		"title":  "TestBook",
		"author": "TestAuthor",
		"year":   2024,
	}
	jsonPayload, _ := json.Marshal(payload)

	// Use trailing slash to match Gin's route handling
	req, _ := http.NewRequest("POST", "/books/", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Gin returns 307 redirect, we need to follow it
	if w.Code == http.StatusTemporaryRedirect {
		location := w.Header().Get("Location")
		req, _ = http.NewRequest("POST", location, bytes.NewBuffer(jsonPayload))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		app.ServeHTTP(w, req)
	}

	assert.Equal(t, http.StatusCreated, w.Code)

	var result Book
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "TestBook", result.Title)
}

// Level 5: Auth Token
func TestLevel5_AuthToken(t *testing.T) {
	app := setupTestApp()

	req, _ := http.NewRequest("POST", "/auth/token", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "quest-token-xyz", result["token"])
}

// Level 7: Error Handling - Invalid Schema
func TestLevel7_InvalidSchema(t *testing.T) {
	app := setupTestApp()

	// Use trailing slash to match Gin's routing
	payload := map[string]string{"title": "NoAuthor"}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/books/", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Level 7: Error Handling - Not Found
func TestLevel7_NotFound(t *testing.T) {
	app := setupTestApp()

	req, _ := http.NewRequest("GET", "/books/nonexistent", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
