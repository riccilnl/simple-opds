package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ricci/calibre-opds-go/internal/config"
	"github.com/ricci/calibre-opds-go/internal/database"
	"github.com/ricci/calibre-opds-go/internal/handlers"
	"github.com/ricci/calibre-opds-go/pkg/logger"
)

func main() {
	// 初始化日志
	logger.Init()
	log.Println("Starting Calibre OPDS Server (Go Edition)...")

	// 加载配置
	cfg := config.Load()
	log.Printf("Database path: %s", cfg.DBPath)
	log.Printf("Books path: %s", cfg.BooksPath)

	// 初始化数据库
	db, err := database.NewDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 验证数据库
	if err := db.Validate(); err != nil {
		log.Fatalf("Database validation failed: %v", err)
	}

	bookCount, err := db.GetBooksCount("")
	if err != nil {
		log.Fatalf("Failed to get book count: %v", err)
	}
	log.Printf("Database loaded successfully. Total books: %d", bookCount)

	// 设置Gin模式
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := gin.Default()

	// 初始化处理器
	h := handlers.NewHandler(db, cfg)

	// OPDS路由
	opdsGroup := router.Group("/opds")
	{
		opdsGroup.GET("", h.OPDSRoot)
		opdsGroup.GET("/books", h.OPDSBooks)
		opdsGroup.GET("/book/:id", h.OPDSBookDetail)
		opdsGroup.GET("/authors", h.OPDSAuthors)
		opdsGroup.GET("/series", h.OPDSSeries)
		opdsGroup.GET("/tags", h.OPDSTags)
		opdsGroup.GET("/cover/:id", h.GetCover)
	}

	// 文件下载路由
	router.GET("/download/:id/:format", h.DownloadBook)

	// REST API路由
	apiGroup := router.Group("/api")
	{
		apiGroup.GET("/books", h.APIBooks)
		apiGroup.GET("/book/:id", h.APIBookDetail)
		apiGroup.GET("/stats", h.APIStats)
		apiGroup.GET("/health", h.APIHealth)
		apiGroup.GET("/connection-stats", h.APIConnectionStats)
		apiGroup.GET("/diagnose", h.APIDiagnose)
	}

	// 启动服务器
	host := getEnv("OPDS_HOST", "0.0.0.0")
	port := getEnv("OPDS_PORT", "1580")
	addr := fmt.Sprintf("%s:%s", host, port)

	log.Printf("OPDS Catalog: http://%s/opds", addr)
	log.Printf("Server starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
