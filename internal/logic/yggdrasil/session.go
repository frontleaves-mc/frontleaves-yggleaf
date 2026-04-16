package yggdrasil

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/cache"
)

// HasJoined 验证客户端是否成功加入服务器。
//
// 该方法用于 Minecraft 服务端验证客户端的进服请求。流程如下：
//  1. 从 Redis 中查询 serverId 对应的会话记录
//  2. 根据 profileUUID 查询角色信息（含关联皮肤和披风）
//  3. 验证 username 与令牌绑定角色的名称一致
//  4. 可选验证客户端 IP（当 ip 参数不为空时）
//
// 验证通过后删除会话缓存（一次性使用），并返回包含纹理属性和数字签名的角色信息。
//
// 参数:
//   - ctx: 上下文对象
//   - username: 角色名称
//   - serverId: 服务端生成的随机字符串
//   - ip: 客户端 IP 地址（可选，用于防止代理连接）
//
// 返回值:
//   - *apiYgg.ProfileResponse: 验证通过的角色信息（含签名）
//   - bool: 是否验证通过
//   - *xError.Error: 验证过程中的错误
func (l *YggdrasilLogic) HasJoined(ctx context.Context, username string, serverId string, ip string) (*apiYgg.ProfileResponse, bool, *xError.Error) {
	l.log.Info(ctx, "HasJoined - 验证客户端加入服务器")

	// 从 Redis 中查询会话记录
	sessionData, found, err := l.repo.sessionCache.Get(ctx, serverId)
	if err != nil {
		// Redis 连接故障等非预期错误，向上传播
		return nil, false, xError.NewError(ctx, xError.CacheError, "会话缓存查询失败", true, err)
	}
	if !found {
		// 正常：会话不存在或已过期
		return nil, false, nil
	}

	// 根据 profileUUID 查询角色信息（含纹理）
	profile, found, xErr := l.repo.profileRepo.GetByUUIDUnsignedWithTextures(ctx, nil, sessionData.ProfileUUID)
	if xErr != nil {
		return nil, false, xErr
	}
	if !found {
		return nil, false, nil
	}

	// 验证 username 与角色名称一致
	if profile.Name != username {
		return nil, false, nil
	}

	// 可选验证客户端 IP（防止代理连接）
	// 使用 net.ParseIP 标准化比较，避免 IPv4/IPv6 双栈环境下格式不一致（如 192.168.1.1 vs ::ffff:192.168.1.1）
	if ip != "" {
		parsedClientIP := net.ParseIP(sessionData.ClientIP)
		parsedRequestIP := net.ParseIP(ip)
		if parsedClientIP != nil && parsedRequestIP != nil && !parsedClientIP.Equal(parsedRequestIP) {
			return nil, false, nil
		}
	}

	// 验证通过后删除会话（一次性使用）
	// 删除失败时记录 Warn 日志但不阻断流程：
	//   - TTL 30 秒自然过期已是安全兜底
	//   - Redis 临时故障不应阻止玩家进服（可用性优先于严格的防重放）
	if delErr := l.repo.sessionCache.Delete(ctx, serverId); delErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("删除会话缓存失败（TTL 兜底仍生效）: %v", delErr))
	}

	// hasJoined 必须包含签名（unsigned=false）
	resp := l.BuildProfileResponse(ctx, profile, false)
	return resp, true, nil
}

// JoinServer 记录客户端加入服务器的会话信息。
//
// 该方法由 Minecraft 客户端调用，将 serverId、accessToken 和 profileUUID 的
// 关联关系写入 Redis 缓存，供后续 hasJoined 验证使用。会话 TTL 为 30 秒。
//
// 参数:
//   - ctx: 上下文对象
//   - accessToken: 访问令牌字符串
//   - profileUUID: 角色无符号 UUID
//   - serverId: 服务端生成的随机字符串
//   - clientIP: 客户端 IP 地址
//
// 返回值:
//   - *xError.Error: 操作过程中的错误
func (l *YggdrasilLogic) JoinServer(ctx context.Context, accessToken string, profileUUID string, serverId string, clientIP string) *xError.Error {
	l.log.Info(ctx, "JoinServer - 记录客户端加入服务器会话")

	// 预校验 profileUUID 是否为合法的无符号 UUID 格式
	if _, decodeErr := DecodeUnsignedUUID(profileUUID); decodeErr != nil {
		return xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
	}

	// Handler 层已完成令牌有效性、角色绑定、一致性验证（Yggdrasil 错误格式控制），
	// Logic 层仅负责 Redis 会话写入。

	sessionData := &cache.SessionData{
		AccessToken: accessToken,
		ProfileUUID: profileUUID,
		ClientIP:    clientIP,
	}

	if err := l.repo.sessionCache.Set(ctx, serverId, sessionData); err != nil {
		return xError.NewError(ctx, xError.ServerInternalError, "写入会话缓存失败", true, err)
	}

	return nil
}

// BuildProfileResponse 根据 GameProfile 实体构建 Yggdrasil 协议的角色信息响应。
//
// 组装角色 ID（无符号 UUID）、名称和 textures 属性（含 Base64 编码的材质载荷）。
// 根据 unsigned 参数决定是否包含数字签名。
//
// 参数:
//   - ctx: 上下文对象，用于日志记录
//   - profile: 游戏档案实体（需包含预加载的 SkinLibrary 和 CapeLibrary）
//   - unsigned: true 不含签名 / false 含签名
//
// 返回值:
//   - *apiYgg.ProfileResponse: 构建完成的角色信息响应
func (l *YggdrasilLogic) BuildProfileResponse(ctx context.Context, profile *entity.GameProfile, unsigned bool) *apiYgg.ProfileResponse {
	profileID := EncodeUnsignedUUID(profile.UUID)

	// 通过 beacon-bucket SDK 解析纹理下载链接（使用 Texture ID 而非 Hash）
	var skinURL string
	var skinModel entity.ModelType
	var capeURL string
	if profile.SkinLibrary != nil {
		skinURL = l.resolveTextureURL(ctx, profile.SkinLibrary.Texture)
		skinModel = profile.SkinLibrary.Model
	}
	if profile.CapeLibrary != nil {
		capeURL = l.resolveTextureURL(ctx, profile.CapeLibrary.Texture)
	}

	// 构建材质载荷并 Base64 编码
	payload := l.BuildTexturesPayload(profileID, profile.Name, skinURL, skinModel, capeURL)
	var value string
	payloadBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		l.log.Error(ctx, fmt.Sprintf("材质载荷序列化失败: %v", marshalErr))
		// 序列化失败时返回不含 textures 属性的精简响应，避免对空字符串签名产生误导性结果
		return &apiYgg.ProfileResponse{
			ID:   profileID,
			Name: profile.Name,
		}
	} else {
		value = encodeBase64(payloadBytes)
	}

	// 构建 textures 属性
	prop := apiYgg.PropertyResponse{
		Name:  "textures",
		Value: value,
	}

	// 若需要签名，计算 SHA1withRSA 签名
	if !unsigned {
		sig, sigErr := l.SignTexturesProperty(value)
		if sigErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("签名材质属性失败: %v", sigErr))
		} else {
			prop.Signature = sig
		}
	}

	return &apiYgg.ProfileResponse{
		ID:         profileID,
		Name:       profile.Name,
		Properties: []apiYgg.PropertyResponse{prop},
	}
}
