package yggdrasil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// mojangLookupResponse Mojang 玩家名称查找 API 的响应结构。
type mojangLookupResponse struct {
	ID   string `json:"id"`   // 无连字符 UUID
	Name string `json:"name"` // 玩家名称
}

// mojangSessionResponse Mojang 会话档案 API 的响应结构。
type mojangSessionResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Properties []struct {
		Name      string `json:"name"`
		Value     string `json:"value"`
		Signature string `json:"signature"`
	} `json:"properties"`
}

// mojangTexturesPayload Mojang 会话响应中 Base64 解码后的纹理结构。
type mojangTexturesPayload struct {
	Textures struct {
		SKIN *struct {
			URL      string `json:"url"`
			Metadata *struct {
				Model string `json:"model"`
			} `json:"metadata,omitempty"`
		} `json:"SKIN,omitempty"`
		CAPE *struct {
			URL string `json:"url"`
		} `json:"CAPE,omitempty"`
	} `json:"textures"`
}

// GetOnlineProfileWithFallback 获取缓存的在线档案，过期时从 Mojang API 刷新。
//
// 流程:
//  1. 查询 DB 缓存（未过期记录）
//  2. 缓存命中 → 直接返回（IsOnline=false 的记录也返回，表示该用户非正版）
//  3. 缓存未命中 → 加锁 → 双重检查 → 调用 Mojang API → 写入缓存 → 返回
//
// 参数:
//   - ctx: 上下文对象
//   - profileName: 玩家名称（用于 Mojang 查找）
//   - profileID: 游戏档案 Snowflake ID（用于缓存查询/写入）
//
// 返回值:
//   - *entity.GameOnlineProfile: 在线档案缓存（可能为 nil，表示获取失败）
//   - *xError.Error: 错误（nil 表示正常）
func (l *YggdrasilLogic) GetOnlineProfileWithFallback(ctx context.Context, profileName string, profileID xSnowflake.SnowflakeID) (*entity.GameOnlineProfile, *xError.Error) {
	// 1. 查缓存
	cached, found, xErr := l.repo.onlineProfileRepo.GetValidByGameProfileID(ctx, nil, profileID)
	if xErr != nil {
		return nil, xErr
	}
	if found {
		return cached, nil
	}

	// 2. 缓存未命中，加 per-name 锁防止重复调用
	mu := l.profileNameMutex(profileName)
	mu.Lock()
	defer mu.Unlock()

	// 3. 双重检查（可能在等锁期间已被其他请求填充）
	cached, found, xErr = l.repo.onlineProfileRepo.GetValidByGameProfileID(ctx, nil, profileID)
	if xErr != nil {
		return nil, xErr
	}
	if found {
		return cached, nil
	}

	// 4. 调用 Mojang API 并写入缓存
	return l.fetchMojangProfile(ctx, profileName, profileID)
}

