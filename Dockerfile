# ========================
# Stage 1: Build (用于编译依赖项)
# ========================
FROM python:3.9-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖和 Python 包的编译工具
RUN apk update && \
    apk add --no-cache gcc musl-dev

# 复制 requirements 文件并安装 Python 依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# ========================
# Stage 2: Final (用于运行，只保留必要内容)
# ========================
FROM python:3.9-alpine

# 设置工作目录
WORKDIR /app

# 设置环境变量
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# 安装运行时依赖（包含sqlite3支持）
RUN apk add --no-cache sqlite-libs

# 复制编译好的Python包和可执行文件
COPY --from=builder /usr/local/lib/python3.9/site-packages /usr/local/lib/python3.9/site-packages
COPY --from=builder /usr/local/bin/gunicorn /usr/local/bin/gunicorn
# 复制所有Python源文件
COPY opds_server.py .
COPY encoding_utils.py .

# 创建书籍目录
RUN mkdir -p /books

# 暴露端口
EXPOSE 5000

# 使用Gunicorn启动应用
CMD ["gunicorn", "--bind", "0.0.0.0:5000", "--workers", "2", "opds_server:app"]