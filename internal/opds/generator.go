package opds

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/ricci/calibre-opds-go/internal/database"
)

// Feed OPDS feed结构
type Feed struct {
	XMLName xml.Name `xml:"feed"`
	Xmlns   string   `xml:"xmlns,attr"`
	XmlnsOPDS string `xml:"xmlns:opds,attr"`
	
	Title   string    `xml:"title"`
	ID      string    `xml:"id"`
	Updated string    `xml:"updated"`
	
	Links   []Link    `xml:"link"`
	Entries []Entry   `xml:"entry"`
	
	// 分页信息
	TotalResults  *int `xml:"opds:totalResults,omitempty"`
	StartIndex    *int `xml:"opds:startIndex,omitempty"`
	ItemsPerPage  *int `xml:"opds:itemsPerPage,omitempty"`
}

// Entry OPDS条目
type Entry struct {
	Title   string   `xml:"title"`
	ID      string   `xml:"id"`
	Updated string   `xml:"updated,omitempty"`
	Summary string   `xml:"summary,omitempty"`
	Authors []Author `xml:"author,omitempty"`
	Links   []Link   `xml:"link"`
}

// Author 作者
type Author struct {
	Name string `xml:"name"`
}

// Link 链接
type Link struct {
	Rel   string `xml:"rel,attr"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr,omitempty"`
	Length string `xml:"length,attr,omitempty"`
}

// Generator OPDS生成器
type Generator struct {
	BaseURL string
}

// NewGenerator 创建OPDS生成器
func NewGenerator(baseURL string) *Generator {
	return &Generator{
		BaseURL: baseURL,
	}
}

// CreateFeed 创建OPDS feed
func (g *Generator) CreateFeed(title string, entries []Entry, links []Link, feedInfo *FeedInfo) ([]byte, error) {
	feed := Feed{
		Xmlns:     "http://www.w3.org/2005/Atom",
		XmlnsOPDS: "http://opds-spec.org/2010/catalog",
		Title:     title,
		ID:        fmt.Sprintf("urn:uuid:%s", generateUUID()),
		Updated:   time.Now().UTC().Format(time.RFC3339),
		Links:     links,
		Entries:   entries,
	}

	if feedInfo != nil {
		if feedInfo.TotalResults > 0 {
			feed.TotalResults = &feedInfo.TotalResults
		}
		if feedInfo.StartIndex >= 0 {
			feed.StartIndex = &feedInfo.StartIndex
		}
		if feedInfo.ItemsPerPage > 0 {
			feed.ItemsPerPage = &feedInfo.ItemsPerPage
		}
	}

	return xml.MarshalIndent(feed, "", "  ")
}

// CreateBookEntry 创建书籍条目
func (g *Generator) CreateBookEntry(book *database.Book) Entry {
	entry := Entry{
		Title:   book.Title,
		ID:      fmt.Sprintf("urn:uuid:%s", book.UUID),
		Summary: book.Comments,
	}

	// 添加作者
	for _, author := range book.Authors {
		entry.Authors = append(entry.Authors, Author{Name: author.Name})
	}

	// 添加封面链接
	if book.HasCover {
		entry.Links = append(entry.Links, Link{
			Rel:  "http://opds-spec.org/image",
			Href: fmt.Sprintf("%s/opds/cover/%d", g.BaseURL, book.ID),
			Type: "image/jpeg",
		})
	}

	// 添加下载链接
	for i, format := range book.Formats {
		rel := "http://opds-spec.org/acquisition"
		if i == 0 {
			rel = "http://opds-spec.org/acquisition/open-access"
		}

		entry.Links = append(entry.Links, Link{
			Rel:    rel,
			Href:   fmt.Sprintf("%s/download/%d/%s", g.BaseURL, book.ID, format.Format),
			Type:   GetMimeType(format.Format),
			Title:  fmt.Sprintf("下载 %s", format.Format),
			Length: fmt.Sprintf("%d", format.Size),
		})
	}

	return entry
}

// CreateNavigationEntry 创建导航条目
func (g *Generator) CreateNavigationEntry(title, href, description string) Entry {
	return Entry{
		Title:   title,
		ID:      fmt.Sprintf("urn:uuid:%d", hashString(title)),
		Summary: description,
		Links: []Link{
			{
				Rel:  "http://opds-spec.org/subsection",
				Href: g.BaseURL + href,
				Type: "application/atom+xml;type=feed;profile=opds-catalog",
			},
		},
	}
}

// FeedInfo feed信息
type FeedInfo struct {
	TotalResults  int
	StartIndex    int
	ItemsPerPage  int
}

// GetMimeType 获取MIME类型
func GetMimeType(format string) string {
	mimeTypes := map[string]string{
		"EPUB": "application/epub+zip",
		"PDF":  "application/pdf",
		"MOBI": "application/x-mobipocket-ebook",
		"AZW3": "application/vnd.amazon.ebook",
		"FB2":  "application/x-fictionbook+xml",
		"RTF":  "application/rtf",
		"TXT":  "text/plain",
		"HTML": "text/html",
		"LIT":  "application/x-ms-reader",
	}

	if mime, ok := mimeTypes[format]; ok {
		return mime
	}
	return "application/octet-stream"
}

// 辅助函数
func generateUUID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	return h
}
