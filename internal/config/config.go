package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config 应用配置
type Config struct {
	// 数据库配置
	DBPath            string
	BooksPath         string
	ConnectionTimeout time.Duration

	// 服务器配置
	Host        string
	Port        string
	Environment string

	// 日志配置
	LogLevel     string
	LogFile      string
	LogToConsole bool
}

// Load 加载配置
func Load() *Config {
	cfg := &Config{
		DBPath:            findDatabasePath(),
		BooksPath:         getEnv("CALIBRE_BOOKS_PATH", "books"),
		ConnectionTimeout: getDurationEnv("DB_CONNECTION_TIMEOUT", 30*time.Second),
		Host:              getEnv("OPDS_HOST", "0.0.0.0"),
		Port:              getEnv("OPDS_PORT", "1580"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "INFO"),
		LogFile:           getEnv("LOG_FILE", "calibre_opds.log"),
		LogToConsole:      getBoolEnv("LOG_TO_CONSOLE", true),
	}

	return cfg
}

// findDatabasePath 智能查找数据库文件
func findDatabasePath() string {
	candidates := []string{
		"books/metadata.db",
		"/books/metadata.db",
		"metadata.db",
		getEnv("CALIBRE_DB_PATH", ""),
	}

	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 默认返回第一个候选路径
	return candidates[0]
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv 获取布尔类型环境变量
func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

// getDurationEnv 获取时间间隔类型环境变量
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		// 尝试解析为秒数
		if seconds, err := strconv.ParseFloat(value, 64); err == nil {
			return time.Duration(seconds * float64(time.Second))
		}
	}
	return defaultValue
}

// GetBooksFullPath 获取书籍完整路径
func (c *Config) GetBooksFullPath() string {
	if filepath.IsAbs(c.BooksPath) {
		return c.BooksPath
	}
	// 如果是相对路径，尝试相对于数据库路径
	dbDir := filepath.Dir(c.DBPath)
	return filepath.Join(dbDir, c.BooksPath)
}
