#!/bin/bash

# ============================================================================
# YggLeaf Docker 构建脚本 (Frontleaves - Phalanx Labs)
# 使用 gum 美化输出
# ============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

USE_GUM=false
if command -v gum &> /dev/null; then
    USE_GUM=true
fi

log_info() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 39 "$(gum style --bold '[INFO]') $1"
    else
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 82 "$(gum style --bold '[SUCCESS]') $1"
    else
        echo -e "${GREEN}[SUCCESS]${NC} $1"
    fi
}

log_warn() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 214 "$(gum style --bold '[WARN]') $1"
    else
        echo -e "${YELLOW}[WARN]${NC} $1"
    fi
}

log_error() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 196 "$(gum style --bold '[ERROR]') $1"
    else
        echo -e "${RED}[ERROR]${NC} $1"
    fi
}

print_separator() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 240 "────────────────────────────────────────────────────────────"
    else
        echo "────────────────────────────────────────────────────────────"
    fi
}

print_step() {
    local step_num=$1
    local step_title=$2
    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 39 --bold "[$step_num] $step_title"
    else
        echo -e "${BLUE}[$step_num] $step_title${NC}"
    fi
    print_separator
}

print_banner() {
    if [ "$USE_GUM" = true ]; then
        gum style \
            --foreground 57 --bold \
            "  __  __          _   _ _____ ____  " \
            " |  \/  |   /\   | \ | |_   _/ __ \ " \
            " | |\/| |  /  \  |  \| | | || |  | |" \
            " | |  | | / /\ \ | . ` | | || |  | |" \
            " |_|  |_|/_/  \_\_|\_\_| |_||_|  |_|" \
            "" \
            "       Yggdrasil Leaf Service         " \
            "" \
            "        Docker Build System v1.0"
    else
        echo -e "${BLUE}  __  __          _   _ _____ ____  ${NC}"
        echo -e "${BLUE} |  \/  |   /\\   | \\ | |_   _/ __ \\ ${NC}"
        echo -e "${BLUE} | |\\/| |  /  \\  |  \\| | | || |  | |${NC}"
        echo -e "${BLUE} | |  | | / /\\ \\ | . \\` | | || |  | |${NC}"
        echo -e "${BLUE} |_|  |_|/_/  \\_\\_|\\_\\_| |_||_|  |_|${NC}"
        echo ""
        echo -e "${BLUE}     Yggdrasil Leaf Service${NC}"
        echo ""
        echo -e "${BLUE}       Docker Build System v1.0${NC}"
    fi
}

if [ -z "$1" ] || [ -z "$2" ]; then
    print_banner
    echo ""
    log_error "缺少必要参数"
    echo ""
    echo "使用方法: ./build-docker.sh <username> <password> [version]"
    echo ""
    echo "参数说明:"
    echo "  username  - 腾讯云容器镜像服务用户名"
    echo "  password  - 腾讯云容器镜像服务密码"
    echo "  version   - 版本号 (可选，不指定则自动递增)"
    echo ""
    echo "示例:"
    echo "  ./build-docker.sh 100032613538 yourpassword"
    echo "  ./build-docker.sh 100032613538 yourpassword 1.0.0"
    echo ""
    exit 1
fi

USERNAME=$1
PASSWORD=$2
SPECIFIED_VERSION=$3

REGISTRY="ccr.ccs.tencentyun.com"
NAMESPACE="frontleaves"
IMAGE_NAME="frontleaves-yggleaf"
TARGET_PLATFORM="linux/amd64"

print_banner
echo ""

# ============================================================================
# STEP 1: 确定版本号
# ============================================================================
print_step "1/5" "确定版本号"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

if [ -n "$SPECIFIED_VERSION" ]; then
    VERSION=$SPECIFIED_VERSION
    log_info "使用指定版本: $(gum style --bold --foreground 214 "$VERSION" 2>/dev/null || echo "$VERSION")"
