# 变量定义，方便后续维护
MAIN_FILE = main.go
SWAG_CMD = swag
SWAG_FLAGS = --parseDependency
BUILD_SCRIPT = script/build-docker.sh
SCRIPT_DIR = script

.DEFAULT_GOAL := help

.PHONY: help swag run dev tidy docker docker-build upload

# 显示帮助信息
help:
	@echo "锋楪YggLeaf - 可用命令"
	@echo ""
	@echo "开发命令:"
	@echo "  make swag       - 生成 Swagger 文档"
	@echo "  make run        - 运行程序"
	@echo "  make dev        - 生成文档并运行 (推荐)"
	@echo "  make tidy       - 整理依赖"
	@echo ""
	@echo "Docker 构建:"
	@echo "  make docker USER=<user> PASS=<pass> [VERSION=<ver>] - 构建 Docker 镜像"
	@echo "  make docker-build                                   - 使用环境变量构建"
	@echo ""
	@echo "服务器部署:"
	@echo "  make upload DEPLOY_SERVER=<server> [DEPLOY_USER=<user>] [DEPLOY_PATH=<path>] - 上传到服务器"
	@echo ""
	@echo "示例:"
	@echo "  make dev"
	@echo "  make docker USER=100032613538 PASS=password"
	@echo ""

# 提取出的 Swagger 生成目标
swag:
	$(SWAG_CMD) init --instanceName frontleaves_yggleaf -g $(MAIN_FILE) --parseDependency

# 提取出的运行目标
run:
	go run $(MAIN_FILE)

tidy:
	go mod tidy

# 组合目标：先生成文档，再运行程序
dev: swag run

# Docker 构建并推送到腾讯云容器镜像服务
docker:
	@if [ -z "$(USER)" ] || [ -z "$(PASS)" ]; then \
		echo "错误: 缺少必要参数"; \
		echo ""; \
		echo "使用方法: make docker USER=<username> PASS=<password> [VERSION=<version>]"; \
		echo ""; \
		echo "示例:"; \
		echo "  make docker USER=100032613538 PASS=yourpassword"; \
		echo "  make docker USER=100032613538 PASS=yourpassword VERSION=1.0.0"; \
		exit 1; \
	fi
	$(BUILD_SCRIPT) $(USER) $(PASS) $(VERSION)

# 快捷命令：使用默认配置构建（需先设置环境变量）
docker-build:
	@if [ -z "$$DOCKER_FRONTLEAVES_USER" ] || [ -z "$$DOCKER_FRONTLEAVES_PASS" ]; then \
		echo "错误: 请先设置环境变量 DOCKER_FRONTLEAVES_USER 和 DOCKER_FRONTLEAVES_PASS"; \
		echo ""; \
		echo "export DOCKER_FRONTLEAVES_USER=100032613538"; \
		echo "export DOCKER_FRONTLEAVES_PASS=yourpassword"; \
		exit 1; \
	fi
	$(BUILD_SCRIPT) $$DOCKER_FRONTLEAVES_USER $$DOCKER_FRONTLEAVES_PASS $(VERSION)

# 上传到服务器
upload:
	@echo "🚀 正在上传到服务器$(if $(DEPLOY_SERVER), $(DEPLOY_SERVER), (默认配置))..."
	@$(SCRIPT_DIR)/upload-to-server.sh "$(DEPLOY_SERVER)" "$(DEPLOY_USER)" "$(DEPLOY_PATH)"
