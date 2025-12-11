package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ricci/calibre-opds-go/internal/config"
	"github.com/ricci/calibre-opds-go/internal/database"
	"github.com/ricci/calibre-opds-go/internal/opds"
)

// Handler HTTP处理器
type Handler struct {
	db     *database.DB
	config *config.Config
}

// NewHandler 创建新的处理器
func NewHandler(db *database.DB, cfg *config.Config) *Handler {
	return &Handler{
		db:     db,
		config: cfg,
	}
}

// OPDSRoot OPDS根目录
func (h *Handler) OPDSRoot(c *gin.Context) {
	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	entries := []opds.Entry{
		gen.CreateNavigationEntry("最新书籍", "/opds/books", "按最近添加或修改的时间排序"),
		gen.CreateNavigationEntry("按作者浏览", "/opds/authors", "按作者分类的书籍"),
		gen.CreateNavigationEntry("按系列浏览", "/opds/series", "按系列分类的书籍"),
		gen.CreateNavigationEntry("按标签浏览", "/opds/tags", "按标签分类的书籍"),
	}

	links := []opds.Link{
		{
			Rel:  "self",
			Href: baseURL + "/opds",
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	xmlData, err := gen.CreateFeed("Calibre OPDS 目录", entries, links, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// OPDSBooks OPDS书籍列表
func (h *Handler) OPDSBooks(c *gin.Context) {
	search := c.Query("search")
	author := c.Query("author")
	series := c.Query("series")
	tag := c.Query("tag")
	limit := getIntParam(c, "limit", 20, 100)
	offset := getIntParam(c, "offset", 0, 0)

	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	// 获取过滤后的书籍
	books, err := h.db.GetBooksFiltered(limit, offset, search, author, series, tag)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get books")
		return
	}

	// 获取总数
	totalBooks, err := h.db.GetBooksCountFiltered(search, author, series, tag)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get book count")
		return
	}

	// 创建条目
	var entries []opds.Entry
	for _, book := range books {
		entries = append(entries, gen.CreateBookEntry(&book))
	}

	// 创建链接
	currentPage := offset/limit + 1
	totalPages := (totalBooks + limit - 1) / limit

	queryParams := url.Values{}
	if search != "" {
		queryParams.Set("search", search)
	}
	if author != "" {
		queryParams.Set("author", author)
	}
	if series != "" {
		queryParams.Set("series", series)
	}
	if tag != "" {
		queryParams.Set("tag", tag)
	}
	queryParams.Set("limit", strconv.Itoa(limit))
	queryParams.Set("offset", strconv.Itoa(offset))

	links := []opds.Link{
		{
			Rel:  "self",
			Href: fmt.Sprintf("%s/opds/books?%s", baseURL, queryParams.Encode()),
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	// 下一页链接
	if offset+limit < totalBooks {
		nextParams := url.Values{}
		if search != "" {
			nextParams.Set("search", search)
		}
		if author != "" {
			nextParams.Set("author", author)
		}
		if series != "" {
			nextParams.Set("series", series)
		}
		if tag != "" {
			nextParams.Set("tag", tag)
		}
		nextParams.Set("limit", strconv.Itoa(limit))
		nextParams.Set("offset", strconv.Itoa(offset+limit))

		links = append(links, opds.Link{
			Rel:   "next",
			Href:  fmt.Sprintf("%s/opds/books?%s", baseURL, nextParams.Encode()),
			Type:  "application/atom+xml;type=feed;profile=opds-catalog",
			Title: fmt.Sprintf("下一页 (第 %d 页)", currentPage+1),
		})
	}

	// 上一页链接
	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		prevParams := url.Values{}
		if search != "" {
			prevParams.Set("search", search)
		}
		if author != "" {
			prevParams.Set("author", author)
		}
		if series != "" {
			prevParams.Set("series", series)
		}
		if tag != "" {
			prevParams.Set("tag", tag)
		}
		prevParams.Set("limit", strconv.Itoa(limit))
		prevParams.Set("offset", strconv.Itoa(prevOffset))

		links = append(links, opds.Link{
			Rel:   "previous",
			Href:  fmt.Sprintf("%s/opds/books?%s", baseURL, prevParams.Encode()),
			Type:  "application/atom+xml;type=feed;profile=opds-catalog",
			Title: fmt.Sprintf("上一页 (第 %d 页)", currentPage-1),
		})
	}

	// 构建标题
	title := fmt.Sprintf("最新书籍列表 - 第 %d/%d 页", currentPage, totalPages)
	if author != "" {
		title = fmt.Sprintf("作者: %s - 第 %d/%d 页", author, currentPage, totalPages)
	} else if series != "" {
		title = fmt.Sprintf("系列: %s - 第 %d/%d 页", series, currentPage, totalPages)
	} else if tag != "" {
		title = fmt.Sprintf("标签: %s - 第 %d/%d 页", tag, currentPage, totalPages)
	} else if search != "" {
		title = fmt.Sprintf("搜索结果: \"%s\" - 第 %d/%d 页", search, currentPage, totalPages)
	}

	feedInfo := &opds.FeedInfo{
		TotalResults:  totalBooks,
		StartIndex:    offset,
		ItemsPerPage:  limit,
	}

	xmlData, err := gen.CreateFeed(title, entries, links, feedInfo)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// OPDSBookDetail OPDS书籍详情
func (h *Handler) OPDSBookDetail(c *gin.Context) {
	bookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := h.db.GetBookDetail(bookID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get book")
		return
	}
	if book == nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	entries := []opds.Entry{gen.CreateBookEntry(book)}
	links := []opds.Link{
		{
			Rel:  "self",
			Href: fmt.Sprintf("%s/opds/book/%d", baseURL, bookID),
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	xmlData, err := gen.CreateFeed(fmt.Sprintf("书籍详情: %s", book.Title), entries, links, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// OPDSAuthors OPDS作者列表
func (h *Handler) OPDSAuthors(c *gin.Context) {
	limit := getIntParam(c, "limit", 50, 100)
	offset := getIntParam(c, "offset", 0, 0)

	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	authors, err := h.db.GetAuthors(limit, offset)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get authors")
		return
	}

	var entries []opds.Entry
	for _, author := range authors {
		entry := gen.CreateNavigationEntry(
			fmt.Sprintf("%s (%d 本书)", author.Name, author.BookCount),
			fmt.Sprintf("/opds/books?author=%s", url.QueryEscape(author.Name)),
			fmt.Sprintf("作者: %s", author.Name),
		)
		entries = append(entries, entry)
	}

	links := []opds.Link{
		{
			Rel:  "self",
			Href: fmt.Sprintf("%s/opds/authors?limit=%d&offset=%d", baseURL, limit, offset),
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	currentPage := offset/limit + 1
	xmlData, err := gen.CreateFeed(fmt.Sprintf("按作者分类 - 第 %d 页", currentPage), entries, links, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// OPDSSeries OPDS系列列表
func (h *Handler) OPDSSeries(c *gin.Context) {
	limit := getIntParam(c, "limit", 50, 100)
	offset := getIntParam(c, "offset", 0, 0)

	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	seriesList, err := h.db.GetSeries(limit, offset)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get series")
		return
	}

	var entries []opds.Entry
	for _, series := range seriesList {
		entry := gen.CreateNavigationEntry(
			fmt.Sprintf("%s (%d 本书)", series.Name, series.BookCount),
			fmt.Sprintf("/opds/books?series=%s", url.QueryEscape(series.Name)),
			fmt.Sprintf("系列: %s", series.Name),
		)
		entries = append(entries, entry)
	}

	links := []opds.Link{
		{
			Rel:  "self",
			Href: fmt.Sprintf("%s/opds/series?limit=%d&offset=%d", baseURL, limit, offset),
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	currentPage := offset/limit + 1
	xmlData, err := gen.CreateFeed(fmt.Sprintf("按系列分类 - 第 %d 页", currentPage), entries, links, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// OPDSTags OPDS标签列表
func (h *Handler) OPDSTags(c *gin.Context) {
	limit := getIntParam(c, "limit", 50, 100)
	offset := getIntParam(c, "offset", 0, 0)

	baseURL := getBaseURL(c)
	gen := opds.NewGenerator(baseURL)

	tags, err := h.db.GetTags(limit, offset)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get tags")
		return
	}

	var entries []opds.Entry
	for _, tag := range tags {
		entry := gen.CreateNavigationEntry(
			fmt.Sprintf("%s (%d 本书)", tag.Name, tag.BookCount),
			fmt.Sprintf("/opds/books?tag=%s", url.QueryEscape(tag.Name)),
			fmt.Sprintf("标签: %s", tag.Name),
		)
		entries = append(entries, entry)
	}

	links := []opds.Link{
		{
			Rel:  "self",
			Href: fmt.Sprintf("%s/opds/tags?limit=%d&offset=%d", baseURL, limit, offset),
			Type: "application/atom+xml;type=feed;profile=opds-catalog",
		},
	}

	currentPage := offset/limit + 1
	xmlData, err := gen.CreateFeed(fmt.Sprintf("按标签分类 - 第 %d 页", currentPage), entries, links, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate feed")
		return
	}

	c.Data(http.StatusOK, "application/atom+xml;charset=utf-8", xmlData)
}

// 辅助函数
func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func getIntParam(c *gin.Context, key string, defaultValue, maxValue int) int {
	val := c.Query(key)
	if val == "" {
		return defaultValue
	}

	intVal, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}

	if maxValue > 0 && intVal > maxValue {
		return maxValue
	}

	return intVal
}