else
    log_info "未指定版本，尝试从远程获取最新版本..."

    LATEST_VERSION=$(curl -s "https://$REGISTRY/v2/$NAMESPACE/$IMAGE_NAME/tags/list" 2>/dev/null | \
        python3 -c "import sys, json; tags=json.load(sys.stdin).get('tags', []);
        version_tags=[t for t in tags if t!='latest' and t.replace('.','').isdigit()];
        print(max(version_tags)) if version_tags else print('')" 2>/dev/null || echo "")

    if [ -z "$LATEST_VERSION" ]; then
        VERSION="1.0.0"
        log_warn "未找到远程版本，使用初始版本: $VERSION"
    else
        MAJOR=$(echo "$LATEST_VERSION" | cut -d. -f1)
        MINOR=$(echo "$LATEST_VERSION" | cut -d. -f2)
        PATCH=$(echo "$LATEST_VERSION" | cut -d. -f3)
        PATCH=$((PATCH + 1))
        VERSION="$MAJOR.$MINOR.$PATCH"

        log_success "最新版本: $LATEST_VERSION → 新版本: $(gum style --bold --foreground 214 "$VERSION" 2>/dev/null || echo "$VERSION")"
    fi
fi

echo ""

# ============================================================================
# STEP 2: 环境检查
# ============================================================================
print_step "2/5" "环境检查"

if ! command -v docker &> /dev/null; then
    log_error "Docker 未安装，请先安装 Docker"
    exit 1
fi
log_success "Docker 已安装"

if [ ! -f "Dockerfile" ]; then
    log_error "Dockerfile 不存在于项目根目录"
    exit 1
fi
log_success "Dockerfile 已找到"

if [ ! -f "go.mod" ]; then
    log_error "go.mod 不存在，请确保在项目根目录运行"
    exit 1
fi
log_success "go.mod 已找到"

OS=$(uname -s)
ARCH=$(uname -m)
log_info "检测到系统: $OS $ARCH"

echo ""

# ============================================================================
# STEP 3: 更新 Swagger 文档
# ============================================================================
print_step "3/5" "更新 Swagger 文档"

if command -v swag &> /dev/null; then
    log_info "正在生成 Swagger 文档..."
    if swag init --instanceName frontleaves_yggleaf -g main.go --parseDependency 2>/dev/null; then
        log_success "Swagger 文档生成成功"
    else
        log_warn "Swagger 文档生成失败，继续构建..."
    fi
else
    log_warn "swag 未安装，跳过 Swagger 文档生成"
    log_info "提示: 运行 'go install github.com/swaggo/swag/cmd/swag@latest' 安装 swag"
fi

echo ""

# ============================================================================
# STEP 4: Docker 登录
# ============================================================================
print_step "4/5" "Docker 仓库登录"

log_info "正在登录腾讯云容器镜像服务..."
log_info "仓库地址: $REGISTRY"
log_info "用户名: $USERNAME"

if echo "$PASSWORD" | docker login "$REGISTRY" --username="$USERNAME" --password-stdin > /dev/null 2>&1; then
    log_success "Docker 登录成功"
else
    log_error "Docker 登录失败，请检查用户名和密码"
    log_info "提示: 请访问 https://console.cloud.tencent.com/tke2/registry/user 获取登录信息"
    exit 1
fi

echo ""

# ============================================================================
# STEP 5: 构建并推送 Docker 镜像
# ============================================================================
print_step "5/5" "构建并推送 Docker 镜像"

FULL_IMAGE_NAME="$REGISTRY/$NAMESPACE/$IMAGE_NAME"
VERSION_TAG="$FULL_IMAGE_NAME:$VERSION"
LATEST_TAG="$FULL_IMAGE_NAME:latest"

echo ""
if [ "$USE_GUM" = true ]; then
    gum style \
        --border normal --border-foreground 57 --padding "0 2" \
        "$(gum style --bold --foreground 57 "构建配置")" \
        "" \
        "$(gum style --foreground 245 "镜像名称: $IMAGE_NAME")" \
        "$(gum style --foreground 245 "完整路径: $FULL_IMAGE_NAME")" \
        "$(gum style --foreground 245 "版本标签: $VERSION")" \
        "$(gum style --foreground 245 "目标平台: $TARGET_PLATFORM")"
else
    echo "┌────────────────────────────────────────┐"
    echo "│ 构建配置                               │"
    echo "├────────────────────────────────────────┤"
    echo "│ 镜像名称: $IMAGE_NAME"
    echo "│ 完整路径: $FULL_IMAGE_NAME"
    echo "│ 版本标签: $VERSION"
    echo "│ 目标平台: $TARGET_PLATFORM"
    echo "└────────────────────────────────────────┘"
fi
echo ""

build_image() {
    local build_cmd="docker buildx build \
        --platform $TARGET_PLATFORM \
        -f Dockerfile \
        -t '$VERSION_TAG' \
        -t '$LATEST_TAG' \
        --push ."

    log_info "开始构建 Docker 镜像..."
    echo ""

    if eval "$build_cmd"; then
        return 0
    else
        return 1
    fi
}

if build_image; then
    echo ""
    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style \
            --border double --border-foreground 82 \
            --padding "1 3" --margin "1 0" \
            "$(gum style --bold --foreground 82 '🎉 构建成功')" \
            "" \
            "$(gum style --foreground 82 "镜像已成功推送到腾讯云容器镜像服务")" \
            "" \
            "$(gum style --foreground 245 "版本标签:")" \
            "$(gum style --foreground 240 "  📦 $VERSION_TAG")" \
            "" \
            "$(gum style --foreground 245 "最新标签:")" \
            "$(gum style --foreground 240 "  🏷️  $LATEST_TAG")" \
            "" \
            "$(gum style --foreground 245 "拉取命令:")" \
            "$(gum style --foreground 240 "  docker pull $VERSION_TAG")"
    else
        echo "╔════════════════════════════════════════╗"
        echo "║         🎉 构建成功                    ║"
        echo "╠════════════════════════════════════════╣"
        echo "║ 镜像已成功推送到腾讯云容器镜像服务    ║"
        echo "║                                        ║"
        echo "║ 版本标签:                              ║"
        echo "║   📦 $VERSION_TAG"
        echo "║                                        ║"
        echo "║ 最新标签:                              ║"
        echo "║   🏷️  $LATEST_TAG"
        echo "║                                        ║"
        echo "║ 拉取命令:                              ║"
        echo "║   docker pull $VERSION_TAG"
        echo "╚════════════════════════════════════════╝"
    fi
    echo ""
    log_success "所有操作已完成！"
    exit 0
else
    echo ""
    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style \
            --border double --border-foreground 196 \
            --padding "1 3" --margin "1 0" \
            "$(gum style --bold --foreground 196 '❌ 构建失败')" \
            "" \
            "$(gum style --foreground 196 "Docker 镜像构建失败，请检查错误信息")"
    else
        echo "╔════════════════════════════════════════╗"
        echo "║         ❌ 构建失败                    ║"
        echo "╠════════════════════════════════════════╣"
        echo "║ Docker 镜像构建失败，请检查错误信息   ║"
        echo "╚════════════════════════════════════════╝"
    fi
    echo ""
    log_error "构建失败，请排查错误后重试"
    exit 1
fi
