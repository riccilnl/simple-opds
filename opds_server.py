#!/usr/bin/env python3
"""
Calibre OPDS服务主程序 - 数据库路径修复版本
基于Flask的OPDS (Open Publication Distribution System) 服务器
专为文石阅读器优化

版本历史:
- v2.1.5: 修复按作者/系列/标签过滤时的SQL构建错误 (incomplete input)
- v2.1.4: 修复数据库路径检测逻辑，解决"unable to open database file"错误
- v2.1.3: 修复容器环境数据库路径配置，支持绝对路径和相对路径
- v2.1.2: 修复数据库路径配置，支持本地metadata.db文件优先使用
- v2.1.1: 修复下载路由格式提取错误，确保URL与文件名正确匹配
- v2.1.0: 修复OPDSE下载URL构建，移除格式重复问题，确保文件名标准化
- v2.0.0: 函数重构和OPDS结构优化

当前版本: v2.1.5
"""
__version__ = "2.1.5"

import os
import sys
import sqlite3
import logging
import uuid
import threading
import re
from datetime import datetime, timezone
from flask import Flask, request, jsonify, Response, send_file, g
from werkzeug.exceptions import NotFound
import xml.etree.ElementTree as ET
from xml.dom import minidom
from urllib.parse import quote

# 导入编码转换工具
from encoding_utils import safe_convert_text, convert_dict_values, convert_list_values

# 配置日志
LOG_LEVEL = os.environ.get('LOG_LEVEL', 'INFO').upper()
LOG_FILE = os.environ.get('LOG_FILE', 'calibre_opds.log')
LOG_TO_CONSOLE = os.environ.get('LOG_TO_CONSOLE', 'true').lower() == 'true'

# 创建日志记录器
logger = logging.getLogger(__name__)
logger.setLevel(getattr(logging, LOG_LEVEL, logging.INFO))

# 创建格式化器
formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')

# 文件处理器
if LOG_FILE:
    try:
        from logging.handlers import RotatingFileHandler
        file_handler = RotatingFileHandler(
            LOG_FILE,
            maxBytes=10485760,  # 10MB
            backupCount=5,
            encoding='utf-8'
        )
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)
    except Exception as e:
        print(f"警告: 无法创建日志文件处理器: {e}")

# 控制台处理器
if LOG_TO_CONSOLE:
    console_handler = logging.StreamHandler()
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)

# 默认处理器
if not logger.handlers:
    logging.basicConfig(level=LOG_LEVEL, format='%(asctime)s - %(levelname)s - %(message)s')
    logger = logging.getLogger(__name__)

logger.info(f"日志系统初始化完成 - 级别: {LOG_LEVEL}")

# --- Flask 应用初始化 ---
app = Flask(__name__)

