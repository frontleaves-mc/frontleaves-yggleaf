# ============================================================
# 基础变量
# ============================================================

MAIN_FILE = main.go
SWAG_CMD = swag
SWAG_FLAGS = --parseDependency
BUILD_SCRIPT = script/build-docker.sh
SCRIPT_DIR = script

# 根版本号（去除 v 前缀，如 v1 → 1）
ROOT_VERSION := $(shell cat version | sed 's/^v//')

# 时间戳（格式：YYYYMMDDHHMM）
TIMESTAMP := $(shell date +"%Y%m%d%H%M")

# Proto 子模块
PROTO_MODULE := proto

.DEFAULT_GOAL := help

.PHONY: help swag run dev tidy docker docker-build upload proto proto-init release-proto vet

# ============================================================
# 帮助信息
# ============================================================

help:
	@echo "锋楪YggLeaf - 可用命令"
	@echo ""
	@echo "开发命令:"
	@echo "  make swag                     - 生成 Swagger 文档"
	@echo "  make run                      - 运行程序"
	@echo "  make dev                      - 生成文档并运行 (推荐)"
	@echo "  make tidy                     - 整理依赖"
	@echo "  make proto-init               - 初始化 proto 符号链接"
	@echo "  make proto                    - 生成 protobuf Go 代码"
	@echo ""
	@echo "Docker 构建:"
	@echo "  make docker USER=<user> PASS=<pass> [VERSION=<ver>] - 构建 Docker 镜像"
	@echo "  make docker-build                                   - 使用环境变量构建"
	@echo ""
	@echo "服务器部署:"
	@echo "  make upload DEPLOY_SERVER=<server> [DEPLOY_USER=<user>] [DEPLOY_PATH=<path>] - 上传到服务器"
	@echo ""
	@echo "发布命令:"
	@echo "  make release-proto            - 发布 proto 子模块"
	@echo ""
	@echo "版本格式: v{ROOT_VERSION}.{SUB_VERSION}-{TIMESTAMP}"
	@echo "  根版本号:   $(ROOT_VERSION)  (来自 ./version)"
	@echo "  时间戳:     $(TIMESTAMP)"
	@echo ""
	@echo "示例:"
	@echo "  make dev"
	@echo "  make docker USER=100032613538 PASS=password"
	@echo "  make release-proto            → proto/v$(ROOT_VERSION).x.x-$(TIMESTAMP)"

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

# Proto 符号链接初始化
BASE_GO_MODULE_DIR := $(shell go list -m -f '{{.Dir}}' github.com/bamboo-services/bamboo-base-go/plugins/grpc 2>/dev/null)
XBASE_LINK := proto/link/base.proto

proto-init:
	@mkdir -p $(dir $(XBASE_LINK))
	@if [ -z "$(BASE_GO_MODULE_DIR)" ]; then \
		echo "错误: 找不到 bamboo-base-go gRPC 模块，请先运行 go mod download"; \
		exit 1; \
	fi
	@ln -sf $(BASE_GO_MODULE_DIR)/proto/base.proto $(XBASE_LINK)
	@echo "符号链接已创建: $(XBASE_LINK) -> $(BASE_GO_MODULE_DIR)/proto/base.proto"

# 生成 protobuf Go 代码
proto:
	@cd proto && buf generate
	@rm -f proto/link/base.pb.go
	@echo "protobuf 代码生成完成"

# ============================================================
# 发布命令
# ============================================================

# 构建 tag 名称的函数
# $(1): 模块目录路径 (如 proto)
# 返回: <path>/v<ROOT_VERSION>.<SUB_VERSION>-<TIMESTAMP>
define build_tag
$(strip $(1))/v$(ROOT_VERSION).$(shell cat $(1)/version)-$(TIMESTAMP)
endef

# --- make release-proto ---
# 发布 proto 子模块
release-proto:
	@$(eval TAG := $(call build_tag,$(PROTO_MODULE)))
	@echo "📦 发布模块: $(PROTO_MODULE)"
	@echo "   tag: $(TAG)"
	@git tag -a "$(TAG)" -m "Release $(TAG)"
	@git push origin "$(TAG)"
	@echo "✅ $(PROTO_MODULE) 发布完成: $(TAG)"
