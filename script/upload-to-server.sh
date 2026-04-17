#!/bin/bash

# ============================================================================
# YggLeaf 服务器上传脚本
# Frontleaves - Phalanx Labs
#
# 用途: 使用 RSYNC 将必要文件上传到服务器
# 使用: ./script/upload-to-server.sh [server] [user] [path]
# ============================================================================

set -e

readonly BRAND_PRIMARY='#2d5a27'
readonly BRAND_SECONDARY='#5c8d89'
readonly BRAND_DARK='#1a1c18'
readonly FOREGROUND='#1f2623'
readonly MUTED='#64748b'
readonly DESTRUCTIVE='#ef4444'

USE_GUM=false
if command -v gum &> /dev/null; then
    USE_GUM=true
fi

log_info() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground "$BRAND_SECONDARY" "$(gum style --bold '[INFO]') $1"
    else
        echo -e "\033[0;36m[INFO]\033[0m $1"
    fi
}

log_success() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground "$BRAND_PRIMARY" "$(gum style --bold '[SUCCESS]') $1"
    else
        echo -e "\033[0;32m[SUCCESS]\033[0m $1"
    fi
}

log_warn() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 214 "$(gum style --bold '[WARN]') $1"
    else
        echo -e "\033[1;33m[WARN]\033[0m $1"
    fi
}

log_error() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground "$DESTRUCTIVE" "$(gum style --bold '[ERROR]') $1"
    else
        echo -e "\033[0;31m[ERROR]\033[0m $1"
    fi
}

print_separator() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground "$MUTED" "────────────────────────────────────────────────────────────"
    else
        echo "────────────────────────────────────────────────────────────"
    fi
}

print_step() {
    local step_num=$1
    local step_title=$2
    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style --foreground "$BRAND_SECONDARY" --bold "[$step_num] $step_title"
    else
        echo -e "\033[0;36m[$step_num] $step_title\033[0m"
    fi
    print_separator
}

print_banner() {
    if [ "$USE_GUM" = true ]; then
        gum style \
            --foreground "$BRAND_PRIMARY" --bold \
            "  __  __          _   _ _____ ____  " \
            " |  \/  |   /\   | \ | |_   _/ __ \ " \
            " | |\/| |  /  \  |  \| | | || |  | |" \
            " | |  | | / /\ \ | . ` | | || |  | |" \
            " |_|  |_|/_/  \_\_|\_\_| |_||_|  |_|" \
            "" \
            "       Yggdrasil Leaf Service         " \
            "" \
            "        Server Upload Script v1.0 💚"
    else
        echo -e "\033[0;32m  __  __          _   _ _____ ____  \033[0m"
        echo -e "\033[0;32m |  \\/  |   /\\   | \\ | |_   _/ __ \\ \033[0m"
        echo -e "\033[0;32m | |\\/| |  /  \\  |  \\| | | || |  | |\033[0m"
        echo -e "\033[0;32m | |  | | / /\\ \\ | . \\` | | || |  | |\033[0m"
        echo -e "\033[0;32m |_|  |_|/_/  \\_\\_|\\_\\_| |_||_|  |_|\033[0m"
        echo ""
        echo -e "\033[0;32m     Yggdrasil Leaf Service\033[0m"
        echo ""
        echo -e "\033[0;32m       Server Upload Script v1.0 💚\033[0m"
    fi
}

DEPLOY_SERVER="${1:-${DEPLOY_FRONTLEAVES_SERVER:-localhost}}"
DEPLOY_USER="${2:-${DEPLOY_USER:-root}}"
DEPLOY_PATH="${3:-${DEPLOY_PATH:-/root/frontleaves-yggleaf}}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
DEPLOY_SSH_KEY="${DEPLOY_SSH_KEY:-}"

DEFAULT_DEPLOY_PATH="/root/frontleaves-yggleaf"
DEFAULT_DEPLOY_USER="root"