# --- Calibre 数据库访问类 ---
class CalibreDatabase:
    """Calibre数据库访问类 - 修复版本"""
    
    def __init__(self, db_path='/books/metadata.db'):
        # 智能数据库路径检测 - 修复版本
        self._db_valid = False
        self.db_path = self._find_valid_database(db_path)
        
        # 基础路径检测
        if os.path.exists('books/metadata.db'):
            self.base_books_path = 'books'
        elif os.path.exists('/books/metadata.db'):
            self.base_books_path = '/books'
        else:
            self.base_books_path = os.environ.get('CALIBRE_BOOKS_PATH', '/books')
        
        # SQLite连接配置
        self.connection_timeout = float(os.environ.get('DB_CONNECTION_TIMEOUT', '30.0'))
        self._connection_stats = {
            'created': 0,
            'reused': 0,
            'closed': 0,
            'errors': 0
        }
        self._stats_lock = threading.Lock()
    
    def _find_valid_database(self, default_path):
        """查找有效的数据库文件"""
        db_candidates = [
            'books/metadata.db',      # 优先使用已验证的正确位置
            '/books/metadata.db',     # 容器环境标准位置
            'metadata.db'             # 最后尝试根目录
        ]
        
        for db_file in db_candidates:
            if os.path.exists(db_file):
                if self._validate_database_file(db_file):
                    logger.info(f"✅ 找到有效数据库文件: {db_file}")
                    return db_file
                else:
                    logger.warning(f"⚠️ 跳过无效数据库文件: {db_file}")
        
        # 如果没有找到有效数据库，使用环境变量或默认值
        fallback_path = os.environ.get('CALIBRE_DB_PATH', default_path)
        logger.error(f"❌ 未找到有效数据库文件，使用配置路径: {fallback_path}")
        return fallback_path
    
    def _validate_database_file(self, db_path):
        """验证数据库文件是否有效"""
        try:
            # 检查文件大小
            if os.path.getsize(db_path) == 0:
                logger.debug(f"数据库文件为空: {db_path}")
                return False
            
            # 尝试连接并执行基本查询
            conn = sqlite3.connect(db_path, timeout=5.0)
            cursor = conn.cursor()
            
            # 检查是否包含必要的表
            cursor.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='books'")
            if not cursor.fetchone():
                conn.close()
                logger.debug(f"数据库缺少books表: {db_path}")
                return False
            
            # 测试基本查询
            cursor.execute("SELECT COUNT(*) FROM books")
            book_count = cursor.fetchone()[0]
            
            conn.close()
            
            logger.debug(f"数据库验证成功: {db_path}, 书籍数量: {book_count}")
            return True
            
        except sqlite3.Error as e:
            logger.debug(f"数据库验证失败: {db_path}, 错误: {e}")
            return False
        except Exception as e:
            logger.debug(f"数据库验证异常: {db_path}, 错误: {e}")
            return False
    
    def get_connection(self):
        """获取数据库连接 - 每请求模式"""
        try:
            # 预先验证数据库文件
            if not hasattr(self, '_validated') or not self._validated:
                if not os.path.exists(self.db_path):
                    logger.error(f"数据库文件不存在: {self.db_path}")
                    raise FileNotFoundError(f"Calibre数据库文件不存在: {self.db_path}")
                
                # 重新验证数据库文件
                if not self._validate_database_file(self.db_path):
                    logger.error(f"数据库文件无效: {self.db_path}")
                    raise sqlite3.DatabaseError(f"数据库文件损坏或格式错误: {self.db_path}")
                
                self._validated = True
            
            # 使用Flask g对象管理当前应用上下文的连接
            if not hasattr(g, 'db_conn'):
                conn = sqlite3.connect(self.db_path, timeout=self.connection_timeout)
                conn.row_factory = sqlite3.Row
                
                g.db_conn = conn
                with self._stats_lock:
                    self._connection_stats['created'] += 1
                logger.debug(f"创建数据库连接: {self.db_path}")
                
                # 只对可写数据库设置PRAGMA
                try:
                    conn.execute("PRAGMA foreign_keys = ON")
                    conn.execute("PRAGMA journal_mode = WAL")
                    conn.execute("PRAGMA synchronous = NORMAL")
                    logger.debug("数据库PRAGMA设置成功")
                except sqlite3.Error as e:
                    logger.debug(f"PRAGMA设置跳过（数据库为只读）: {e}")
            else:
                with self._stats_lock:
                    self._connection_stats['reused'] += 1
            
            return g.db_conn
            
        except sqlite3.Error as e:
            with self._stats_lock:
                self._connection_stats['errors'] += 1
            logger.error(f"数据库连接失败: {e}")
            logger.error(f"尝试连接的数据库路径: {self.db_path}")
            raise
        except Exception as e:
            with self._stats_lock:
                self._connection_stats['errors'] += 1
            logger.error(f"获取数据库连接时发生未知错误: {e}")
            raise
    
    def close_connection(self, conn=None):
        """显式关闭数据库连接"""
        try:
            if conn:
                try:
                    conn.close()
                    with self._stats_lock:
                        self._connection_stats['closed'] += 1
                    logger.debug(f"关闭数据库连接")
                except Exception as close_error:
                    logger.warning(f"关闭连接时发生错误: {close_error}")
        except Exception as e:
            logger.error(f"关闭数据库连接时发生未知错误: {e}")
    
    def get_connection_stats(self):
        """获取连接统计信息"""
        with self._stats_lock:
            stats_copy = self._connection_stats.copy()
        return {
            'connection_strategy': 'per_request',
            'stats': stats_copy,
            'database_path': self.db_path,
            'base_books_path': self.base_books_path
        }
    
    def check_connection_health(self, conn):
        """检查数据库连接健康状态"""
        try:
            if conn:
                cursor = conn.cursor()
                cursor.execute("SELECT 1")
                result = cursor.fetchone()
                return result is not None and result[0] == 1
        except Exception as e:
            logger.warning(f"数据库连接健康检查失败: {e}")
            return False
        return False
    
    def execute_and_log_errors(self, func):
        """执行数据库操作并统一记录错误"""
        try:
            return func()
        except sqlite3.OperationalError as e:
            if "database is locked" in str(e):
                logger.error(f"数据库锁定错误: {e}")
            else:
                logger.error(f"数据库操作失败: {e}")
            raise
        except sqlite3.Error as e:
            logger.error(f"数据库错误: {e}")
            raise
        except Exception as e:
            logger.error(f"执行数据库操作时发生未知错误: {e}")
            raise
    
    def get_all_books(self, limit=20, offset=0, search=None):
        """获取所有书籍列表"""
        def _get_all_books():
            conn = self.get_connection()
            cursor = conn.cursor()
            base_query = """
                SELECT DISTINCT b.id, b.title, b.author_sort, b.path,
                        b.series_index, b.isbn, b.pubdate, b.last_modified,
                        b.has_cover, b.uuid
                FROM books b
            """
            conditions = []
            params = []
            if search:
                conditions.append("(b.title LIKE ? OR b.author_sort LIKE ?)")
                search_term = f"%{search}%"
                params.extend([search_term, search_term])
            
            if conditions:
                base_query += " WHERE " + " AND ".join(conditions)
            
            base_query += " ORDER BY b.last_modified DESC LIMIT ? OFFSET ?"
            params.extend([limit, offset])
            cursor.execute(base_query, params)
            books = cursor.fetchall()
            book_list = []
            for book in books:
                book_dict = dict(book)
                book_dict['title'] = safe_convert_text(book_dict['title'])
                book_dict['author_sort'] = safe_convert_text(book_dict['author_sort'])
                
                book_dict['authors'] = self.get_book_authors(book['id'])
                book_dict['tags'] = self.get_book_tags(book['id'])
                book_dict['series'] = self.get_book_series(book['id'])
                book_dict['formats'] = self.get_book_formats(book['id'])
                book_dict['has_cover'] = bool(book['has_cover'])
                book_list.append(book_dict)
            return book_list
        
        return self.execute_and_log_errors(_get_all_books)
    
    def get_books_count(self, search=None):
        """获取书籍总数"""
        def _get_books_count():
            conn = self.get_connection()
            cursor = conn.cursor()
            
            if search:
                cursor.execute("""
                    SELECT COUNT(DISTINCT b.id)
                    FROM books b
                    WHERE b.title LIKE ? OR b.author_sort LIKE ?
                """, (f"%{search}%", f"%{search}%"))
            else:
                cursor.execute("SELECT COUNT(*) FROM books")
            
            return cursor.fetchone()[0]
        
        return self.execute_and_log_errors(_get_books_count)
    
    def get_book_authors(self, book_id):
        """获取书籍作者信息"""
        def _get_authors():
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute("""
                SELECT a.name, a.sort
                FROM authors a
                JOIN books_authors_link bal ON a.id = bal.author
                WHERE bal.book = ?
                ORDER BY bal.id
            """, (book_id,))
            authors = []
            for row in cursor.fetchall():
                authors.append({
                    'name': safe_convert_text(row[0]),
                    'sort': safe_convert_text(row[1])
                })
            return authors
        
        return self.execute_and_log_errors(_get_authors)
    
    def get_book_tags(self, book_id):
        """获取书籍标签信息"""
        def _get_tags():
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute("""
                SELECT t.name
                FROM tags t
                JOIN books_tags_link btl ON t.id = btl.tag
                WHERE btl.book = ?
                ORDER BY t.name
            """, (book_id,))
            return [safe_convert_text(row[0]) for row in cursor.fetchall()]
        
        return self.execute_and_log_errors(_get_tags)
    
    def get_book_series(self, book_id):
        """获取书籍系列信息"""
        def _get_series():
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute("""
                SELECT s.name, s.sort, b.series_index
                FROM series s
                JOIN books_series_link bsl ON s.id = bsl.series
                JOIN books b ON bsl.book = b.id
                WHERE b.id = ?
            """, (book_id,))
            result = cursor.fetchone()
            if result:
                return {
                    'name': safe_convert_text(result[0]),
                    'sort': safe_convert_text(result[1]),
                    'index': result[2]
                }
            return None
        
        return self.execute_and_log_errors(_get_series)
    
    def get_book_formats(self, book_id):
        """获取书籍格式信息"""
        def _get_formats():
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute("""
                SELECT format, uncompressed_size, name
                FROM data
                WHERE book = ?
                ORDER BY format
            """, (book_id,))
            formats = []
            for row in cursor.fetchall():
                formats.append({
                    'format': row[0], 'size': row[1], 'filename': row[2]
                })
            return formats
        
        return self.execute_and_log_errors(_get_formats)
    
    def get_book_detail(self, book_id):
        """获取书籍详细信息"""
        def _get_book_detail():
            conn = self.get_connection()
            cursor = conn.cursor()
            cursor.execute("""
                SELECT b.id, b.title, b.author_sort, b.path, b.series_index,
                        b.isbn, b.pubdate, b.last_modified, b.has_cover,
                        b.uuid, b.flags, b.lccn
                FROM books b
                WHERE b.id = ?
            """, (book_id,))
            book = cursor.fetchone()
            if not book:
                return None
            
            book_dict = dict(book)
            book_dict['title'] = safe_convert_text(book_dict['title'])
            book_dict['author_sort'] = safe_convert_text(book_dict['author_sort'])
            
            cursor.execute("SELECT text FROM comments WHERE book = ?", (book_id,))
            comment_row = cursor.fetchone()
            book_dict['comments'] = safe_convert_text(comment_row[0]) if comment_row else ""
            
            book_dict['authors'] = self.get_book_authors(book_id)
            book_dict['tags'] = self.get_book_tags(book_id)
            book_dict['series'] = self.get_book_series(book_id)
            book_dict['formats'] = self.get_book_formats(book_id)
            book_dict['has_cover'] = bool(book_dict['has_cover'])
            return book_dict
        
        return self.execute_and_log_errors(_get_book_detail)

