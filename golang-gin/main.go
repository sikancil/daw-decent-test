package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Book Schema
type Book struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
}

// In-Memory Storage & Concurrency
// Gin uses net/http which has different memory management than FastHTTP
var (
	books   = make(map[string]*Book)
	booksMu sync.RWMutex
	sortedKeys []string  // Cached sorted keys for fast iteration
	keysCacheValid bool   // Whether the cache is valid

	// Auth state with proper synchronization
	authState struct {
		sync.RWMutex
		unlocked bool
	}
)

func main() {
	// Initialize Gin
	gin.SetMode(gin.ReleaseMode)
	app := gin.New()

	// Add recovery middleware to catch panics
	app.Use(gin.Recovery())

	// Enable auto-redirect for trailing slashes (important for /books vs /books/)
	// app.RedirectTrailingSlash = false  // REMOVED - this was causing 404s

	// Level 1: Ping
	app.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	// Level 2: Echo
	app.POST("/echo", func(c *gin.Context) {
		data, _ := c.GetRawData()
		c.Data(http.StatusOK, "application/json", data)
	})

	// JWT secret key (in production, use environment variable)
	var jwtSecret = []byte("quest-secret-key")

	// Level 5: Auth Token Issuer
	app.POST("/auth/token", func(c *gin.Context) {
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
	bookGroup := app.Group("/books")

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

		// Level 7: Error handling for invalid payload
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

		booksMu.Lock()
		books[bookID] = book
		keysCacheValid = false  // Invalidate cache
		booksMu.Unlock()

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

		booksMu.RLock()

		// Build or use cached sorted keys
		if !keysCacheValid {
			// Release read lock and get write lock
			booksMu.RUnlock()
			booksMu.Lock()

			// Double-check after acquiring write lock
			if !keysCacheValid {
				// Build sorted keys
				sortedKeys = make([]string, 0, len(books))
				for k := range books {
					sortedKeys = append(sortedKeys, k)
				}

				// Simple string sort
				for i := 0; i < len(sortedKeys); i++ {
					for j := i + 1; j < len(sortedKeys); j++ {
						if sortedKeys[i] > sortedKeys[j] {
							sortedKeys[i], sortedKeys[j] = sortedKeys[j], sortedKeys[i]
						}
					}
				}
				keysCacheValid = true
			}

			// Downgrade to read lock
			booksMu.Unlock()
			booksMu.RLock()
		}

		keys := sortedKeys

		// Filter by author if specified
		var filteredBooks []*Book
		if authorQuery != "" {
			filteredBooks = make([]*Book, 0)
			for _, k := range keys {
				b := books[k]
				if strings.Contains(strings.ToLower(b.Author), strings.ToLower(authorQuery)) {
					filteredBooks = append(filteredBooks, b)
				}
			}
		} else {
			filteredBooks = make([]*Book, len(keys))
			for i, k := range keys {
				filteredBooks[i] = books[k]
			}
		}

		booksMu.RUnlock()

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

		booksMu.RLock()
		book, exists := books[id]
		booksMu.RUnlock()

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

		booksMu.Lock()
		defer booksMu.Unlock()

		if _, exists := books[id]; !exists {
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
		books[id] = updatedBook
		keysCacheValid = false  // Invalidate cache

		c.JSON(http.StatusOK, updatedBook)
	})

	// Level 4: Delete Book
	bookGroup.DELETE("/:id", func(c *gin.Context) {
		id := c.Param("id")

		booksMu.Lock()
		defer booksMu.Unlock()

		if _, exists := books[id]; !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		delete(books, id)
		keysCacheValid = false  // Invalidate cache
		c.Status(http.StatusNoContent)
	})

	// Configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "3082" // Different port to avoid conflict with Fiber
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	addr := net.JoinHostPort(host, port)
	app.Run(addr)
}