// fetchMojangProfile 从 Mojang API 获取正版皮肤/披风信息并更新缓存。
//
// 流程:
//  1. 调用 lookup/name API 获取正版 UUID
//  2. 若玩家不存在（404）→ 缓存 IsOnline=false 记录
//  3. 调用 session/minecraft/profile API 获取纹理信息
//  4. Base64 解码 textures 属性，提取皮肤/披风 URL 和模型类型
//  5. 写入/更新 game_online_profile 缓存记录
func (l *YggdrasilLogic) fetchMojangProfile(ctx context.Context, profileName string, profileID xSnowflake.SnowflakeID) (*entity.GameOnlineProfile, *xError.Error) {
	expiresAt := time.Now().Add(time.Duration(bConst.OnlineProfileCacheDurationMin) * time.Minute)

	// Step 1: 查正版 UUID
	onlineUUID, err := l.lookupMojangUUID(ctx, profileName)
	if err != nil {
		// 玩家不存在或查询失败 → 缓存非正版记录
		l.log.Warn(ctx, fmt.Sprintf("Mojang 查找玩家 UUID 失败(%s)，缓存为非正版用户: %v", profileName, err))
		onlineProfile := &entity.GameOnlineProfile{
			GameProfileID: profileID,
			IsOnline:      false,
			ExpiresAt:     expiresAt,
		}
		if _, upsertErr := l.repo.onlineProfileRepo.Upsert(ctx, nil, onlineProfile); upsertErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("缓存非正版记录失败: %v", upsertErr))
			return nil, nil // best-effort
		}
		return onlineProfile, nil
	}

	// Step 2: 查皮肤/披风
	textures, err := l.fetchMojangSessionProfile(ctx, onlineUUID)
	if err != nil {
		// UUID 存在但纹理查询失败 → 仍缓存为正版（有 UUID 但纹理暂时不可用）
		l.log.Warn(ctx, fmt.Sprintf("Mojang 查询纹理失败(%s)，缓存为正版无纹理: %v", profileName, err))
		onlineUUIDStr := onlineUUID
		onlineProfile := &entity.GameOnlineProfile{
			GameProfileID: profileID,
			OnlineUUID:    &onlineUUIDStr,
			IsOnline:      true,
			ExpiresAt:     expiresAt,
		}
		if _, upsertErr := l.repo.onlineProfileRepo.Upsert(ctx, nil, onlineProfile); upsertErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("缓存正版无纹理记录失败: %v", upsertErr))
			return nil, nil
		}
		return onlineProfile, nil
	}

	// Step 3: 解析纹理信息并缓存
	onlineUUIDStr := onlineUUID
	onlineProfile := &entity.GameOnlineProfile{
		GameProfileID: profileID,
		OnlineUUID:    &onlineUUIDStr,
		IsOnline:      true,
		ExpiresAt:     expiresAt,
	}

	if textures.Textures.SKIN != nil {
		skinURL := textures.Textures.SKIN.URL
		onlineProfile.SkinURL = &skinURL
		if textures.Textures.SKIN.Metadata != nil && textures.Textures.SKIN.Metadata.Model == "slim" {
			model := entity.ModelTypeSlim
			onlineProfile.SkinModel = &model
		} else {
			model := entity.ModelTypeClassic
			onlineProfile.SkinModel = &model
		}
	}
	if textures.Textures.CAPE != nil {
		capeURL := textures.Textures.CAPE.URL
		onlineProfile.CapeURL = &capeURL
	}

	result, upsertErr := l.repo.onlineProfileRepo.Upsert(ctx, nil, onlineProfile)
	if upsertErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("缓存正版纹理记录失败: %v", upsertErr))
		return onlineProfile, nil // 返回内存中的数据（未被持久化，但仍然可用）
	}
	return result, nil
}

// lookupMojangUUID 调用 Mojang 名称查找 API 获取正版 UUID。
func (l *YggdrasilLogic) lookupMojangUUID(ctx context.Context, name string) (string, error) {
	url := bConst.MojangAPIProfileLookupURL + name

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("创建 Mojang lookup 请求失败: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Mojang lookup 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("玩家 %s 不存在于 Mojang", name)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Mojang lookup 返回异常状态码: %d", resp.StatusCode)
	}

	var result mojangLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 Mojang lookup 响应失败: %w", err)
	}
	return result.ID, nil
}

// fetchMojangSessionProfile 调用 Mojang 会话 API 获取纹理信息。
func (l *YggdrasilLogic) fetchMojangSessionProfile(ctx context.Context, onlineUUID string) (*mojangTexturesPayload, error) {
	url := bConst.MojangAPISessionProfileURL + onlineUUID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Mojang session 请求失败: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Mojang session 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("Mojang session 未找到 UUID: %s", onlineUUID)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("Mojang session API 限流(429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mojang session 返回异常状态码: %d", resp.StatusCode)
	}

	var sessionResp mojangSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return nil, fmt.Errorf("解析 Mojang session 响应失败: %w", err)
	}

	// 查找 textures 属性
	var texturesValue string
	for _, prop := range sessionResp.Properties {
		if prop.Name == "textures" {
			texturesValue = prop.Value
			break
		}
	}
	if texturesValue == "" {
		return nil, fmt.Errorf("Mojang session 响应中未找到 textures 属性")
	}

	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(texturesValue)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码 textures 失败: %w", err)
	}

	var payload mojangTexturesPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, fmt.Errorf("解析 textures JSON 失败: %w", err)
	}
	return &payload, nil
}

// profileNameMutex 获取指定玩家名称的去重互斥锁。
func (l *YggdrasilLogic) profileNameMutex(name string) *sync.Mutex {
	mu, _ := l.mojangMu.LoadOrStore(name, &sync.Mutex{})
	return mu.(*sync.Mutex)
}