# --- OPDS XML 生成器 ---
class OPDSGenerator:
    """OPDS XML生成器 - 专为文石阅读器优化"""
    
    def __init__(self, base_url):
        if not base_url:
            raise ValueError("OPDSGenerator必须在路由函数中通过 request.url_root 初始化")
        self.base_url = base_url.rstrip('/')

    def create_feed(self, title, entries=None, links=None, feed_info=None):
        """创建OPDS feed"""
        feed = ET.Element('feed')
        feed.set('xmlns', 'http://www.w3.org/2005/Atom')
        feed.set('xmlns:opds', 'http://opds-spec.org/2010/catalog')
        
        # 添加基本信息
        ET.SubElement(feed, 'title').text = title
        ET.SubElement(feed, 'id').text = f"urn:uuid:{uuid.uuid4()}"
        ET.SubElement(feed, 'updated').text = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')
        
        # 添加分页信息
        if feed_info:
            if 'total_results' in feed_info:
                total_elem = ET.SubElement(feed, 'opds:totalResults')
                total_elem.text = str(feed_info['total_results'])
            
            if 'start_index' in feed_info:
                start_elem = ET.SubElement(feed, 'opds:startIndex')
                start_elem.text = str(feed_info['start_index'])
            
            if 'items_per_page' in feed_info:
                items_elem = ET.SubElement(feed, 'opds:itemsPerPage')
                items_elem.text = str(feed_info['items_per_page'])
        
        if links is None:
            links = []
        
        # 添加导航链接
        for link_info in links:
            link_elem = ET.SubElement(feed, 'link')
            link_elem.set('rel', link_info.get('rel', 'self'))
            link_elem.set('href', link_info['href'])
            link_elem.set('type', link_info.get('type', 'application/atom+xml;type=feed;profile=opds-catalog'))
            if 'title' in link_info:
                link_elem.set('title', link_info['title'])
        
        # 添加条目
        if entries:
            for entry in entries:
                if isinstance(entry, dict):
                    entry_elem = self.create_entry(entry)
                else:
                    entry_elem = entry
                feed.append(entry_elem)
        
        return self.prettify_xml(feed)
    
    def create_entry(self, book_data):
        """创建书籍条目 - 文石优化版本"""
        entry = ET.Element('entry')
        
        ET.SubElement(entry, 'title').text = book_data['title']
        
        if book_data.get('authors'):
            for author in book_data['authors']:
                author_elem = ET.SubElement(entry, 'author')
                ET.SubElement(author_elem, 'name').text = author['name']
        
        if book_data.get('comments'):
            summary = ET.SubElement(entry, 'summary')
            summary.text = book_data['comments']
        
        ET.SubElement(entry, 'id').text = f"urn:uuid:{book_data.get('uuid', uuid.uuid4())}"
        
        # 封面链接
        if book_data.get('has_cover'):
            cover_link = ET.SubElement(entry, 'link')
            cover_link.set('rel', 'http://opds-spec.org/image')
            cover_link.set('href', f"{self.base_url}/opds/cover/{book_data['id']}")
            cover_link.set('type', 'image/jpeg')
        
        # 直接下载链接 - 主要交互方式
        if book_data.get('formats'):
            for fmt in book_data['formats']:
                download_link = ET.SubElement(entry, 'link')
                
                # 第一个格式作为主要下载链接
                if fmt == book_data['formats'][0]:
                    download_link.set('rel', 'http://opds-spec.org/acquisition/open-access')
                else:
                    download_link.set('rel', 'http://opds-spec.org/acquisition')
                
                # 关键修正：使用书名构建下载文件名，只包含书名和扩展名
                book_title = book_data['title']
                
                # 只使用书名作为基础文件名，不包含格式名
                # 例如: 《三体》.epub
                filename_base = book_title
                
                # 确保文件名安全，但保留中文
                # 放宽清理规则，只移除路径非法字符
                safe_filename = re.sub(r'[<>:"/\\|?*]', '', filename_base)
                safe_filename = safe_filename.replace(' ', '_')
                
                # 确保有扩展名（强制小写）
                format_extension_map = {
                    'EPUB': '.epub', 'PDF': '.pdf', 'MOBI': '.mobi',
                    'AZW3': '.azw3', 'FB2': '.fb2', 'RTF': '.rtf',
                    'TXT': '.txt', 'HTML': '.html', 'LIT': '.lit'
                }
                ext = format_extension_map.get(fmt['format'].upper(), '.epub')  # 默认使用小写
                if ext and not safe_filename.lower().endswith(ext.lower()):
                    safe_filename += ext
                
                download_link.set('href', f"{self.base_url}/download/{book_data['id']}/{fmt['format']}")
                download_link.set('type', self.get_mime_type(fmt['format']))
                download_link.set('title', f"下载 {fmt['format']}")
                if fmt.get('size'):
                    download_link.set('length', str(fmt['size']))
        
        return entry
    
    def create_navigation_entry(self, title, href, description=""):
        """创建导航条目"""
        entry = ET.Element('entry')
        ET.SubElement(entry, 'title').text = title
        if description:
            ET.SubElement(entry, 'summary').text = description
        ET.SubElement(entry, 'id').text = f"urn:uuid:{hash(title)}"
        link = ET.SubElement(entry, 'link')
        link.set('rel', 'http://opds-spec.org/subsection')
        link.set('href', f"{self.base_url}{href}" if href.startswith('/') else href)
        link.set('type', 'application/atom+xml;type=feed;profile=opds-catalog')
        return entry
    
    def get_mime_type(self, format_name):
        """获取文件格式的MIME类型"""
        mime_types = {
            'EPUB': 'application/epub+zip', 'PDF': 'application/pdf',
            'MOBI': 'application/x-mobipocket-ebook', 'AZW3': 'application/vnd.amazon.ebook',
            'FB2': 'application/x-fictionbook+xml', 'RTF': 'application/rtf',
            'TXT': 'text/plain', 'HTML': 'text/html', 'LIT': 'application/x-ms-reader'
        }
        return mime_types.get(format_name.upper(), 'application/octet-stream')
    
    def prettify_xml(self, elem):
        """格式化XML输出"""
        rough_string = ET.tostring(elem, 'utf-8')
        reparsed = minidom.parseString(rough_string)
        return reparsed.toprettyxml(indent="  ", encoding='utf-8').decode('utf-8')

