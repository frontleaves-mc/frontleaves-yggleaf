package yggdrasil

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	yggdrasilAPI "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	"github.com/google/uuid"
)

// yggUserNamespaceUUID 用于从 SnowflakeID 派生确定性 UUID 的命名空间。
//
// 使用 RFC 4122 定义的 NameSpace_DNS (UUIDv5 DNS 命名空间)。
// 虽然 RFC 4122 语义上 DNS 命名空间应用于"全局唯一的 DNS 名字"，但在此场景下：
//   1. 输出仍是确定性 UUID（同一 SnowflakeID 始终映射到相同 UUID）
//   2. 碰撞概率极低（需另一系统同时使用 NameSpace_DNS + 相同 SnowflakeID 字符串）
//   3. 若需彻底消除理论碰撞风险，可替换为项目专用 v4 UUID（uuid.New() 一次后硬编码）
var yggUserNamespaceUUID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// SignTexturesProperty 使用 SHA1withRSA 算法对 textures 属性的 Base64 值进行数字签名。
//
// 根据 Yggdrasil 协议规范（§6），签名流程为：
//  1. 对 Base64 编码的 value 字符串计算 SHA-1 摘要
//  2. 使用 RSA 私钥对摘要进行 PKCS#1 v1.5 签名
//  3. 将签名结果进行 Base64 编码
//
// 参数:
//   - value: textures 属性的 Base64 编码值（待签名的原始数据）
//
// 返回值:
//   - string: Base64 编码的数字签名字符串
//   - error: 私钥未初始化或签名计算过程中发生的错误
func (l *YggdrasilLogic) SignTexturesProperty(value string) (string, error) {
	if l.privKey == nil {
		return "", fmt.Errorf("RSA 私钥未初始化，无法进行签名")
	}

	// 计算 SHA-1 摘要
	hash := sha1.Sum([]byte(value))

	// 使用 PKCS#1 v1.5 签名
	signature, err := rsa.SignPKCS1v15(nil, l.privKey, crypto.SHA1, hash[:])
	if err != nil {
		return "", fmt.Errorf("RSA 签名失败: %w", err)
	}

	// Base64 编码签名结果
	return base64.StdEncoding.EncodeToString(signature), nil
}

// BuildTexturesPayload 构建 Yggdrasil 协议规定的 textures 材质信息载荷。
//
// 根据 Yggdrasil 协议规范（§4.3），组装包含时间戳、角色标识、角色名称和
// 材质信息的 JSON 结构。该结构需要经 Base64 编码后作为 textures 属性的 value。
//
// 材质 URL 格式遵循项目常量 YggdrasilTextureURLTemplate 中定义的模板，
// 即 https://yggleaf.frontleaves.com/textures/{TextureHash}。
//
// 参数:
//   - profileID: 角色的无符号 UUID（去除连字符）
//   - profileName: 角色名称
//   - skinURL: 皮肤材质的完整 URL，为空时省略 SKIN 字段
//   - skinModel: 皮肤模型类型，由 entity.ModelType 决定（"default" 或 "slim"）
//   - capeURL: 披风材质的完整 URL，为空时省略 CAPE 字段
//
// 返回值:
//   - *yggdrasilAPI.TexturesPayload: 组装完成的材质信息载荷
func (l *YggdrasilLogic) BuildTexturesPayload(
	profileID string,
	profileName string,
	skinHash string,
	skinModel entity.ModelType,
	capeHash string,
) *yggdrasilAPI.TexturesPayload {
	payload := &yggdrasilAPI.TexturesPayload{
		Timestamp:   currentTimeMillis(),
		ProfileID:   profileID,
		ProfileName: profileName,
		Textures:    yggdrasilAPI.TexturesInfo{},
	}

	// 填充皮肤材质信息
	if skinHash != "" {
		skinTexture := &yggdrasilAPI.SkinTexture{
			URL: fmt.Sprintf(bConst.YggdrasilTextureURLTemplate, skinHash),
		}
		// 仅纤细模型需要设置 metadata.model，经典模型省略（客户端默认 classic）
		if skinModel == entity.ModelTypeSlim {
			skinTexture.Metadata = &yggdrasilAPI.SkinMetadata{
				Model: "slim",
			}
		}
		payload.Textures.SKIN = skinTexture
	}

	// 填充披风材质信息
	if capeHash != "" {
		payload.Textures.CAPE = &yggdrasilAPI.CapeTexture{
			URL: fmt.Sprintf(bConst.YggdrasilTextureURLTemplate, capeHash),
		}
	}

	return payload
}

// EncodeUnsignedUUID 将标准 UUID 字符串转换为无连字符格式。
//
// Yggdrasil 协议中所有 UUID 均使用无连字符格式（32 位十六进制字符串），
// 该方法用于将标准 UUID（8-4-4-4-12 格式）转换为协议要求的格式。
//
// 参数:
//   - u: 标准 UUID 对象
//
// 返回值:
//   - string: 去除连字符后的 32 位十六进制字符串
func EncodeUnsignedUUID(u uuid.UUID) string {
	return strings.ReplaceAll(u.String(), "-", "")
}

// DecodeUnsignedUUID 将无连字符 UUID 字符串还原为标准 UUID 格式。
//
// 将 32 位无连字符 UUID 字符串还原为标准的 8-4-4-4-12 格式，
// 并解析为 uuid.UUID 对象。
//
// 参数:
//   - s: 32 位无连字符 UUID 字符串
//
// 返回值:
//   - uuid.UUID: 解析得到的标准 UUID 对象
//   - error: 字符串长度不合法或格式无效时返回错误
func DecodeUnsignedUUID(s string) (uuid.UUID, error) {
	if len(s) != 32 {
		return uuid.Nil, fmt.Errorf("无效的无连字符 UUID 长度: 期望 32, 实际 %d", len(s))
	}

	// 还原为标准格式: 8-4-4-4-12
	standard := s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
	return uuid.Parse(standard)
}

// IsValidUnsignedUUID 验证字符串是否为合法的无符号 UUID 格式。
//
// 无符号 UUID 为 32 个十六进制字符（0-9, a-f, A-F）。
// 该函数是 Handler 层和 Repository 层的统一入口，
// 避免在 4 处重复实现相同的校验逻辑。
//
// 参数:
//   - s: 待验证的字符串
//
// 返回值:
//   - bool: true 表示合法的无符号 UUID 格式
func IsValidUnsignedUUID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// DeriveUserUUID 从用户的 Snowflake ID 派生一个确定性的 UUID。
//
// 使用 UUIDv5 (SHA-1 命名空间) 算法，将 SnowflakeID 映射为 RFC 4122 兼容的 UUID。
// 该方法保证同一 SnowflakeID 始终生成相同的 UUID，满足 Yggdrasil 协议中
// 用户需要拥有稳定 UUID 标识的需求。
//
// 参数:
//   - snowflakeID: 用户的雪花算法 ID
//
// 返回值:
//   - uuid.UUID: 派生得到的确定性 UUID
func DeriveUserUUID(snowflakeID int64) uuid.UUID {
	return uuid.NewSHA1(yggUserNamespaceUUID, []byte(fmt.Sprintf("%d", snowflakeID)))
}

// currentTimeMillis 返回当前时间的毫秒级 Unix 时间戳。
//
// 用于填充 TexturesPayload 中的 timestamp 字段，
// 符合 Yggdrasil 协议中使用毫秒级时间戳的约定。
func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

// encodeBase64 将字节数组编码为标准 Base64 字符串。
//
// 用于将 JSON 序列化后的材质载荷编码为 Yggdrasil 协议要求的 Base64 格式。
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
