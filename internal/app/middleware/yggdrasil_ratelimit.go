package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	"github.com/gin-gonic/gin"
)

// YggdrasilAuthRateLimit Yggdrasil 认证接口速率限制中间件。
//
// Yggdrasil 规范 §11.2 明确要求 authenticate 和 signout 接口必须实施
// 速率限制（按用户而非 IP），防止暴力破解和 DoS 攻击。
//
// 使用 Redis INCR + EXPIRE 实现固定窗口计数器算法：
//   - Key: fyl:yggdrasil:ratelimit:{endpoint}:{sha256(username)}
//   - Value: 窗口内请求计数
//   - TTL: 窗口时长（60 秒）
//
// 按 username 哈希值限流而非客户端 IP，因为：
//   1. 同一用户可能通过代理/VPN 使用不同 IP
//   2. 多个合法用户可能共享同一出口 IP（NAT/CDN）
//   3. 攻击者无法通过伪造 X-Forwarded-For 绕过限制
//
// 参数:
//   - endpoint: 接口标识（"authenticate" 或 "signout"）
//   - maxAttempts: 窗口内最大允许请求次数
func YggdrasilAuthRateLimit(endpoint string, maxAttempts int) gin.HandlerFunc {
	log := xLog.WithName(xLog.NamedMIDE, "YggdrasilAuthRateLimit")

	return func(c *gin.Context) {
		// 手动读取并缓存请求体，避免 ShouldBindJSON 消费 Body 后
		// 下游 Handler（Authenticate/Signout）无法再次绑定请求体（Gin 不自动缓存 Body）
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			// 读取失败时放行，由后续 Handler 处理
			c.Next()
			return
		}
		// 立即还原 Body，确保下游 Handler 可正常读取
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// 使用缓存的字节提取 username（不消费 Body）
		var rawReq struct {
			Username string `json:"username"`
		}
		if err := json.Unmarshal(bodyBytes, &rawReq); err != nil || rawReq.Username == "" {
			// 解析失败或无 username 时跳过限流（由后续 Handler 返回错误响应）
			c.Next()
			return
		}

		// 基于 username 的 SHA-256 哈希生成限流 key（避免 username 特殊字符污染 Redis Key）
		usernameHash := sha256.Sum256([]byte(rawReq.Username))
		rateLimitKey := fmt.Sprintf("fyl:yggdrasil:ratelimit:%s:%s", endpoint, hex.EncodeToString(usernameHash[:]))

		// 获取 Redis 客户端
		rdb := xCtxUtil.MustGetRDB(c.Request.Context())

		// 原子递增 + 设置过期时间（首次请求时）
		count, err := rdb.Incr(c.Request.Context(), rateLimitKey).Result()
		if err != nil {
			// Redis 故障时放行（避免因 Redis 不可用导致所有用户无法登录）
			log.Warn(c.Request.Context(), fmt.Sprintf("速率限制 Redis 操作失败（已放行）: %v", err))
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(c.Request.Context(), rateLimitKey, time.Duration(bConst.YggdrasilRateLimitWindowSec)*time.Second)
		}

		// 超出限制时返回 429 Too Many Requests
		if count > int64(maxAttempts) {
			log.Info(c.Request.Context(), fmt.Sprintf(
				"接口 %s 速率限制触发: username=%s, count=%d/%d, window=%ds",
				endpoint, rawReq.Username, count, maxAttempts, bConst.YggdrasilRateLimitWindowSec,
			))
			apiYgg.AbortYggError(c, http.StatusTooManyRequests, "TooManyRequests",
				"请求过于频繁，请稍后再试")
			return
		}

		c.Next()
	}
}