# --- 实例化对象 ---
db = CalibreDatabase()

# --- SQLite 连接管理 ---
@app.teardown_appcontext
def close_db_connection(exception):
    """在应用上下文结束时关闭数据库连接"""
    try:
        db_conn = g.pop('db_conn', None)
        if db_conn is not None:
            db.close_connection(db_conn)
            logger.debug(f"应用上下文结束，关闭数据库连接")
    except Exception as e:
        logger.error(f"应用上下文结束时关闭连接失败: {e}")

# --- OPDS 路由定义 ---
@app.route('/opds')
def opds_root():
    """OPDS根目录"""
    opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
    entries = []
    entries.append(opds.create_navigation_entry('最新书籍','/opds/books','按最近添加或修改的时间排序'))
    entries.append(opds.create_navigation_entry('按作者浏览','/opds/authors','按作者分类的书籍'))
    entries.append(opds.create_navigation_entry('按系列浏览','/opds/series','按系列分类的书籍'))
    entries.append(opds.create_navigation_entry('按标签浏览','/opds/tags','按标签分类的书籍'))
    xml_content = opds.create_feed('Calibre OPDS 目录', entries)
    return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')

@app.route('/opds/books')
def opds_books():
    """OPDS书籍列表 - 支持搜索和分类过滤"""
    search = request.args.get('search')
    author = request.args.get('author')
    series = request.args.get('series')
    tag = request.args.get('tag')
    limit = min(int(request.args.get('limit', 20)), 100)
    offset = int(request.args.get('offset', 0))
    
    # 构建过滤条件
    filters = []
    params = []
    
    if search:
        filters.append("(b.title LIKE ? OR b.author_sort LIKE ?)")
        search_term = f"%{search}%"
        params.extend([search_term, search_term])
    
    if author:
        filters.append("EXISTS (SELECT 1 FROM books_authors_link bal JOIN authors a ON bal.author = a.id WHERE bal.book = b.id AND a.name = ?)")
        params.append(author)
    
    if series:
        filters.append("EXISTS (SELECT 1 FROM books_series_link bsl JOIN series s ON bsl.series = s.id WHERE bsl.book = b.id AND s.name = ?)")
        params.append(series)
    
    if tag:
        filters.append("EXISTS (SELECT 1 FROM books_tags_link btl JOIN tags t ON btl.tag = t.id WHERE btl.book = b.id AND t.name = ?)")
        params.append(tag)
    
    # 获取过滤后的书籍
    def _get_filtered_books():
        conn = db.get_connection()
        cursor = conn.cursor()
        
        base_query = """
            SELECT DISTINCT b.id, b.title, b.author_sort, b.path,
                    b.series_index, b.isbn, b.pubdate, b.last_modified,
                    b.has_cover, b.uuid
            FROM books b
        """
        
        if filters:
            base_query += " WHERE " + " AND ".join(filters)
        
        base_query += " ORDER BY b.last_modified DESC LIMIT ? OFFSET ?"
        params.extend([limit, offset])
        
        cursor.execute(base_query, params)
        books = cursor.fetchall()
        
        book_list = []
        for book in books:
            book_dict = dict(book)
            book_dict['title'] = safe_convert_text(book_dict['title'])
            book_dict['author_sort'] = safe_convert_text(book_dict['author_sort'])
            
            book_dict['authors'] = db.get_book_authors(book['id'])
            book_dict['tags'] = db.get_book_tags(book['id'])
            book_dict['series'] = db.get_book_series(book['id'])
            book_dict['formats'] = db.get_book_formats(book['id'])
            book_dict['has_cover'] = bool(book['has_cover'])
            book_list.append(book_dict)
        return book_list
    
    def _get_filtered_count():
        conn = db.get_connection()
        cursor = conn.cursor()
        
        count_query = "SELECT COUNT(DISTINCT b.id) FROM books b"
        count_params = []
        
        if filters:
            # 对于COUNT查询，需要调整WHERE子句
            if search:
                count_query += " WHERE (b.title LIKE ? OR b.author_sort LIKE ?)"
                count_params.extend([f"%{search}%", f"%{search}%"])
            
            if author:
                if not search:
                    count_query += " WHERE EXISTS (SELECT 1 FROM books_authors_link bal JOIN authors a ON bal.author = a.id WHERE bal.book = b.id AND a.name = ?)"
                else:
                    count_query += " AND EXISTS (SELECT 1 FROM books_authors_link bal JOIN authors a ON bal.author = a.id WHERE bal.book = b.id AND a.name = ?)"
                count_params.append(author)
            
            if series:
                if not search and not author:
                    count_query += " WHERE"
                else:
                    count_query += " AND"
                count_query += " EXISTS (SELECT 1 FROM books_series_link bsl JOIN series s ON bsl.series = s.id WHERE bsl.book = b.id AND s.name = ?)"
                count_params.append(series)
            
            if tag:
                if not search and not author and not series:
                    count_query += " WHERE"
                else:
                    count_query += " AND"
                count_query += " EXISTS (SELECT 1 FROM books_tags_link btl JOIN tags t ON btl.tag = t.id WHERE btl.book = b.id AND t.name = ?)"
                count_params.append(tag)
        else:
            count_params = []
        
        cursor.execute(count_query, count_params)
        return cursor.fetchone()[0]
    
    try:
        books = db.execute_and_log_errors(_get_filtered_books)
        total_books = db.execute_and_log_errors(_get_filtered_count)
        
        opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
        base_url = f"{request.url_root.rstrip('/')}/opds/books"
        
        # 构建查询参数
        query_params = []
        if search:
            query_params.append(f'search={quote(search)}')
        if author:
            query_params.append(f'author={quote(author)}')
        if series:
            query_params.append(f'series={quote(series)}')
        if tag:
            query_params.append(f'tag={quote(tag)}')
        query_params.extend([f'limit={limit}', f'offset={offset}'])
        
        query_string = '&'.join(query_params) if query_params else f'limit={limit}&offset={offset}'
        
        links = [
            {'rel': 'self', 'href': f'{base_url}?{query_string}', 'type': 'application/atom+xml;type=feed;profile=opds-catalog'}
        ]
        
        # 添加分页导航链接
        if offset + limit < total_books:
            next_offset = offset + limit
            next_params = query_params.copy()
            next_params[-1] = f'offset={next_offset}'
            links.append({
                'rel': 'next',
                'href': f'{base_url}?{"&".join(next_params)}',
                'type': 'application/atom+xml;type=feed;profile=opds-catalog',
                'title': f'下一页 (第 {offset//limit + 2} 页)'
            })
        
        if offset > 0:
            prev_offset = max(0, offset - limit)
            prev_params = query_params.copy()
            prev_params[-1] = f'offset={prev_offset}'
            links.append({
                'rel': 'previous',
                'href': f'{base_url}?{"&".join(prev_params)}',
                'type': 'application/atom+xml;type=feed;profile=opds-catalog',
                'title': f'上一页 (第 {offset//limit} 页)'
            })
        
        # 构建feed标题
        current_page = offset // limit + 1
        total_pages = (total_books + limit - 1) // limit
        
        if author:
            title = f'作者: {author} - 第 {current_page}/{total_pages} 页'
        elif series:
            title = f'系列: {series} - 第 {current_page}/{total_pages} 页'
        elif tag:
            title = f'标签: {tag} - 第 {current_page}/{total_pages} 页'
        elif search:
            title = f'搜索结果: "{search}" - 第 {current_page}/{total_pages} 页'
        else:
            title = f'最新书籍列表 - 第 {current_page}/{total_pages} 页'
        
        feed_info = {
            'total_results': total_books,
            'start_index': offset,
            'items_per_page': limit
        }
        
        xml_content = opds.create_feed(title, books, links, feed_info=feed_info)
        return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')
    except Exception as e:
        logger.error(f"获取书籍列表失败: {e}")
        return NotFound()

