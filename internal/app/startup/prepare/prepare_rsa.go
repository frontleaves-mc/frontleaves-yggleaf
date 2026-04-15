package prepare

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// LoadRSAKeyPair 加载或生成 Yggdrasil RSA 签名密钥对。
//
// 该方法执行以下逻辑：
//  1. 从环境变量读取密钥文件路径
//  2. 若密钥文件不存在，自动生成 RSA-2048 密钥对并写入文件
//  3. 加载私钥并导出公钥 PEM，组装为 RSAKeyPair 返回
//
// 密钥对用于 Yggdrasil 协议中的 textures 属性数字签名（SHA1withRSA）。
//
// 返回值:
//   - *bConst.RSAKeyPair: 加载完成的密钥对
//   - error: 加载或生成过程中的错误
func LoadRSAKeyPair() (*bConst.RSAKeyPair, error) {
	privKeyPath := xEnv.GetEnvString(bConst.EnvYggdrasilPrivateKeyPath, "keys/yggdrasil_private.pem")
	pubKeyPath := xEnv.GetEnvString(bConst.EnvYggdrasilPublicKeyPath, "keys/yggdrasil_public.pem")

	// 检查密钥文件是否存在，不存在则生成
	if _, err := os.Stat(privKeyPath); err != nil {
		if os.IsNotExist(err) {
			if genErr := generateRSAKeyPair(privKeyPath, pubKeyPath); genErr != nil {
				return nil, fmt.Errorf("生成 Yggdrasil RSA 密钥对失败: %w", genErr)
			}
		} else {
			return nil, fmt.Errorf("检查密钥文件状态失败: %w", err)
		}
	}

	// 加载私钥
	privKey, err := loadPrivateKey(privKeyPath)
	if err != nil {
		return nil, fmt.Errorf("加载 Yggdrasil RSA 私钥失败: %w", err)
	}

	// 导出公钥 PEM
	pubKeyPEM, err := exportPublicKeyPEM(&privKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("导出 Yggdrasil RSA 公钥 PEM 失败: %w", err)
	}

	return &bConst.RSAKeyPair{
		PrivKey:   privKey,
		PubKeyPEM: pubKeyPEM,
	}, nil
}

// generateRSAKeyPair 生成 RSA-2048 密钥对并写入指定文件路径。
func generateRSAKeyPair(privKeyPath, pubKeyPath string) error {
	// 确保目录存在
	dir := filepath.Dir(privKeyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 生成 RSA-2048 密钥对
	// 注意：选择 2048 位而非 4096 位是 Yggdrasil 生态的事实标准（authlib-injector / Skin Providers 均使用 2048）。
	// 升级至 4096 会导致兼容性问题——许多现有客户端实现不支持 4096 位签名验证，玩家皮肤/披风将无法加载。
	// 若未来需升级密钥强度，必须同步更新所有客户端的验证逻辑（不现实），因此保持 2048 位。
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("生成 RSA 密钥对失败: %w", err)
	}

	// 写入私钥文件（权限 0600：仅所有者可读写，防止同机用户读取签名私钥）
	privFile, err := os.OpenFile(privKeyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("创建私钥文件失败: %w", err)
	}
	defer privFile.Close()

	if err := pem.Encode(privFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		return fmt.Errorf("写入私钥文件失败: %w", err)
	}

	// 写入公钥文件（权限 0644：公钥需被 API 元数据接口读取）
	pubFile, err := os.OpenFile(pubKeyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建公钥文件失败: %w", err)
	}
	defer pubFile.Close()

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("序列化公钥失败: %w", err)
	}

	if err := pem.Encode(pubFile, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}); err != nil {
		return fmt.Errorf("写入公钥文件失败: %w", err)
	}

	return nil
}

// loadPrivateKey 从 PEM 文件加载 RSA 私钥。
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	block, rest := pem.Decode(data)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("无效的私钥 PEM 格式")
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("私钥文件包含意外的额外数据，可能已损坏或被篡改")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return privKey, nil
}

// exportPublicKeyPEM 将 RSA 公钥导出为 PEM 格式字符串。
func exportPublicKeyPEM(pubKey *rsa.PublicKey) (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("序列化公钥失败: %w", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubPEM), nil
}
