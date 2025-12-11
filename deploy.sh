#!/bin/bash
# Calibre OPDS服务容器化部署脚本

# 端口配置说明
# 外部端口: 1580 (在docker-compose.yml中配置)
# 容器端口: 5000 (在opds_server.py中配置)
# 访问时请使用: http://您的服务器IP:1580

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_message() {
    echo -e "${2:-$NC}$1${NC}"
}

print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_success() {
    print_message "✅ $1" "$GREEN"
}

print_warning() {
    print_message "⚠️  $1" "$YELLOW"
}

print_error() {
    print_message "❌ $1" "$RED"
}

print_info() {
    print_message "ℹ️  $1" "$BLUE"
}

# 检查Docker是否安装
check_docker() {
    print_header "检查Docker环境"
    
    if ! command -v docker &> /dev/null; then
        print_error "Docker未安装，请先安装Docker"
        print_info "安装命令: curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh"
        exit 1
    fi
    print_success "Docker已安装: $(docker --version)"
    
    # 检查Docker Compose
    COMPOSE_CMD=""
    if command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
        print_success "Docker Compose已安装: $(docker-compose --version)"
    elif docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
        print_success "Docker Compose (plugin)已安装: $(docker compose version)"
    else
        print_warning "Docker Compose未安装"
        print_info "提供两种解决方案："
        print_info "1. 安装Docker Compose (推荐):"
        print_info "   curl -L \"https://github.com/docker/compose/releases/download/v2.20.2/docker-compose-\$(uname -s)-\$(uname -m)\" -o /usr/local/bin/docker-compose"
        print_info "   chmod +x /usr/local/bin/docker-compose"
        print_info "2. 或者手动运行Docker命令"
        echo
        read -p "选择部署方式 (1=安装Compose 2=手动Docker 3=跳过): " choice
        case $choice in
            1)
                print_message "正在安装Docker Compose..."
                curl -L "https://github.com/docker/compose/releases/download/v2.20.2/docker-compose-\$(uname -s)-\$(uname -m)" -o /usr/local/bin/docker-compose
                chmod +x /usr/local/bin/docker-compose
                COMPOSE_CMD="docker-compose"
                ;;
            2)
                COMPOSE_CMD="manual"
                print_success "将使用手动Docker命令"
                ;;
            3)
                exit 0
                ;;
            *)
                print_error "无效选择，退出"
                exit 1
                ;;
        esac
    fi
    
    # 设置全局变量
    export COMPOSE_CMD
    print_success "Docker环境检查通过"
}