@app.route('/opds/authors')
def opds_authors():
    """按作者分类的OPDS列表"""
    limit = min(int(request.args.get('limit', 50)), 100)
    offset = int(request.args.get('offset', 0))
    
    def _get_authors():
        conn = db.get_connection()
        cursor = conn.cursor()
        cursor.execute("""
            SELECT DISTINCT a.name, a.sort, COUNT(b.id) as book_count
            FROM authors a
            JOIN books_authors_link bal ON a.id = bal.author
            JOIN books b ON bal.book = b.id
            GROUP BY a.id, a.name, a.sort
            ORDER BY a.sort
            LIMIT ? OFFSET ?
        """, (limit, offset))
        return [dict(row) for row in cursor.fetchall()]
    
    try:
        authors = db.execute_and_log_errors(_get_authors)
        
        opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
        entries = []
        
        for author in authors:
            safe_name = safe_convert_text(author['name'])
            entry = opds.create_navigation_entry(
                f"{safe_name} ({author['book_count']} 本书)",
                f"/opds/books?author={quote(safe_name)}",
                f"作者: {safe_name}"
            )
            entries.append(entry)
        
        xml_content = opds.create_feed(f'按作者分类 - 第 {offset//limit + 1} 页', entries)
        return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')
    except Exception as e:
        logger.error(f"按作者分类获取失败: {e}")
        return NotFound()

