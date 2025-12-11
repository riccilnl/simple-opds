package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库连接
type DB struct {
	conn *sql.DB
	path string
}

// NewDB 创建新的数据库连接
func NewDB(dbPath string) (*DB, error) {
	// 检查文件是否存在
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file not found: %s", dbPath)
	}

	// 打开数据库连接
	conn, err := sql.Open("sqlite3", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池参数
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// 测试连接
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: dbPath,
	}

	return db, nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Validate 验证数据库结构
func (db *DB) Validate() error {
	// 检查必要的表是否存在
	tables := []string{"books", "authors", "tags", "series", "data"}
	for _, table := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		err := db.conn.QueryRow(query, table).Scan(&name)
		if err == sql.ErrNoRows {
			return fmt.Errorf("required table '%s' not found", table)
		}
		if err != nil {
			return fmt.Errorf("failed to check table '%s': %w", table, err)
		}
	}

	log.Printf("Database validation successful")
	return nil
}

// GetBooksCount 获取书籍总数
func (db *DB) GetBooksCount(search string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if search != "" {
		query = `SELECT COUNT(DISTINCT b.id) FROM books b 
		         WHERE b.title LIKE ? OR b.author_sort LIKE ?`
		searchTerm := "%" + search + "%"
		args = []interface{}{searchTerm, searchTerm}
	} else {
		query = "SELECT COUNT(*) FROM books"
	}

	err := db.conn.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetBooksCountFiltered 获取过滤后的书籍总数
func (db *DB) GetBooksCountFiltered(search, author, series, tag string) (int, error) {
	query := "SELECT COUNT(DISTINCT b.id) FROM books b"
	var conditions []string
	var args []interface{}

	if search != "" {
		conditions = append(conditions, "(b.title LIKE ? OR b.author_sort LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if author != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_authors_link bal JOIN authors a ON bal.author = a.id WHERE bal.book = b.id AND a.name = ?)")
		args = append(args, author)
	}

	if series != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_series_link bsl JOIN series s ON bsl.series = s.id WHERE bsl.book = b.id AND s.name = ?)")
		args = append(args, series)
	}

	if tag != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_tags_link btl JOIN tags t ON btl.tag = t.id WHERE btl.book = b.id AND t.name = ?)")
		args = append(args, tag)
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions, " AND ")
	}

	var count int
	err := db.conn.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetBooks 获取书籍列表
func (db *DB) GetBooks(limit, offset int, search string) ([]Book, error) {
	query := `
		SELECT DISTINCT b.id, b.title, b.author_sort, b.path,
		       b.series_index, b.isbn, b.pubdate, b.last_modified,
		       b.has_cover, b.uuid
		FROM books b
	`
	
	var args []interface{}
	
	if search != "" {
		query += " WHERE (b.title LIKE ? OR b.author_sort LIKE ?)"
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}
	
	query += " ORDER BY b.last_modified DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	return db.executeBookQuery(query, args...)
}

// GetBooksFiltered 获取过滤后的书籍列表
func (db *DB) GetBooksFiltered(limit, offset int, search, author, series, tag string) ([]Book, error) {
	query := `
		SELECT DISTINCT b.id, b.title, b.author_sort, b.path,
		       b.series_index, b.isbn, b.pubdate, b.last_modified,
		       b.has_cover, b.uuid
		FROM books b
	`

	var conditions []string
	var args []interface{}

	if search != "" {
		conditions = append(conditions, "(b.title LIKE ? OR b.author_sort LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if author != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_authors_link bal JOIN authors a ON bal.author = a.id WHERE bal.book = b.id AND a.name = ?)")
		args = append(args, author)
	}

	if series != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_series_link bsl JOIN series s ON bsl.series = s.id WHERE bsl.book = b.id AND s.name = ?)")
		args = append(args, series)
	}

	if tag != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM books_tags_link btl JOIN tags t ON btl.tag = t.id WHERE btl.book = b.id AND t.name = ?)")
		args = append(args, tag)
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinConditions(conditions, " AND ")
	}

	query += " ORDER BY b.last_modified DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	return db.executeBookQuery(query, args...)
}

// executeBookQuery 执行书籍查询并加载关联数据
func (db *DB) executeBookQuery(query string, args ...interface{}) ([]Book, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var book Book
		err := rows.Scan(
			&book.ID, &book.Title, &book.AuthorSort, &book.Path,
			&book.SeriesIndex, &book.ISBN, &book.PubDate, &book.LastModified,
			&book.HasCover, &book.UUID,
		)
		if err != nil {
			return nil, err
		}

		// 加载关联数据
		book.Authors, _ = db.GetBookAuthors(book.ID)
		book.Tags, _ = db.GetBookTags(book.ID)
		book.Series, _ = db.GetBookSeries(book.ID)
		book.Formats, _ = db.GetBookFormats(book.ID)

		books = append(books, book)
	}

	return books, rows.Err()
}

