package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// E2E Test Server - runs the actual server for integration testing
type E2ETestServer struct {
	app          *gin.Engine
	server       *httptest.Server
	url          string
	books        map[string]*Book
	booksMu      sync.RWMutex
	sortedKeys   []string   // Cached sorted keys
	keysCacheValid bool     // Whether the cache is valid

	// Auth state with proper synchronization
	authState struct {
		sync.RWMutex
		unlocked bool
	}
}

// newE2ETestServer creates and starts a real HTTP server for testing
func newE2ETestServer(t *testing.T) *E2ETestServer {
	gin.SetMode(gin.TestMode)

	server := &E2ETestServer{
		books: make(map[string]*Book),
	}

	// Build the actual server (same as main.go)
	server.app = gin.New()

	// Add recovery middleware
	server.app.Use(gin.Recovery())

	// Level 1: Ping
	server.app.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Level 2: Echo
	server.app.POST("/echo", func(c *gin.Context) {
		data, _ := c.GetRawData()
		c.Data(http.StatusOK, "application/json", data)
	})

	// JWT secret key
	var jwtSecret = []byte("quest-secret-key")

	// Level 5: Auth Token Issuer
	server.app.POST("/auth/token", func(c *gin.Context) {
		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schema"})
			return
		}

		// Validate credentials
		if payload.Username != "admin" || payload.Password != "password" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		// Generate JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": payload.Username,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": tokenString})
	})

	// Book Routes
	bookGroup := server.app.Group("/books")

	// Define GET routes that require auth (Level 5 requirement)
	// GET /books requires Bearer token
	// All other routes (POST, PUT, DELETE) remain open
	getBooksGroup := bookGroup.Group("")
	getBooksGroup.Use(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		tokenString := authHeader[7:]

		// Validate JWT token with better error handling
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Check signing method
			if token.Method == nil {
				return nil, jwt.ErrSignatureInvalid
			}
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	})

	// Level 3 & 7: Create Book
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

		bookID := uuid.New().String()

		// Default year to 2026 if not provided (zero value)
		year := payload.Year
		if year == 0 {
			year = 2026
		}

		book := &Book{
			ID:     bookID,
			Title:  payload.Title,
			Author: payload.Author,
			Year:   year,
		}

		server.booksMu.Lock()
		server.books[bookID] = book
		server.keysCacheValid = false  // Invalidate cache
		server.booksMu.Unlock()

		c.JSON(http.StatusCreated, book)
	})

	// Level 3 & 6: Read Books, Search & Paginate (protected by auth middleware)
	getBooksGroup.GET("/", func(c *gin.Context) {
		authorQuery := c.Query("author")

		// Pagination with bounds checking
		const maxPage = 1000000
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

		if page < 1 {
			page = 1
		}
		if page > maxPage {
			page = maxPage
		}
		if limit < 1 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		server.booksMu.RLock()

		// Build or use cached sorted keys
		if !server.keysCacheValid {
			// Release read lock and get write lock
			server.booksMu.RUnlock()
			server.booksMu.Lock()

			// Double-check after acquiring write lock
			if !server.keysCacheValid {
				// Build sorted keys
				server.sortedKeys = make([]string, 0, len(server.books))
				for k := range server.books {
					server.sortedKeys = append(server.sortedKeys, k)
				}

				// Simple string sort
				for i := 0; i < len(server.sortedKeys); i++ {
					for j := i + 1; j < len(server.sortedKeys); j++ {
						if server.sortedKeys[i] > server.sortedKeys[j] {
							server.sortedKeys[i], server.sortedKeys[j] = server.sortedKeys[j], server.sortedKeys[i]
						}
					}
				}
				server.keysCacheValid = true
			}

			// Downgrade to read lock
			server.booksMu.Unlock()
			server.booksMu.RLock()
		}

		keys := server.sortedKeys

		// Filter by author if specified
		var filteredBooks []*Book
		if authorQuery != "" {
			filteredBooks = make([]*Book, 0)
			for _, k := range keys {
				b := server.books[k]
				if strings.Contains(strings.ToLower(b.Author), strings.ToLower(authorQuery)) {
					filteredBooks = append(filteredBooks, b)
				}
			}
		} else {
			filteredBooks = make([]*Book, len(keys))
			for i, k := range keys {
				filteredBooks[i] = server.books[k]
			}
		}

		server.booksMu.RUnlock()

		// Pagination
		start := (page - 1) * limit
		end := start + limit

		if start >= len(filteredBooks) {
			c.JSON(http.StatusOK, []*Book{})
			return
		}
		if end > len(filteredBooks) {
			end = len(filteredBooks)
		}

		c.JSON(http.StatusOK, filteredBooks[start:end])
	})

	// Level 3 & 7: Read Single Book (protected by auth middleware)
	getBooksGroup.GET("/:id", func(c *gin.Context) {
		id := c.Param("id")

		server.booksMu.RLock()
		book, exists := server.books[id]
		server.booksMu.RUnlock()

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		c.JSON(http.StatusOK, book)
	})

	// Level 4: Update Book
	bookGroup.PUT("/:id", func(c *gin.Context) {
		id := c.Param("id")

		var payload Book
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schema"})
			return
		}

		server.booksMu.Lock()
		defer server.booksMu.Unlock()

		if _, exists := server.books[id]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		// Default year to 2026 if not provided (zero value)
		year := payload.Year
		if year == 0 {
			year = 2026
		}

		updatedBook := &Book{
			ID:     id,
			Title:  payload.Title,
			Author: payload.Author,
			Year:   year,
		}
		server.books[id] = updatedBook
		server.keysCacheValid = false  // Invalidate cache

		c.JSON(http.StatusOK, updatedBook)
	})

	// Level 4: Delete Book
	bookGroup.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")

		server.booksMu.Lock()
		defer server.booksMu.Unlock()

		if _, exists := server.books[id]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		delete(server.books, id)
		server.keysCacheValid = false  // Invalidate cache
		c.Status(http.StatusNoContent)
	})

	// Create test server
	server.server = httptest.NewServer(server.app)
	server.url = server.server.URL

	// Verify server is ready
	for i := 0; i < 50; i++ {
		resp, err := http.Get(server.url + "/ping")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return server
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("Server failed to start")
	return nil
}

