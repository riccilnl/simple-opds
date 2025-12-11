# Calibre OPDS Server

[English](#english) | [中文](#中文)

---

## English

A high-performance OPDS (Open Publication Distribution System) server for Calibre libraries, available in both Python and Go implementations.

### 🚀 Features

- 📚 **OPDS 1.2 Compliant** - Full OPDS standard support
- 🔍 **Advanced Search** - Search by title, author, series, and tags
- 📖 **Multi-Format** - EPUB, PDF, MOBI, AZW3, and more
- 🌐 **RESTful API** - Complete REST API for integration
- 🖼️ **Cover Support** - Book cover image service
- 🔤 **Chinese Support** - GBK/Big5 encoding support
- 🐳 **Docker Ready** - Easy containerized deployment

### 📦 Two Implementations

#### Python Version
- **Mature & Stable** - Battle-tested implementation
- **Easy Setup** - Simple pip install
- **File**: `opds_server.py`

#### Go Version (Recommended)
- **High Performance** - 3x faster response time
- **Low Memory** - 73% less memory usage
- **Single Binary** - No runtime dependencies
- **Small Docker Image** - 85% smaller (15MB vs 100MB)
- **Directory**: `cmd/`, `internal/`, `pkg/`

### 🎯 Performance Comparison

| Metric | Python | Go | Improvement |
|--------|--------|-----|-------------|
| Startup Time | ~2s | ~0.1s | **20x faster** |
| Response Time | ~100ms | ~30ms | **3x faster** |
| Memory Usage | ~150MB | ~40MB | **73% less** |
| Docker Image | ~100MB | ~15MB | **85% smaller** |

### 🚀 Quick Start

#### Using Go (Recommended)

```bash
# Build
go build -o opds-server ./cmd/server

# Run
export CALIBRE_DB_PATH=books/metadata.db
export CALIBRE_BOOKS_PATH=books
./opds-server
```

#### Using Python

```bash
# Install dependencies
pip install -r requirements.txt

# Run
python opds_server.py
```

#### Using Docker

```bash
# Go version
docker-compose -f docker-compose.go.yml up -d

# Python version
docker-compose up -d
```

### 📖 API Endpoints

- `GET /opds` - OPDS catalog root
- `GET /opds/books` - Book list with search/filter
- `GET /opds/authors` - Browse by authors
- `GET /opds/series` - Browse by series
- `GET /opds/tags` - Browse by tags
- `GET /api/health` - Health check
- `GET /api/stats` - Statistics

### 📝 Configuration

Environment variables:

```bash
CALIBRE_DB_PATH=books/metadata.db    # Database path
CALIBRE_BOOKS_PATH=books             # Books directory
OPDS_HOST=0.0.0.0                    # Listen address
OPDS_PORT=1580                       # Listen port
LOG_LEVEL=INFO                       # Log level
```

### 📄 Documentation

- [Go Implementation Guide](README.go.md) - Detailed Go version documentation
- [Python Implementation](README.md) - Original Python documentation

### 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### 📄 License

MIT License

---

## 中文

为Calibre书库提供的高性能OPDS服务器，提供Python和Go两种实现。

### 🚀 特性

- 📚 **OPDS 1.2标准** - 完全符合OPDS规范
- 🔍 **高级搜索** - 支持书名、作者、系列、标签搜索
- 📖 **多格式支持** - EPUB、PDF、MOBI、AZW3等
- 🌐 **RESTful API** - 完整的REST API接口
- 🖼️ **封面支持** - 书籍封面图片服务
- 🔤 **中文支持** - 完美支持GBK/Big5编码
- 🐳 **Docker部署** - 支持容器化部署

### 📦 两种实现

#### Python版本
- **成熟稳定** - 经过充分测试
- **简单易用** - pip安装即可
- **文件**: `opds_server.py`

#### Go版本（推荐）
- **高性能** - 响应速度快3倍
- **低内存** - 内存占用减少73%
- **单一二进制** - 无需运行时环境
- **小镜像** - Docker镜像减小85%（15MB vs 100MB）
- **目录**: `cmd/`, `internal/`, `pkg/`

### 🎯 性能对比

| 指标 | Python | Go | 提升 |
|------|--------|-----|------|
| 启动时间 | ~2秒 | ~0.1秒 | **快20倍** |
| 响应时间 | ~100ms | ~30ms | **快3倍** |
| 内存占用 | ~150MB | ~40MB | **减少73%** |
| Docker镜像 | ~100MB | ~15MB | **减小85%** |

### 🚀 快速开始

#### 使用Go版本（推荐）

```bash
# 编译
go build -o opds-server ./cmd/server

# 运行
export CALIBRE_DB_PATH=books/metadata.db
export CALIBRE_BOOKS_PATH=books
./opds-server
```

#### 使用Python版本

```bash
# 安装依赖
pip install -r requirements.txt

# 运行
python opds_server.py
```

#### 使用Docker

```bash
# Go版本
docker-compose -f docker-compose.go.yml up -d

# Python版本
docker-compose up -d
```

### 📖 API端点

- `GET /opds` - OPDS目录根
- `GET /opds/books` - 书籍列表（支持搜索/过滤）
- `GET /opds/authors` - 按作者浏览
- `GET /opds/series` - 按系列浏览
- `GET /opds/tags` - 按标签浏览
- `GET /api/health` - 健康检查
- `GET /api/stats` - 统计信息

### 📝 配置

环境变量：

```bash
CALIBRE_DB_PATH=books/metadata.db    # 数据库路径
CALIBRE_BOOKS_PATH=books             # 书籍目录
OPDS_HOST=0.0.0.0                    # 监听地址
OPDS_PORT=1580                       # 监听端口
LOG_LEVEL=INFO                       # 日志级别
```

### 📄 文档

- [Go实现指南](README.go.md) - Go版本详细文档
- [Python实现](README.md) - Python版本文档

### 🤝 贡献

欢迎贡献代码！请随时提交Pull Request。

### 📄 许可证

MIT License