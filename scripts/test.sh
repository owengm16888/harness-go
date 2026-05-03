#!/bin/bash

# Harness Engineering 测试脚本

set -e

echo "=== Harness Engineering 测试 ==="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 运行单元测试
run_unit_tests() {
    echo -e "${YELLOW}运行单元测试...${NC}"
    go test ./internal/... -v -coverprofile=coverage.out
    echo -e "${GREEN}单元测试完成${NC}"
}

# 运行集成测试
run_integration_tests() {
    echo -e "${YELLOW}运行集成测试...${NC}"
    go test ./internal/... -v -tags=integration
    echo -e "${GREEN}集成测试完成${NC}"
}

# 运行 API 测试
run_api_tests() {
    echo -e "${YELLOW}运行 API 测试...${NC}"
    go test ./internal/api/... -v
    echo -e "${GREEN}API 测试完成${NC}"
}

# 运行存储测试
run_storage_tests() {
    echo -e "${YELLOW}运行存储测试...${NC}"
    go test ./internal/storage/... -v
    echo -e "${GREEN}存储测试完成${NC}"
}

# 运行适配器测试
run_adapter_tests() {
    echo -e "${YELLOW}运行适配器测试...${NC}"
    go test ./internal/adapters/... -v
    echo -e "${GREEN}适配器测试完成${NC}"
}

# 生成覆盖率报告
generate_coverage() {
    echo -e "${YELLOW}生成覆盖率报告...${NC}"
    go tool cover -html=coverage.out -o coverage.html
    echo -e "${GREEN}覆盖率报告已生成: coverage.html${NC}"
}

# 运行代码检查
run_lint() {
    echo -e "${YELLOW}运行代码检查...${NC}"
    
    # 检查 go vet
    echo "运行 go vet..."
    go vet ./...
    
    # 检查 golint (如果安装)
    if command -v golint &> /dev/null; then
        echo "运行 golint..."
        golint ./...
    fi
    
    # 检查 golangci-lint (如果安装)
    if command -v golangci-lint &> /dev/null; then
        echo "运行 golangci-lint..."
        golangci-lint run
    fi
    
    echo -e "${GREEN}代码检查完成${NC}"
}

# 运行基准测试
run_benchmarks() {
    echo -e "${YELLOW}运行基准测试...${NC}"
    go test ./internal/... -bench=. -benchmem
    echo -e "${GREEN}基准测试完成${NC}"
}

# 清理测试文件
cleanup() {
    echo -e "${YELLOW}清理测试文件...${NC}"
    rm -f coverage.out coverage.html
    rm -rf /tmp/harness-test-*
    echo -e "${GREEN}清理完成${NC}"
}

# 显示帮助
show_help() {
    echo "用法: ./test.sh [命令]"
    echo ""
    echo "命令:"
    echo "  unit        运行单元测试"
    echo "  integration 运行集成测试"
    echo "  api         运行 API 测试"
    echo "  storage     运行存储测试"
    echo "  adapters    运行适配器测试"
    echo "  coverage    生成覆盖率报告"
    echo "  lint        运行代码检查"
    echo "  bench       运行基准测试"
    echo "  clean       清理测试文件"
    echo "  all         运行所有测试"
    echo "  help        显示帮助"
}

# 主函数
main() {
    case "${1:-all}" in
        unit)
            run_unit_tests
            ;;
        integration)
            run_integration_tests
            ;;
        api)
            run_api_tests
            ;;
        storage)
            run_storage_tests
            ;;
        adapters)
            run_adapter_tests
            ;;
        coverage)
            run_unit_tests
            generate_coverage
            ;;
        lint)
            run_lint
            ;;
        bench)
            run_benchmarks
            ;;
        clean)
            cleanup
            ;;
        all)
            run_unit_tests
            run_api_tests
            run_storage_tests
            run_adapter_tests
            run_lint
            generate_coverage
            echo ""
            echo -e "${GREEN}=== 所有测试完成 ===${NC}"
            ;;
        help|*)
            show_help
            ;;
    esac
}

main "$@"