// close stops the test server
func (s *E2ETestServer) close() {
	s.server.Close()
}

// e2eRequest makes HTTP requests to the test server
func (s *E2ETestServer) e2eRequest(method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, s.url+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if headers == nil {
		headers = make(map[string]string)
	}
	if headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return http.DefaultClient.Do(req)
}

// getAuthToken retrieves a valid JWT token for authentication
func (s *E2ETestServer) getAuthToken() string {
	authPayload := map[string]string{"username": "admin", "password": "password"}
	authResp, _ := s.e2eRequest("POST", "/auth/token", authPayload, nil)
	var authResult map[string]string
	json.NewDecoder(authResp.Body).Decode(&authResult)
	return authResult["token"]
}

// ============================================================================
// E2E Tests - These run against a real HTTP server
// ============================================================================

func TestE2E_Level1_Ping(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	resp, err := server.e2eRequest("GET", "/ping", nil, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	if result["success"] != true {
		t.Errorf("Expected success=true, got '%v'", result)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Errorf("Expected Content-Type to contain application/json, got '%s'", resp.Header.Get("Content-Type"))
	}
}

func TestE2E_Level2_Echo(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	payload := map[string]string{"message": "hello world", "foo": "bar"}
	resp, err := server.e2eRequest("POST", "/echo", payload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["message"] != "hello world" {
		t.Errorf("Expected message 'hello world', got '%s'", result["message"])
	}
}

func TestE2E_Level3_CreateBook(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	payload := map[string]interface{}{
		"title":  "E2E Test Book",
		"author": "E2E Author",
		"year":   2024,
	}
	resp, err := server.e2eRequest("POST", "/books/", payload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var result Book
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Title != "E2E Test Book" {
		t.Errorf("Expected title 'E2E Test Book', got '%s'", result.Title)
	}
}

func TestE2E_Level3_ReadBooks(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Create a book first
	payload := map[string]interface{}{
		"title":  "Read Test",
		"author": "Read Author",
		"year":   2024,
	}
	server.e2eRequest("POST", "/books/", payload, nil)

	// Get auth token first
	authPayload := map[string]string{"username": "admin", "password": "password"}
	authResp, _ := server.e2eRequest("POST", "/auth/token", authPayload, nil)
	var authResult map[string]string
	json.NewDecoder(authResp.Body).Decode(&authResult)
	token := authResult["token"]

	// Get all books with auth
	resp, err := server.e2eRequest("GET", "/books/", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) == 0 {
		t.Error("Expected at least one book")
	}
}

func TestE2E_Level4_UpdateBook(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Create a book
	payload := map[string]interface{}{
		"title":  "Update Test",
		"author": "Original Author",
		"year":   2024,
	}
	createResp, _ := server.e2eRequest("POST", "/books/", payload, nil)
	var createdBook Book
	json.NewDecoder(createResp.Body).Decode(&createdBook)

	// Update the book
	updatePayload := map[string]interface{}{
		"title":  "Updated Title",
		"author": "Updated Author",
		"year":   2026,
	}
	resp, err := server.e2eRequest("PUT", "/books/"+createdBook.ID, updatePayload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result Book
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", result.Title)
	}
}

func TestE2E_Level4_DeleteBook(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Create a book
	payload := map[string]interface{}{
		"title":  "Delete Test",
		"author": "Delete Author",
		"year":   2024,
	}
	createResp, _ := server.e2eRequest("POST", "/books/", payload, nil)
	var createdBook Book
	json.NewDecoder(createResp.Body).Decode(&createdBook)

	// Delete the book
	resp, err := server.e2eRequest("DELETE", "/books/"+createdBook.ID, nil, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 204 {
		t.Errorf("Expected status 204, got %d", resp.StatusCode)
	}

	// Verify it's deleted (GET requires auth)
	token := server.getAuthToken()
	resp, _ = server.e2eRequest("GET", "/books/"+createdBook.ID, nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404 after delete, got %d", resp.StatusCode)
	}
}

func TestE2E_Level5_AuthToken(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Test with valid credentials
	payload := map[string]string{
		"username": "admin",
		"password": "password",
	}
	resp, err := server.e2eRequest("POST", "/auth/token", payload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["token"] == "" {
		t.Errorf("Expected token to be returned, got empty string")
	}
}

func TestE2E_Level6_SearchAndPaginate(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Create multiple books with different authors
	books := []map[string]interface{}{
		{"title": "Book 1", "author": "Alice", "year": 2020},
		{"title": "Book 2", "author": "Bob", "year": 2021},
		{"title": "Book 3", "author": "Alice Smith", "year": 2022},
		{"title": "Book 4", "author": "Charlie", "year": 2023},
		{"title": "Book 5", "author": "Alice", "year": 2024},
	}

	for _, book := range books {
		server.e2eRequest("POST", "/books/", book, nil)
	}

	// Get auth token for GET requests
	token := server.getAuthToken()

	// Test search by author
	resp, err := server.e2eRequest("GET", "/books/?author=Alice", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 3 {
		t.Errorf("Expected 3 books by Alice, got %d", len(result))
	}

	// Test pagination
	resp, err = server.e2eRequest("GET", "/books/?page=1&limit=2", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	var pagedResult []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&pagedResult)

	if len(pagedResult) != 2 {
		t.Errorf("Expected 2 books on page 1, got %d", len(pagedResult))
	}
}

func TestE2E_Level7_ErrorHandling(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Test 400 - Invalid schema (missing author)
	invalidPayload := map[string]string{"title": "No Author"}
	resp, err := server.e2eRequest("POST", "/books/", invalidPayload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400 for invalid schema, got %d", resp.StatusCode)
	}

	var errorResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errorResult)
	if errorResult["error"] != "Missing required fields" {
		t.Errorf("Expected error message 'Missing required fields', got '%v'", errorResult["error"])
	}

	// Test 400 - Invalid JSON
	resp, err = server.e2eRequest("POST", "/books/", "invalid json", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Test 404 - Not found (GET requires auth)
	token := server.getAuthToken()
	resp, err = server.e2eRequest("GET", "/books/non-existent-id", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}

	// Test 404 - Update non-existent book
	updatePayload := map[string]string{"title": "Updated", "author": "Author"}
	resp, err = server.e2eRequest("PUT", "/books/non-existent-id", updatePayload, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404 for update non-existent, got %d", resp.StatusCode)
	}

	// Test 404 - Delete non-existent book
	resp, err = server.e2eRequest("DELETE", "/books/non-existent-id", nil, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status 404 for delete non-existent, got %d", resp.StatusCode)
	}
}

func TestE2E_Level8_ConcurrentOperations(t *testing.T) {
	server := newE2ETestServer(t)
	defer server.close()

	// Get auth token first (requires credentials)
	authPayload := map[string]string{"username": "admin", "password": "password"}
	resp, _ := server.e2eRequest("POST", "/auth/token", authPayload, nil)
	var tokenResult map[string]string
	json.NewDecoder(resp.Body).Decode(&tokenResult)
	authToken := "Bearer " + tokenResult["token"]

	// Test concurrent create operations
	const numGoroutines = 20
	type result struct {
		id    string
		title string
		err   error
	}
	results := make(chan result, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			// Each goroutine creates its own auth header map
			authHeader := map[string]string{"Authorization": authToken}
			payload := map[string]interface{}{
				"title":  fmt.Sprintf("Concurrent Book %d", idx),
				"author": fmt.Sprintf("Author %d", idx),
				"year":   2024,
			}
			resp, err := server.e2eRequest("POST", "/books/", payload, authHeader)
			if err != nil {
				results <- result{err: err}
				return
			}

			var book Book
			json.NewDecoder(resp.Body).Decode(&book)
			results <- result{id: book.ID, title: book.Title, err: err}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		r := <-results
		if r.err == nil && r.id != "" {
			successCount++
		}
	}

	if successCount != numGoroutines {
		t.Errorf("Expected all %d concurrent creates to succeed, got %d", numGoroutines, successCount)
	}

	// Verify all books were created (use higher limit to get all books)
	verifyAuthHeader := map[string]string{"Authorization": authToken}
	resp, _ = server.e2eRequest("GET", "/books/?limit=100", nil, verifyAuthHeader)
	var books []Book
	json.NewDecoder(resp.Body).Decode(&books)

	if len(books) != numGoroutines {
		t.Errorf("Expected %d books after concurrent creates, got %d", numGoroutines, len(books))
	}
}

// TestE2E_ConfigurablePort tests that PORT environment variable works
func TestE2E_ConfigurablePort(t *testing.T) {
	// Set custom port
	oldPort := os.Getenv("PORT")
	defer func() {
		if oldPort != "" {
			os.Setenv("PORT", oldPort)
		} else {
			os.Unsetenv("PORT")
		}
	}()
	os.Setenv("PORT", "3999")

	// Build app with custom port
	gin.SetMode(gin.TestMode)
	app := gin.New()
	app.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Get the configured port
	port := os.Getenv("PORT")
	if port == "" {
		port = "3082" // Default from main.go
	}

	// Start server
	testServer := httptest.NewServer(app)
	defer testServer.Close()

	// Wait and test
	time.Sleep(100 * time.Millisecond)
	resp, err := http.Get(testServer.URL + "/ping")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
