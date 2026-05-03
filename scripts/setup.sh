#!/bin/bash

# Harness Engineering 开发环境设置脚本

set -e

echo "=== Harness Engineering 开发环境设置 ==="

# 检查 Go 版本
check_go() {
    if ! command -v go &> /dev/null; then
        echo "错误: 未安装 Go"
        echo "请安装 Go 1.21 或更高版本: https://golang.org/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo "Go 版本: $GO_VERSION"
}

# 检查依赖
check_deps() {
    echo "检查依赖..."
    
    # 检查 SQLite
    if ! command -v sqlite3 &> /dev/null; then
        echo "警告: 未安装 SQLite3，将使用内存存储"
    fi
    
    # 检查 Docker
    if command -v docker &> /dev/null; then
        echo "Docker 已安装"
    else
        echo "警告: 未安装 Docker，将无法使用容器化部署"
    fi
}

# 安装 Go 依赖
install_deps() {
    echo "安装 Go 依赖..."
    go mod download
    go mod tidy
}

# 创建必要的目录
create_dirs() {
    echo "创建必要的目录..."
    mkdir -p data
    mkdir -p logs
    mkdir -p build
}

# 运行测试
run_tests() {
    echo "运行测试..."
    go test ./... -v
}

# 构建项目
build() {
    echo "构建项目..."
    go build -o build/harness ./cmd/harness
    go build -o build/harness-cli ./cmd/harness-cli
}

# 运行项目
run() {
    echo "运行项目..."
    go run ./cmd/harness
}

# 显示帮助
show_help() {
    echo "用法: ./setup.sh [命令]"
    echo ""
    echo "命令:"
    echo "  check      检查环境"
    echo "  install    安装依赖"
    echo "  test       运行测试"
    echo "  build      构建项目"
    echo "  run        运行项目"
    echo "  all        执行所有步骤"
    echo "  help       显示帮助"
}

# 主函数
main() {
    case "${1:-all}" in
        check)
            check_go
            check_deps
            ;;
        install)
            check_go
            install_deps
            create_dirs
            ;;
        test)
            check_go
            run_tests
            ;;
        build)
            check_go
            build
            ;;
        run)
            check_go
            run
            ;;
        all)
            check_go
            check_deps
            install_deps
            create_dirs
            run_tests
            build
            echo ""
            echo "=== 设置完成 ==="
            echo "运行项目: ./build/harness"
            echo "运行 CLI: ./build/harness-cli --help"
            ;;
        help|*)
            show_help
            ;;
    esac
}

main "$@"