# 检查必要的文件
check_files() {
    print_header "检查部署文件"
    
    required_files=("Dockerfile" "docker-compose.yml" "requirements.txt" "opds_server.py")
    missing_files=()
    
    for file in "${required_files[@]}"; do
        if [ ! -f "$file" ]; then
            missing_files+=("$file")
        fi
    done
    
    if [ ${#missing_files[@]} -ne 0 ]; then
        print_error "缺少必要的部署文件：${missing_files[*]}"
        exit 1
    fi
    
    print_success "所有必要文件检查通过"
}

# 创建示例目录结构
create_sample_structure() {
    print_header "创建示例目录结构"
    
    if [ ! -d "calibre-library" ]; then
        mkdir -p calibre-library
        print_warning "已创建示例目录 calibre-library/"
        print_warning "请将您的Calibre库内容复制到此目录，或者修改docker-compose.yml中的卷挂载路径"
    else
        print_message "calibre-library 目录已存在"
    fi
}

# 构建和启动容器 (支持Docker Compose和手动Docker)
deploy() {
    print_header "构建和部署OPDS服务"
    
    if [ "$COMPOSE_CMD" = "manual" ]; then
        # 手动Docker部署
        print_message "使用手动Docker命令部署..."
        
        # 构建镜像
        print_message "构建Docker镜像..."
        docker build -t calibre-opds:latest .
        
        # 运行容器
        print_message "启动容器..."
        # 检查是否已存在容器
        if docker ps -a --format 'table {{.Names}}' | grep -q "^calibre-opds-server$"; then
            print_message "删除现有容器..."
            docker rm -f calibre-opds-server
        fi
        
        # 运行新容器
        docker run -d \
            --name calibre-opds-server \
            -p 1580:5000 \
            -v "$(pwd)/calibre-library:/books:ro" \
            --restart unless-stopped \
            calibre-opds:latest
        
        print_success "OPDS服务已启动！"
    else
        # 使用Docker Compose部署
        print_message "使用Docker Compose部署..."
        
        # 构建镜像
        print_message "构建Docker镜像..."
        $COMPOSE_CMD build
        
        # 启动服务
        print_message "启动服务..."
        $COMPOSE_CMD up -d
        
        print_success "OPDS服务已启动！"
    fi
}

# 检查服务状态
check_status() {
    print_header "检查服务状态"
    
    if [ "$COMPOSE_CMD" = "manual" ]; then
        # 检查手动Docker容器状态
        if docker ps --format 'table {{.Names}}' | grep -q "^calibre-opds-server$"; then
            print_success "容器运行正常"
            
            # 测试API
            print_message "测试API连通性..."
            sleep 5
            if curl -s http://localhost:1580/api/stats > /dev/null; then
                print_success "API测试成功"
                
                # 显示统计信息
                echo
                print_message "📊 服务统计信息:" "$BLUE"
                curl -s http://localhost:1580/api/stats | python3 -m json.tool 2>/dev/null || echo "API返回响应"
            else
                print_warning "API测试失败，请检查日志"
                echo "查看日志: docker logs -f calibre-opds-server"
            fi
        else
            print_error "容器未运行"
            echo "查看所有容器: docker ps -a"
        fi
    else
        # 检查Docker Compose状态
        if $COMPOSE_CMD ps | grep -q "Up"; then
            print_success "服务运行正常"
            
            # 测试API
            print_message "测试API连通性..."
            sleep 5
            if curl -s http://localhost:1580/api/stats > /dev/null; then
                print_success "API测试成功"
                
                # 显示统计信息
                echo
                print_message "📊 服务统计信息:" "$BLUE"
                curl -s http://localhost:1580/api/stats | python3 -m json.tool 2>/dev/null || echo "API返回响应"
            else
                print_warning "API测试失败，请检查日志"
                echo "查看日志: $COMPOSE_CMD logs -f"
            fi
        else
            print_error "服务启动失败，请检查日志"
            echo "查看日志: $COMPOSE_CMD logs"
        fi
    fi
}

# 显示使用信息
show_usage() {
    print_header "Calibre OPDS服务已启动"
    
    echo -e "${GREEN}访问信息：${NC}"
    echo -e "  📚 OPDS目录: ${BLUE}http://localhost:1580/opds${NC}"
    echo -e "  🔧 API接口: ${BLUE}http://localhost:1580/api${NC}"
    echo -e "  📖 API文档: ${BLUE}http://localhost:1580/api/stats${NC}"
    echo
    echo -e "${YELLOW}常用管理命令：${NC}"
    if [ "$COMPOSE_CMD" = "manual" ]; then
        echo -e "  查看日志: ${BLUE}docker logs -f calibre-opds-server${NC}"
        echo -e "  停止服务: ${BLUE}docker stop calibre-opds-server${NC}"
        echo -e "  重启服务: ${BLUE}docker restart calibre-opds-server${NC}"
        echo -e "  查看状态: ${BLUE}docker ps${NC}"
        echo -e "  重新构建: ${BLUE}docker build -t calibre-opds:latest .${NC}"
    else
        echo -e "  查看日志: ${BLUE}$COMPOSE_CMD logs -f${NC}"
        echo -e "  停止服务: ${BLUE}$COMPOSE_CMD down${NC}"
        echo -e "  重启服务: ${BLUE}$COMPOSE_CMD restart${NC}"
        echo -e "  查看状态: ${BLUE}$COMPOSE_CMD ps${NC}"
        echo -e "  重新构建: ${BLUE}$COMPOSE_CMD build${NC}"
    fi
    echo
    echo -e "${YELLOW}在阅读器中配置OPDS：${NC}"
    echo -e "  OPDS URL: ${BLUE}http://您的服务器IP:1580/opds${NC}"
    echo
    echo -e "${YELLOW}注意事项：${NC}"
    echo -e "  • 确保已将Calibre库内容复制到 calibre-library 目录"
    echo -e "  • 或者修改 docker-compose.yml 中的卷挂载路径"
    echo -e "  • 外部端口使用1580，容器内部端口5000"
    echo
}

# 清理函数
cleanup() {
    print_header "清理资源"
    
    read -p "是否要停止并删除容器？(y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        if [ "$COMPOSE_CMD" = "manual" ]; then
            docker stop calibre-opds-server
            docker rm calibre-opds-server
        else
            $COMPOSE_CMD down
        fi
        print_success "资源已清理"
    else
        print_message "保留容器运行"
    fi
}

# 主函数
main() {
    case "${1:-deploy}" in
        "deploy")
            check_docker
            check_files
            create_sample_structure
            deploy
            check_status
            show_usage
            ;;
        "stop")
            if [ "$COMPOSE_CMD" = "manual" ]; then
                docker stop calibre-opds-server
            else
                $COMPOSE_CMD down
            fi
            print_success "服务已停止"
            ;;
        "restart")
            if [ "$COMPOSE_CMD" = "manual" ]; then
                docker restart calibre-opds-server
            else
                $COMPOSE_CMD restart
            fi
            print_success "服务已重启"
            ;;
        "build")
            if [ "$COMPOSE_CMD" = "manual" ]; then
                docker build -t calibre-opds:latest .
            else
                $COMPOSE_CMD build
            fi
            print_success "镜像构建完成"
            ;;
        "status")
            check_status
            ;;
        "logs")
            if [ "$COMPOSE_CMD" = "manual" ]; then
                docker logs -f calibre-opds-server
            else
                $COMPOSE_CMD logs -f
            fi
            ;;
        "cleanup")
            cleanup
            ;;
        "help"|"-h"|"--help")
            echo "使用方法: $0 [deploy|stop|restart|build|status|logs|cleanup|help]"
            echo
            echo "命令说明:"
            echo "  deploy   - 完整部署OPDS服务 (默认)"
            echo "  stop     - 停止服务"
            echo "  restart  - 重启服务"
            echo "  build    - 构建镜像"
            echo "  status   - 检查服务状态"
            echo "  logs     - 查看服务日志"
            echo "  cleanup  - 清理容器和资源"
            echo "  help     - 显示此帮助信息"
            ;;
        *)
            print_error "未知命令: $1"
            echo "使用 '$0 help' 查看可用命令"
            exit 1
            ;;
    esac
}

# 捕获Ctrl+C并清理
trap cleanup INT

# 运行主函数
main "$@"