@app.route('/opds/series')
def opds_series():
    """按系列分类的OPDS列表"""
    limit = min(int(request.args.get('limit', 50)), 100)
    offset = int(request.args.get('offset', 0))
    
    def _get_series():
        conn = db.get_connection()
        cursor = conn.cursor()
        cursor.execute("""
            SELECT DISTINCT s.name, s.sort, COUNT(b.id) as book_count
            FROM series s
            JOIN books_series_link bsl ON s.id = bsl.series
            JOIN books b ON bsl.book = b.id
            GROUP BY s.id, s.name, s.sort
            ORDER BY s.sort
            LIMIT ? OFFSET ?
        """, (limit, offset))
        return [dict(row) for row in cursor.fetchall()]
    
    try:
        series = db.execute_and_log_errors(_get_series)
        
        opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
        entries = []
        
        for serie in series:
            safe_name = safe_convert_text(serie['name'])
            entry = opds.create_navigation_entry(
                f"{safe_name} ({serie['book_count']} 本书)",
                f"/opds/books?series={quote(safe_name)}",
                f"系列: {safe_name}"
            )
            entries.append(entry)
        
        xml_content = opds.create_feed(f'按系列分类 - 第 {offset//limit + 1} 页', entries)
        return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')
    except Exception as e:
        logger.error(f"按系列分类获取失败: {e}")
        return NotFound()

@app.route('/opds/tags')
def opds_tags():
    """按标签分类的OPDS列表"""
    limit = min(int(request.args.get('limit', 50)), 100)
    offset = int(request.args.get('offset', 0))
    
    def _get_tags():
        conn = db.get_connection()
        cursor = conn.cursor()
        cursor.execute("""
            SELECT DISTINCT t.name, COUNT(b.id) as book_count
            FROM tags t
            JOIN books_tags_link btl ON t.id = btl.tag
            JOIN books b ON btl.book = b.id
            GROUP BY t.id, t.name
            ORDER BY t.name
            LIMIT ? OFFSET ?
        """, (limit, offset))
        return [dict(row) for row in cursor.fetchall()]
    
    try:
        tags = db.execute_and_log_errors(_get_tags)
        
        opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
        entries = []
        
        for tag in tags:
            safe_name = safe_convert_text(tag['name'])
            entry = opds.create_navigation_entry(
                f"{safe_name} ({tag['book_count']} 本书)",
                f"/opds/books?tag={quote(safe_name)}",
                f"标签: {safe_name}"
            )
            entries.append(entry)
        
        xml_content = opds.create_feed(f'按标签分类 - 第 {offset//limit + 1} 页', entries)
        return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')
    except Exception as e:
        logger.error(f"按标签分类获取失败: {e}")
        return NotFound()

@app.route('/opds/book/<int:book_id>')
def opds_book_detail(book_id):
    """书籍详情"""
    book = db.get_book_detail(book_id)
    if not book:
        return NotFound()
    
    opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
    entries = [book] if book else []
    xml_content = opds.create_feed(f'书籍详情: {book["title"]}', entries)
    return Response(xml_content, mimetype='application/atom+xml;charset=utf-8')

@app.route('/opds/cover/<int:book_id>')
def get_cover(book_id):
    """获取书籍封面"""
    try:
        book = db.get_book_detail(book_id)
        if not book or not book.get('path'):
            return NotFound("Book or path not found")
        
        base_path = db.base_books_path
        book_path = book['path'].replace('\\', '/')
        
        cover_extensions = ['.jpg', '.png']
        
        for ext in cover_extensions:
            cover_filename = f'cover{ext}'
            cover_path = os.path.join(base_path, book_path, cover_filename)
            
            if os.path.exists(cover_path):
                if ext.lower() == '.png':
                    mime_type = 'image/png'
                elif ext.lower() in ['.jpg', '.jpeg']:
                    mime_type = 'image/jpeg'
                else:
                    mime_type = 'image/jpeg'
                
                return send_file(cover_path, mimetype=mime_type)
        
        return NotFound("Cover not found")
        
    except Exception as e:
        logger.error(f"获取封面失败: {e}")
        return jsonify({'error': 'Internal server error'}), 500

