# 多阶段构建 - 优化的Go Docker镜像
FROM golang:1.21-alpine AS builder

# 设置Go代理（中国区加速）
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

# 安装必要的构建工具
RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /build

# 复制go mod文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译应用（启用CGO以支持sqlite3）
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o opds-server ./cmd/server

# 最终镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

# 从builder复制编译好的二进制文件
COPY --from=builder /build/opds-server .

# 创建书籍目录
RUN mkdir -p /books

# 设置环境变量
ENV CALIBRE_DB_PATH=/books/metadata.db
ENV CALIBRE_BOOKS_PATH=/books
ENV OPDS_HOST=0.0.0.0
ENV OPDS_PORT=1580
ENV LOG_LEVEL=INFO
ENV ENVIRONMENT=production

# 暴露端口
EXPOSE 1580

# 运行应用
CMD ["./opds-server"]
