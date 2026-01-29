package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type LotteryEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Ticket    string `json:"ticket"`
	CreatedAt string `json:"created_at"`
}

type AddEntryRequest struct {
	Name   string `json:"name"`
	Ticket string `json:"ticket"`
}

var (
	entries   []LotteryEntry
	entriesMu sync.RWMutex
	nextID    int
)

func main() {
	entries = make([]LotteryEntry, 0)
	r := gin.Default()

	r.GET("/entries", listEntries)
	r.POST("/entries", addEntry)
	r.GET("/openapi.yaml", serveOpenAPI)
	r.GET("/swagger", serveSwaggerUI)
	r.GET("/", serveLanding)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	r.Run(":" + port)
}

func listEntries(c *gin.Context) {
	entriesMu.RLock()
	defer entriesMu.RUnlock()
	c.JSON(http.StatusOK, entries)
}

func addEntry(c *gin.Context) {
	var req AddEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: name and ticket required"})
		return
	}
	if req.Name == "" || req.Ticket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and ticket are required"})
		return
	}
	entriesMu.Lock()
	nextID++
	id := fmt.Sprintf("%d", nextID)
	entry := LotteryEntry{
		ID:        id,
		Name:      req.Name,
		Ticket:    req.Ticket,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	entries = append(entries, entry)
	entriesMu.Unlock()
	c.JSON(http.StatusCreated, entry)
}

func serveOpenAPI(c *gin.Context) {
	c.Header("Content-Type", "application/x-yaml")
	c.File("openapi.yaml")
}

func serveLanding(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, landingHTML)
}

func serveSwaggerUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, swaggerHTML)
}

const landingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Lottery API</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 40rem; margin: 4rem auto; padding: 0 1rem; }
    a { display: inline-block; margin-top: 1rem; padding: 0.75rem 1.5rem; background: #499; color: #fff; text-decoration: none; border-radius: 6px; }
    a:hover { background: #3a8; }
  </style>
</head>
<body>
  <h1>Lottery API</h1>
  <p>Add and list lottery entries. Use Swagger UI to try the APIs.</p>
  <a href="/swagger">Open Swagger UI</a>
</body>
</html>
`

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Lottery API - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ]
      });
    };
  </script>
</body>
</html>
`