# --- 文件服务路由 (下载) ---
@app.route('/download/<int:book_id>/<path:filename>')
def download_book(book_id, filename):
    """下载书籍 - 文石优化版本"""
    try:
        book = db.get_book_detail(book_id)
        if not book:
            logger.warning(f"书籍未找到 - ID: {book_id}")
            return NotFound(f"Book not found: ID {book_id}")
        
        # 从文件名中提取格式（标准格式：/download/{book_id}/{format}）
        requested_format = filename.upper()
        base_filename = None
        
        # 查找匹配的格式
        target_format = None
        for fmt in book['formats']:
            if fmt['format'].upper() == requested_format:
                target_format = fmt
                break
        
        if not target_format:
            available_formats = [fmt['format'] for fmt in book['formats']]
            logger.warning(f"格式未找到 - 书籍ID: {book_id}, 请求格式: {requested_format}, 可用格式: {available_formats}")
            return NotFound(f"Format {requested_format} not found for this book. Available formats: {', '.join(available_formats)}")
        
        # 构建文件路径
        base_path = db.base_books_path
        book_path = book['path'].replace('\\', '/')
        db_filename = target_format['filename']
        
        # 尝试多个可能的文件路径
        possible_files = []
        
        # 1. 数据库中的原始文件名
        possible_files.append(os.path.join(base_path, book_path, db_filename))
        
        # 2. 添加扩展名
        format_extension_map = {
            'EPUB': '.epub', 'PDF': '.pdf', 'MOBI': '.mobi',
            'AZW3': '.azw3', 'FB2': '.fb2', 'RTF': '.rtf',
            'TXT': '.txt', 'HTML': '.html', 'LIT': '.lit'
        }
        ext = format_extension_map.get(target_format['format'].upper(), '')
        if ext and not db_filename.lower().endswith(ext.lower()):
            possible_files.append(os.path.join(base_path, book_path, db_filename + ext))
        
        # 3. 如果URL中包含文件名，尝试精确匹配
        if base_filename:
            possible_files.append(os.path.join(base_path, book_path, base_filename + ext))
        
        # 4. 查找目录中匹配的文件
        book_dir = os.path.join(base_path, book_path)
        if os.path.exists(book_dir):
            for file in os.listdir(book_dir):
                file_lower = file.lower()
                if ext and file_lower.endswith(ext.lower()):
                    possible_files.append(os.path.join(book_dir, file))
                if base_filename and file_lower.startswith(base_filename.lower()):
                    possible_files.append(os.path.join(book_dir, file))
        
        # 查找存在的文件
        full_path = None
        for path in possible_files:
            normalized_path = os.path.normpath(path)
            if os.path.exists(normalized_path):
                full_path = normalized_path
                break
        
        if not full_path:
            logger.error(f"文件不存在: 已尝试路径 {possible_files}")
            return NotFound(f"File not found for book {book_id} in format {target_format['format']}")
        
        logger.info(f"成功找到文件，开始下载: {os.path.basename(full_path)}, 大小: {os.path.getsize(full_path)} 字节")
        
        # 获取MIME类型
        temp_opds = OPDSGenerator(base_url=request.url_root.rstrip('/'))
        mime_type = temp_opds.get_mime_type(target_format['format'])
        
        # 使用数据库中的书名生成下载文件名
        book_title = book.get('title', '未知书籍')
        logger.debug(f"原始书名: {repr(book_title)}")
        
        format_extension = {
            'EPUB': '.epub', 'PDF': '.pdf', 'MOBI': '.mobi',
            'AZW3': '.azw3', 'FB2': '.fb2', 'RTF': '.rtf',
            'TXT': '.txt', 'HTML': '.html', 'LIT': '.lit'
        }.get(target_format['format'].upper(), '.epub')
        
        # 生成安全的中文文件名
        safe_title = safe_convert_text(book_title)
        logger.debug(f"转换后书名: {repr(safe_title)}")
        
        # 确保原始书名不为空
        if not safe_title or safe_title.strip() == "":
            logger.warning(f"书名为空，使用默认文件名")
            safe_filename = f"书籍_{book_id}{format_extension}"
        else:
            safe_filename = re.sub(r'[<>:"/\\|?*]', '', safe_title)
            safe_filename = safe_filename.replace(' ', '_')
            logger.debug(f"处理后文件名: {repr(safe_filename)}")
            
            if not safe_filename.lower().endswith(format_extension.lower()):
                safe_filename += format_extension
            
            # 双重检查：确保文件名不为空
            base_name = os.path.splitext(safe_filename)[0]
            if not base_name or base_name == format_extension or len(base_name.strip()) == 0:
                safe_filename = f"书籍_{book_id}{format_extension}"
                logger.debug(f"触发fallback机制，使用默认文件名: {safe_filename}")
        
        logger.debug(f"最终文件名: {repr(safe_filename)}")
        
        try:
            # 设置响应头 - 使用更安全的方式
            response = send_file(
                full_path,
                as_attachment=True,
                download_name=safe_filename,
                mimetype=mime_type,
                conditional=True,
                etag=True,
                last_modified=True,
                max_age=3600
            )
            
            # 添加缓存控制头
            response.headers['Cache-Control'] = 'public, max-age=3600'
            response.headers['X-Content-Type-Options'] = 'nosniff'
            
            logger.info(f"下载文件: {safe_filename}")
            return response
        except Exception as e:
            logger.error(f"发送文件失败: {e}")
            return jsonify({'error': f'文件发送失败: {str(e)}'}), 500
        
    except Exception as e:
        logger.error(f"下载书籍失败: {e}")
        return jsonify({'error': 'Internal server error'}), 500

# --- API 路由定义 ---
@app.route('/api/books')
def api_books():
    search = request.args.get('search')
    limit = int(request.args.get('limit', 20))
    offset = int(request.args.get('offset', 0))
    books = db.get_all_books(limit=limit, offset=offset, search=search)
    return jsonify({
        'books': books, 'total': len(books), 'limit': limit, 'offset': offset
    })

@app.route('/api/book/<int:book_id>')
def api_book_detail(book_id):
    book = db.get_book_detail(book_id)
    if not book: return jsonify({'error': 'Book not found'}), 404
    return jsonify(book)

