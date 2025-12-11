package database

import (
	"time"
)

// Book 书籍模型
type Book struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	AuthorSort   string    `json:"author_sort"`
	Path         string    `json:"path"`
	SeriesIndex  *float64  `json:"series_index,omitempty"`
	ISBN         *string   `json:"isbn,omitempty"`
	PubDate      *string   `json:"pubdate,omitempty"`
	LastModified time.Time `json:"last_modified"`
	HasCover     bool      `json:"has_cover"`
	UUID         string    `json:"uuid"`
	Comments     string    `json:"comments,omitempty"`
	
	// 关联数据
	Authors []Author `json:"authors,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Series  *Series  `json:"series,omitempty"`
	Formats []Format `json:"formats,omitempty"`
}

// Author 作者模型
type Author struct {
	Name string `json:"name"`
	Sort string `json:"sort"`
}

// Series 系列模型
type Series struct {
	Name  string   `json:"name"`
	Sort  string   `json:"sort"`
	Index *float64 `json:"index,omitempty"`
}

// Format 格式模型
type Format struct {
	Format   string `json:"format"`
	Size     int64  `json:"size"`
	Filename string `json:"filename"`
}

// Tag 标签模型
type Tag struct {
	Name      string `json:"name"`
	BookCount int    `json:"book_count,omitempty"`
}

// AuthorInfo 作者信息（用于列表）
type AuthorInfo struct {
	Name      string `json:"name"`
	Sort      string `json:"sort"`
	BookCount int    `json:"book_count"`
}

// SeriesInfo 系列信息（用于列表）
type SeriesInfo struct {
	Name      string `json:"name"`
	Sort      string `json:"sort"`
	BookCount int    `json:"book_count"`
}

// Stats 统计信息
type Stats struct {
	TotalBooks   int            `json:"total_books"`
	TotalAuthors int            `json:"total_authors"`
	Formats      map[string]int `json:"formats"`
}