// GetBookDetail 获取书籍详情
func (db *DB) GetBookDetail(bookID int) (*Book, error) {
	query := `
		SELECT b.id, b.title, b.author_sort, b.path, b.series_index,
		       b.isbn, b.pubdate, b.last_modified, b.has_cover, b.uuid
		FROM books b
		WHERE b.id = ?
	`

	var book Book
	err := db.conn.QueryRow(query, bookID).Scan(
		&book.ID, &book.Title, &book.AuthorSort, &book.Path,
		&book.SeriesIndex, &book.ISBN, &book.PubDate, &book.LastModified,
		&book.HasCover, &book.UUID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 获取评论
	var comments sql.NullString
	commentQuery := "SELECT text FROM comments WHERE book = ?"
	db.conn.QueryRow(commentQuery, bookID).Scan(&comments)
	if comments.Valid {
		book.Comments = comments.String
	}

	// 加载关联数据
	book.Authors, _ = db.GetBookAuthors(book.ID)
	book.Tags, _ = db.GetBookTags(book.ID)
	book.Series, _ = db.GetBookSeries(book.ID)
	book.Formats, _ = db.GetBookFormats(book.ID)

	return &book, nil
}

// GetBookAuthors 获取书籍作者
func (db *DB) GetBookAuthors(bookID int) ([]Author, error) {
	query := `
		SELECT a.name, a.sort
		FROM authors a
		JOIN books_authors_link bal ON a.id = bal.author
		WHERE bal.book = ?
		ORDER BY bal.id
	`

	rows, err := db.conn.Query(query, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []Author
	for rows.Next() {
		var author Author
		if err := rows.Scan(&author.Name, &author.Sort); err != nil {
			return nil, err
		}
		authors = append(authors, author)
	}

	return authors, rows.Err()
}

// GetBookTags 获取书籍标签
func (db *DB) GetBookTags(bookID int) ([]string, error) {
	query := `
		SELECT t.name
		FROM tags t
		JOIN books_tags_link btl ON t.id = btl.tag
		WHERE btl.book = ?
		ORDER BY t.name
	`

	rows, err := db.conn.Query(query, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetBookSeries 获取书籍系列
func (db *DB) GetBookSeries(bookID int) (*Series, error) {
	query := `
		SELECT s.name, s.sort, b.series_index
		FROM series s
		JOIN books_series_link bsl ON s.id = bsl.series
		JOIN books b ON bsl.book = b.id
		WHERE b.id = ?
	`

	var series Series
	err := db.conn.QueryRow(query, bookID).Scan(&series.Name, &series.Sort, &series.Index)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &series, nil
}

// GetBookFormats 获取书籍格式
func (db *DB) GetBookFormats(bookID int) ([]Format, error) {
	query := `
		SELECT format, uncompressed_size, name
		FROM data
		WHERE book = ?
		ORDER BY format
	`

	rows, err := db.conn.Query(query, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var formats []Format
	for rows.Next() {
		var format Format
		if err := rows.Scan(&format.Format, &format.Size, &format.Filename); err != nil {
			return nil, err
		}
		formats = append(formats, format)
	}

	return formats, rows.Err()
}

// GetAuthors 获取作者列表
func (db *DB) GetAuthors(limit, offset int) ([]AuthorInfo, error) {
	query := `
		SELECT DISTINCT a.name, a.sort, COUNT(b.id) as book_count
		FROM authors a
		JOIN books_authors_link bal ON a.id = bal.author
		JOIN books b ON bal.book = b.id
		GROUP BY a.id, a.name, a.sort
		ORDER BY a.sort
		LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []AuthorInfo
	for rows.Next() {
		var author AuthorInfo
		if err := rows.Scan(&author.Name, &author.Sort, &author.BookCount); err != nil {
			return nil, err
		}
		authors = append(authors, author)
	}

	return authors, rows.Err()
}

// GetSeries 获取系列列表
func (db *DB) GetSeries(limit, offset int) ([]SeriesInfo, error) {
	query := `
		SELECT DISTINCT s.name, s.sort, COUNT(b.id) as book_count
		FROM series s
		JOIN books_series_link bsl ON s.id = bsl.series
		JOIN books b ON bsl.book = b.id
		GROUP BY s.id, s.name, s.sort
		ORDER BY s.sort
		LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seriesList []SeriesInfo
	for rows.Next() {
		var series SeriesInfo
		if err := rows.Scan(&series.Name, &series.Sort, &series.BookCount); err != nil {
			return nil, err
		}
		seriesList = append(seriesList, series)
	}

	return seriesList, rows.Err()
}

// GetTags 获取标签列表
func (db *DB) GetTags(limit, offset int) ([]Tag, error) {
	query := `
		SELECT DISTINCT t.name, COUNT(b.id) as book_count
		FROM tags t
		JOIN books_tags_link btl ON t.id = btl.tag
		JOIN books b ON btl.book = b.id
		GROUP BY t.id, t.name
		ORDER BY t.name
		LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.Name, &tag.BookCount); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}

	return tags, rows.Err()
}

// GetStats 获取统计信息
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{
		Formats: make(map[string]int),
	}

	// 获取书籍总数
	err := db.conn.QueryRow("SELECT COUNT(*) FROM books").Scan(&stats.TotalBooks)
	if err != nil {
		return nil, err
	}

	// 获取作者总数
	err = db.conn.QueryRow("SELECT COUNT(*) FROM authors").Scan(&stats.TotalAuthors)
	if err != nil {
		return nil, err
	}

	// 获取格式统计
	rows, err := db.conn.Query("SELECT format, COUNT(*) FROM data GROUP BY format")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var format string
		var count int
		if err := rows.Scan(&format, &count); err != nil {
			return nil, err
		}
		stats.Formats[format] = count
	}

	return stats, rows.Err()
}

// 辅助函数
func joinConditions(conditions []string, separator string) string {
	result := ""
	for i, cond := range conditions {
		if i > 0 {
			result += separator
		}
		result += cond
	}
	return result
}