@app.route('/api/stats')
def api_stats():
    """获取统计信息"""
    def _get_stats():
        conn = db.get_connection()
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) FROM books")
        total_books = cursor.fetchone()[0]
        cursor.execute("SELECT COUNT(*) FROM authors")
        total_authors = cursor.fetchone()[0]
        cursor.execute("SELECT format, COUNT(*) FROM data GROUP BY format")
        format_stats = dict(cursor.fetchall())
        return {'total_books': total_books, 'total_authors': total_authors, 'formats': format_stats}
    
    try:
        stats = db.execute_and_log_errors(_get_stats)
        return jsonify(stats)
    except Exception as e:
        logger.error(f"Error getting stats: {e}")
        return jsonify({'error': 'Internal server error'}), 500

@app.route('/api/health')
def api_health():
    """健康检查端点"""
    try:
        conn = db.get_connection()
        if not db.check_connection_health(conn):
            return jsonify({
                'status': 'unhealthy',
                'database': 'unhealthy',
                'connection_stats': db.get_connection_stats(),
                'timestamp': datetime.now(timezone.utc).isoformat()
            }), 503
        
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) FROM books")
        book_count = cursor.fetchone()[0]
        
        return jsonify({
            'status': 'healthy',
            'database': 'healthy',
            'book_count': book_count,
            'connection_stats': db.get_connection_stats(),
            'timestamp': datetime.now(timezone.utc).isoformat()
        })
        
    except Exception as e:
        logger.error(f"健康检查失败: {e}")
        return jsonify({
            'status': 'unhealthy',
            'database': 'error',
            'error': str(e),
            'connection_stats': db.get_connection_stats(),
            'timestamp': datetime.now(timezone.utc).isoformat()
        }), 503

@app.route('/api/connection-stats')
def api_connection_stats():
    """获取数据库连接统计信息"""
    return jsonify(db.get_connection_stats())

@app.route('/api/diagnose')
def api_diagnose():
    """诊断端点"""
    try:
        # 获取应用状态
        app_status = {
            'connection_stats': db.get_connection_stats(),
            'log_level': LOG_LEVEL,
            'books_path': db.base_books_path,
            'db_path': db.db_path,
            'server_time': datetime.now(timezone.utc).isoformat()
        }

        # 测试基本功能
        tests = {}
        
        # 测试数据库连接
        try:
            conn = db.get_connection()
            cursor = conn.cursor()
            cursor.execute("SELECT COUNT(*) FROM books")
            book_count = cursor.fetchone()[0]
            tests['database'] = {
                'status': 'ok',
                'book_count': book_count,
                'connection_valid': db.check_connection_health(conn)
            }
        except Exception as e:
            tests['database'] = {
                'status': 'error',
                'error': str(e)
            }

        # 测试文件系统访问
        try:
            if os.path.exists(db.base_books_path):
                books_dir_count = len([d for d in os.listdir(db.base_books_path) if os.path.isdir(os.path.join(db.base_books_path, d))])
                tests['filesystem'] = {
                    'status': 'ok',
                    'books_directory_accessible': True,
                    'author_directories': books_dir_count
                }
            else:
                tests['filesystem'] = {
                    'status': 'error',
                    'error': f'Books directory not found: {db.base_books_path}'
                }
        except Exception as e:
            tests['filesystem'] = {
                'status': 'error',
                'error': str(e)
            }

        # 测试样本书籍
        try:
            sample_books = db.get_all_books(limit=3)
            if sample_books:
                book = sample_books[0]
                tests['sample_book'] = {
                    'status': 'ok',
                    'book_id': book['id'],
                    'title': book['title'][:50],
                    'has_cover': book.get('has_cover', False),
                    'formats': [f['format'] for f in book.get('formats', [])]
                }
            else:
                tests['sample_book'] = {
                    'status': 'warning',
                    'message': 'No books found in database'
                }
        except Exception as e:
            tests['sample_book'] = {
                'status': 'error',
                'error': str(e)
            }

        return jsonify({
            'application': app_status,
            'tests': tests,
            'recommendations': _get_diagnostic_recommendations(tests, app_status)
        })

    except Exception as e:
        logger.error(f"诊断失败: {e}")
        return jsonify({'error': 'Internal server error'}), 500

def _get_diagnostic_recommendations(tests, app_status):
    """根据诊断结果提供建议"""
    recommendations = []
    
    # 数据库问题
    db_test = tests.get('database', {})
    if db_test.get('status') != 'ok':
        recommendations.append({
            'level': 'error',
            'issue': '数据库连接失败',
            'solution': '检查数据库文件路径和权限，确保metadata.db文件存在且可访问'
        })
    
    # 文件系统问题
    fs_test = tests.get('filesystem', {})
    if fs_test.get('status') != 'ok':
        recommendations.append({
            'level': 'error',
            'issue': '书籍目录无法访问',
            'solution': '检查CALIBRE_BOOKS_PATH环境变量，确保书籍目录存在且可读取'
        })
    
    if not recommendations:
        recommendations.append({
            'level': 'info',
            'issue': '系统运行正常',
            'solution': '无需特别处理，如文石阅读器仍有问题，请检查客户端配置'
        })
    
    return recommendations

# --- 错误处理与启动 ---
@app.errorhandler(404)
def not_found(error):
    return 'Not Found', 404

if __name__ == '__main__':
    logger.warning("在生产环境（容器）中，请使用 Gunicorn 或 uWSGI 运行此应用！")
    logger.info("启动Calibre OPDS服务 - 数据库修复版本...")
    host = os.environ.get('OPDS_HOST', '0.0.0.0')
    port = int(os.environ.get('OPDS_PORT', 1580))
    logger.info(f"OPDS目录: http://{host}:{port}/opds")
    logger.info(f"数据库路径: {db.db_path}")
    logger.info(f"书籍路径: {db.base_books_path}")
    app.run(host=host, port=port, debug=True)