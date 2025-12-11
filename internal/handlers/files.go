package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ricci/calibre-opds-go/internal/database"
	"github.com/ricci/calibre-opds-go/internal/opds"
)

// GetCover 获取书籍封面
func (h *Handler) GetCover(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := h.db.GetBookDetail(bookID)
	if err != nil || book == nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	basePath := h.config.BooksPath
	bookPath := strings.ReplaceAll(book.Path, "\\", "/")

	// 尝试不同的封面扩展名
	coverExtensions := []string{".jpg", ".png"}
	for _, ext := range coverExtensions {
		coverPath := filepath.Join(basePath, bookPath, "cover"+ext)
		if _, err := os.Stat(coverPath); err == nil {
			mimeType := "image/jpeg"
			if ext == ".png" {
				mimeType = "image/png"
			}
			c.File(coverPath)
			c.Header("Content-Type", mimeType)
			return
		}
	}

	c.String(http.StatusNotFound, "Cover not found")
}

// DownloadBook 下载书籍
func (h *Handler) DownloadBook(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	requestedFormat := strings.ToUpper(c.Param("format"))

	book, err := h.db.GetBookDetail(bookID)
	if err != nil || book == nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	// 查找匹配的格式
	var targetFormat *database.Format
	for i := range book.Formats {
		if strings.ToUpper(book.Formats[i].Format) == requestedFormat {
			targetFormat = &book.Formats[i]
			break
		}
	}

	if targetFormat == nil {
		c.String(http.StatusNotFound, fmt.Sprintf("Format %s not found", requestedFormat))
		return
	}

	// 构建文件路径
	basePath := h.config.BooksPath
	bookPath := strings.ReplaceAll(book.Path, "\\", "/")

	// 尝试多个可能的文件路径
	possiblePaths := []string{
		filepath.Join(basePath, bookPath, targetFormat.Filename),
	}

	// 添加扩展名的变体
	ext := getFileExtension(targetFormat.Format)
	if ext != "" && !strings.HasSuffix(strings.ToLower(targetFormat.Filename), ext) {
		possiblePaths = append(possiblePaths, filepath.Join(basePath, bookPath, targetFormat.Filename+ext))
	}

	// 查找存在的文件
	var fullPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			fullPath = path
			break
		}
	}

	if fullPath == "" {
		c.String(http.StatusNotFound, "File not found")
		return
	}

	// 生成安全的文件名
	safeFilename := generateSafeFilename(book.Title, targetFormat.Format)

	// 设置响应头
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(safeFilename)))
	c.Header("Content-Type", opds.GetMimeType(targetFormat.Format))
	c.Header("Cache-Control", "public, max-age=3600")

	// 发送文件
	file, err := os.Open(fullPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to open file")
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err == nil {
		c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	}

	io.Copy(c.Writer, file)
}

// 辅助函数
func getFileExtension(format string) string {
	extensions := map[string]string{
		"EPUB": ".epub",
		"PDF":  ".pdf",
		"MOBI": ".mobi",
		"AZW3": ".azw3",
		"FB2":  ".fb2",
		"RTF":  ".rtf",
		"TXT":  ".txt",
		"HTML": ".html",
		"LIT":  ".lit",
	}
	return extensions[strings.ToUpper(format)]
}

func generateSafeFilename(title, format string) string {
	// 移除非法字符
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	safe := reg.ReplaceAllString(title, "")
	safe = strings.ReplaceAll(safe, " ", "_")

	// 添加扩展名
	ext := getFileExtension(format)
	if ext != "" && !strings.HasSuffix(strings.ToLower(safe), ext) {
		safe += ext
	}

	return safe
}