if [ -z "$DEPLOY_SERVER" ]; then
    print_banner
    echo ""
    log_error "缺少必要参数"
    echo ""
    echo "使用方法:"
    echo "  ./script/upload-to-server.sh <server> [user] [path]"
    echo ""
    echo "或使用环境变量:"
    echo "  export DEPLOY_SERVER=your-server.com"
    echo "  export DEPLOY_USER=root"
    echo "  export DEPLOY_PATH=/opt/app"
    echo "  ./script/upload-to-server.sh"
    echo ""
    echo "参数说明:"
    echo "  server  - 服务器地址 (必填)"
    echo "  user    - SSH 用户名 (默认: root)"
    echo "  path    - 部署路径 (默认: $DEFAULT_DEPLOY_PATH)"
    echo ""
    echo "示例:"
    echo "  ./script/upload-to-server.sh 192.168.1.100"
    echo "  ./script/upload-to-server.sh your-server.com admin /opt/app"
    echo "  ./script/upload-to-server.sh your-server.com root /opt/1panel/www/sites/yggleaf/index"
    echo ""
    exit 1
fi

print_banner
echo ""
print_step "1/5" "部署配置"

echo ""
if [ "$USE_GUM" = true ]; then
    gum style \
        --border normal --border-foreground "$BRAND_PRIMARY" --padding "0 2" \
        "$(gum style --bold --foreground "$BRAND_PRIMARY" "部署信息")" \
        "" \
        "$(gum style --foreground "$MUTED" "服务器地址: $DEPLOY_SERVER")" \
        "$(gum style --foreground "$MUTED" "SSH 用户:   $DEPLOY_USER")" \
        "$(gum style --foreground "$MUTED" "SSH 端口:   $DEPLOY_PORT")" \
        "$(gum style --foreground "$MUTED" "部署路径:   $DEPLOY_PATH")"
else
    echo "┌────────────────────────────────────────┐"
    echo "│ 部署信息                               │"
    echo "├────────────────────────────────────────┤"
    echo "│ 服务器地址: $DEPLOY_SERVER"
    echo "│ SSH 用户:   $DEPLOY_USER"
    echo "│ SSH 端口:   $DEPLOY_PORT"
    echo "│ 部署路径:   $DEPLOY_PATH"
    echo "└────────────────────────────────────────┘"
fi
echo ""

print_step "2/5" "环境检查"

if ! command -v rsync &> /dev/null; then
    log_error "rsync 未安装"
    log_info "请先安装 rsync:"
    log_info "  Ubuntu/Debian: apt-get install rsync"
    log_info "  CentOS/RHEL:   yum install rsync"
    log_info "  macOS:         brew install rsync"
    exit 1
fi
log_success "rsync 已安装"

if ! command -v ssh &> /dev/null; then
    log_error "ssh 未安装"
    exit 1
fi
log_success "ssh 已安装"

FILES_TO_UPLOAD=("docker-compose.yml" "Makefile.run" ".env.prod")
for file in "${FILES_TO_UPLOAD[@]}"; do
    if [ ! -f "$file" ]; then
        log_error "文件不存在: $file"
        exit 1
    fi
    log_success "找到文件: $file"
done

echo ""

print_step "3/5" "测试服务器连接"

log_info "正在测试 SSH 连接..."
echo ""

SSH_CMD="ssh -p ${DEPLOY_PORT} -o ConnectTimeout=10 -o StrictHostKeyChecking=no"
if [ -n "$DEPLOY_SSH_KEY" ]; then
    SSH_CMD="$SSH_CMD -i $DEPLOY_SSH_KEY"
fi

if $SSH_CMD ${DEPLOY_USER}@${DEPLOY_SERVER} "echo > /dev/null 2>&1" &> /dev/null; then
    log_success "SSH 连接测试成功"
else
    log_error "SSH 连接测试失败"
    echo ""
    log_info "请检查:"
    log_info "  1. 服务器地址是否正确"
    log_info "  2. SSH 服务是否运行"
    log_info "  3. 网络连接是否正常"
    log_info "  4. SSH 密钥是否正确 (如使用密钥认证)"
    exit 1
fi

echo ""

print_step "4/5" "上传文件到服务器"

log_info "开始上传文件..."
echo ""

