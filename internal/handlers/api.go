package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// APIBooks REST API书籍列表
func (h *Handler) APIBooks(c *gin.Context) {
	search := c.Query("search")
	limit := getIntParam(c, "limit", 20, 100)
	offset := getIntParam(c, "offset", 0, 0)

	books, err := h.db.GetBooks(limit, offset, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get books"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"books":  books,
		"total":  len(books),
		"limit":  limit,
		"offset": offset,
	})
}

// APIBookDetail REST API书籍详情
func (h *Handler) APIBookDetail(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	book, err := h.db.GetBookDetail(bookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get book"})
		return
	}
	if book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	c.JSON(http.StatusOK, book)
}

// APIStats REST API统计信息
func (h *Handler) APIStats(c *gin.Context) {
	stats, err := h.db.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// APIHealth 健康检查
func (h *Handler) APIHealth(c *gin.Context) {
	// 测试数据库连接
	count, err := h.db.GetBooksCount("")
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"database":  "unhealthy",
			"error":     err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"database":   "healthy",
		"book_count": count,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

// APIConnectionStats 连接统计信息
func (h *Handler) APIConnectionStats(c *gin.Context) {
	stats := gin.H{
		"connection_strategy": "per_request",
		"database_path":       h.config.DBPath,
		"books_path":          h.config.BooksPath,
		"timestamp":           time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, stats)
}

// APIDiagnose 诊断信息
func (h *Handler) APIDiagnose(c *gin.Context) {
	// 获取统计信息
	stats, _ := h.db.GetStats()

	// 获取样本书籍
	sampleBooks, _ := h.db.GetBooks(3, 0, "")

	diagnosis := gin.H{
		"application": gin.H{
			"db_path":    h.config.DBPath,
			"books_path": h.config.BooksPath,
			"log_level":  h.config.LogLevel,
		},
		"tests": gin.H{
			"database": gin.H{
				"status":      "ok",
				"total_books": stats.TotalBooks,
			},
			"sample_books": sampleBooks,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, diagnosis)
}