RSYNC_CMD="rsync -avz --progress"
RSYNC_CMD="$RSYNC_CMD --exclude='.env'"
RSYNC_CMD="$RSYNC_CMD --exclude='.git'"
RSYNC_CMD="$RSYNC_CMD --exclude='node_modules'"
RSYNC_CMD="$RSYNC_CMD --exclude='*.log'"
RSYNC_CMD="$RSYNC_CMD -e \"ssh -p ${DEPLOY_PORT} -o StrictHostKeyChecking=no\""

if [ -n "$DEPLOY_SSH_KEY" ]; then
    RSYNC_CMD="$RSYNC_CMD -i $DEPLOY_SSH_KEY"
fi

eval "$RSYNC_CMD docker-compose.yml Makefile.run .env.prod ${DEPLOY_USER}@${DEPLOY_SERVER}:${DEPLOY_PATH}/"

echo ""
log_success "文件上传完成"

print_step "5/5" "配置服务器端文件"

log_info "远程重命名文件..."
echo ""

if $SSH_CMD ${DEPLOY_USER}@${DEPLOY_SERVER} \
    "cd ${DEPLOY_PATH} && mv -f Makefile.run Makefile && chmod +x Makefile && mv -f .env.prod .env && chmod 600 .env"; then
    log_success "文件重命名成功"
else
    log_warn "文件重命名失败，请手动执行:"
    echo ""
    echo "  ssh ${DEPLOY_USER}@${DEPLOY_SERVER}"
    echo "  cd ${DEPLOY_PATH}"
    echo "  mv Makefile.run Makefile"
    echo "  chmod +x Makefile"
    echo "  mv .env.prod .env"
    echo "  chmod 600 .env"
    echo ""
fi

echo ""
echo ""

if [ "$USE_GUM" = true ]; then
    gum style \
        --border double --border-foreground "$BRAND_PRIMARY" \
        --padding "1 3" --margin "1 0" \
        "$(gum style --bold --foreground "$BRAND_PRIMARY" '🎉 上传完成')" \
        "" \
        "$(gum style --foreground "$BRAND_SECONDARY" "文件已成功上传到服务器")" \
        "" \
        "$(gum style --foreground "$MUTED" "服务器: ${DEPLOY_SERVER}")" \
        "$(gum style--foreground "$MUTED" "路径:   ${DEPLOY_PATH}")" \
        "" \
        "$(gum style --bold --foreground "$BRAND_PRIMARY" "后续步骤:")" \
        "$(gum style--foreground "$MUTED" "1. SSH 登录服务器:")" \
        "$(gum style --foreground 240 "   ssh ${DEPLOY_USER}@${DEPLOY_SERVER}")" \
        "" \
        "$(gum style--foreground "$MUTED" "2. 进入部署目录:")" \
        "$(gum style --foreground 240 "   cd ${DEPLOY_PATH}")" \
        "" \
        "$(gum style--foreground "$MUTED" "3. 启动服务:")" \
        "$(gum style --foreground 240 "   make run")" \
        "" \
        "$(gum style--foreground "$MUTED" "4. 查看日志:")" \
        "$(gum style --foreground 240 "   make logs")"
else
    echo "╔════════════════════════════════════════════════════╗"
    echo "║         🎉 上传完成                                ║"
    echo "╠════════════════════════════════════════════════════╣"
    echo "║ 文件已成功上传到服务器                            ║"
    echo "║                                                    ║"
    echo "║ 服务器: ${DEPLOY_SERVER}                        ║"
    echo "║ 路径:   ${DEPLOY_PATH} ║"
    echo "║                                                    ║"
    echo "║ 后续步骤:                                          ║"
    echo "║ 1. SSH 登录服务器:                                ║"
    echo "║    ssh ${DEPLOY_USER}@${DEPLOY_SERVER}"
    echo "║                                                    ║"
    echo "║ 2. 进入部署目录:                                  ║"
    echo "║    cd ${DEPLOY_PATH}"
    echo "║                                                    ║"
    echo "║ 3. 启动服务:                                      ║"
    echo "║    make run                                       ║"
    echo "║                                                    ║"
    echo "║ 4. 查看日志:                                      ║"
    echo "║    make logs                                      ║"
    echo "╚════════════════════════════════════════════════════╝"
fi

echo ""
log_success "所有操作已完成！"
echo ""
