package yggdrasil

import (
	"context"
	"fmt"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ValidateGameToken 验证游戏令牌的有效性。
//
// 根据 accessToken 查询游戏令牌记录，验证令牌状态是否为有效（GameTokenStatusValid）。
// 该方法被 Yggdrasil Bearer 认证中间件调用。
//
// 参数:
//   - ctx: 上下文对象
//   - accessToken: 访问令牌字符串
//
// 返回值:
//   - *entity.GameToken: 有效的游戏令牌实体
//   - bool: 是否找到有效令牌
//   - *xError.Error: 查询过程中发生的错误
func (l *YggdrasilLogic) ValidateGameToken(ctx context.Context, accessToken string) (*entity.GameToken, bool, *xError.Error) {
	l.log.Info(ctx, "ValidateGameToken - 验证游戏令牌")

	token, found, xErr := l.repo.gameTokenRepo.GetByAccessToken(ctx, nil, accessToken)
	if xErr != nil {
		return nil, false, xErr
	}
	if !found {
		return nil, false, nil
	}
	if token.Status != entity.GameTokenStatusValid {
		return nil, false, nil
	}

	// 检查令牌是否已过期
	if time.Now().After(token.ExpiresAt) {
		return nil, false, nil
	}

	return token, true, nil
}

// AuthenticateUser 用户登录认证。
//
// 支持邮箱或手机号作为登录凭证，验证密码后生成游戏令牌。
// 单角色时自动绑定到令牌并返回 selectedProfile，多角色时通过 refresh 选择。
//
// 参数:
//   - ctx: 上下文对象
//   - username: 邮箱或手机号
//   - password: 明文密码
//   - clientToken: 客户端令牌标识（可选）
//   - requestUser: 是否请求用户信息
//
// 返回值:
//   - accessToken: 服务端生成的访问令牌
//   - clientToken: 与请求中相同的客户端令牌
//   - availableProfiles: 用户可用的角色列表
//   - selectedProfile: 自动选中的角色（单角色时）
//   - userResp: 用户信息（仅在 requestUser=true 时返回）
//   - *xError.Error: 认证过程中的错误
func (l *YggdrasilLogic) AuthenticateUser(ctx context.Context, username string, password string, clientToken string, requestUser bool) (string, string, []entity.GameProfile, *entity.GameProfile, *entity.User, *xError.Error) {
	l.log.Info(ctx, "AuthenticateUser - 用户登录认证")

	// 1. 查找用户：先尝试邮箱，再尝试手机号
	user, found, xErr := l.repo.userRepo.GetByEmail(ctx, nil, username)
	if xErr != nil {
		return "", "", nil, nil, nil, xErr
	}
	if !found {
		user, found, xErr = l.repo.userRepo.GetByPhone(ctx, nil, username)
		if xErr != nil {
			return "", "", nil, nil, nil, xErr
		}
	}
	if !found {
		// 恒定时间比较：即使用户不存在也执行 bcrypt 比较，防止时序侧信道泄露账号存在性
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"), []byte(password))
		return "", "", nil, nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
	}

	// 2. 检查用户是否被封禁
	if user.HasBan {
		return "", "", nil, nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
	}

	// 3. 验证密码（bcrypt 对比 GamePassword）
	if err := bcrypt.CompareHashAndPassword([]byte(user.GamePassword), []byte(password)); err != nil {
		return "", "", nil, nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
	}

	// 4. 创建游戏令牌
	gameToken, xErr := l.CreateGameToken(ctx, user.ID, clientToken)
	if xErr != nil {
		return "", "", nil, nil, nil, xErr
	}

	// 5. 查询用户的游戏档案列表
	profiles, xErr := l.repo.profileRepo.ListByUserID(ctx, nil, user.ID)
	if xErr != nil {
		return "", "", nil, nil, nil, xErr
	}

	// 6. 单角色时绑定并返回 selectedProfile（绑定成功后才设置）
	var selectedProfile *entity.GameProfile
	if len(profiles) == 1 {
		// 先尝试绑定角色到令牌，成功后才设置 selectedProfile
		_, updateErr := l.repo.gameTokenRepo.UpdateBoundProfile(ctx, nil, gameToken.ID, &profiles[0].ID)
		if updateErr != nil {
			l.log.Error(ctx, fmt.Sprintf("绑定角色到令牌失败: %s", updateErr.ErrorMessage))
			// 回滚刚创建的令牌，避免产生无角色绑定的"孤儿令牌"占用配额槽位
			if _, rollbackErr := l.repo.gameTokenRepo.InvalidateByAccessToken(ctx, nil, gameToken.AccessToken); rollbackErr != nil {
				l.log.Error(ctx, fmt.Sprintf("回滚孤儿令牌失败: %s", rollbackErr.ErrorMessage))
			}
			// 不设置 selectedProfile，客户端将通过 refresh 流程选择角色
		} else {
			selectedProfile = &profiles[0]
		}
	}

	// 7. 构建 user 信息（仅在 requestUser=true 时返回）
	var userResp *entity.User
	if requestUser {
		userResp = user
	}

	return gameToken.AccessToken, gameToken.ClientToken, profiles, selectedProfile, userResp, nil
}

// RefreshToken 刷新游戏令牌。
//
// 吊销原令牌并颁发新令牌。若携带 selectedProfile 则为角色选择操作。
// 暂时失效状态的令牌也可以执行刷新。新令牌的 clientToken 与原令牌相同。
//
// 参数:
//   - ctx: 上下文对象
//   - accessToken: 当前访问令牌
//   - clientToken: 客户端令牌（可选，用于验证）
//   - selectedProfileID: 要选择的角色无符号 UUID（可选）
//   - requestUser: 是否请求用户信息
//
// 返回值:
//   - newAccessToken: 新的访问令牌
//   - clientToken: 客户端令牌
//   - selectedProfile: 选中的角色信息
//   - userResp: 用户信息（仅在 requestUser=true 时返回）
//   - *xError.Error: 刷新过程中的错误
func (l *YggdrasilLogic) RefreshToken(ctx context.Context, accessToken string, clientToken string, selectedProfileID string, requestUser bool) (string, string, *entity.GameProfile, *entity.User, *xError.Error) {
	l.log.Info(ctx, "RefreshToken - 刷新游戏令牌")

	// 查找原令牌
	var oldToken *entity.GameToken
	var found bool
	var xErr *xError.Error

	if clientToken != "" {
		oldToken, found, xErr = l.repo.gameTokenRepo.GetByAccessTokenAndClientToken(ctx, nil, accessToken, clientToken)
	} else {
		oldToken, found, xErr = l.repo.gameTokenRepo.GetByAccessToken(ctx, nil, accessToken)
	}
	if xErr != nil {
		return "", "", nil, nil, xErr
	}
	if !found {
		return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
	}

	// 检查令牌状态和有效期（与 ValidateGameToken 保持一致）
	// 防止已吊销或已过期的令牌被刷新续期
	if oldToken.Status != entity.GameTokenStatusValid &&
		oldToken.Status != entity.GameTokenStatusTempInvalid {
		return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
	}
	if time.Now().After(oldToken.ExpiresAt) {
		return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
	}

	// 生成新令牌的 accessToken（内联生成，不经过 CreateGameToken 以避免触发配额检查）
	// 状态检查已移入 RevokeAndCreate 事务内部（WHERE status IN (Valid, TempInvalid) + RowsAffected 判断），
	// 消除 TOCTOU 竞态条件：并发刷新请求中仅第一个能成功吊销原令牌
	accessUUID, err := uuid.NewRandom()
	if err != nil {
		return "", "", nil, nil, xError.NewError(ctx, xError.ServerInternalError, "生成访问令牌失败", true, err)
	}

	// 构建新令牌实体
	now := time.Now()
	newTokenEntity := &entity.GameToken{
		AccessToken: accessUUID.String(),
		ClientToken: oldToken.ClientToken,
		UserID:      oldToken.UserID,
		Status:      entity.GameTokenStatusValid,
		IssuedAt:    now,
		ExpiresAt:   now.Add(time.Duration(bConst.YggdrasilTokenExpireHours) * time.Hour),
	}

	var selectedProfile *entity.GameProfile
	var bindProfileID *xSnowflake.SnowflakeID

	// 处理角色选择
	if selectedProfileID != "" {
		// 预校验 selectedProfileID 是否为合法的无符号 UUID 格式
		if _, decodeErr := DecodeUnsignedUUID(selectedProfileID); decodeErr != nil {
			return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
		}

		// 原令牌已绑定角色时不能再选择
		if oldToken.BoundProfileID != nil {
			return "", "", nil, nil, xError.NewError(ctx, xError.OperationDenied, "Access token already has a profile assigned.", true)
		}

		// 验证角色属于该用户
		profile, found, xErr := l.repo.profileRepo.GetByUserIDAndUUID(ctx, nil, oldToken.UserID, selectedProfileID)
		if xErr != nil {
			return "", "", nil, nil, xErr
		}
		if !found {
			return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
		}

		bindProfileID = &profile.ID
		selectedProfile = profile
	} else if oldToken.BoundProfileID != nil {
		// 继承原令牌的角色绑定（在事务内原子执行）
		bindProfileID = oldToken.BoundProfileID
	}

	// 在事务内原子执行：吊销旧令牌 + 创建新令牌 + （可选）绑定角色
	newToken, xErr := l.repo.gameTokenTxnRepo.RevokeAndCreate(ctx, oldToken.ID, newTokenEntity, bindProfileID)
	if xErr != nil {
		return "", "", nil, nil, xErr
	}

	// 继承原令牌角色时，需查询绑定的角色信息用于响应
	if selectedProfile == nil && bindProfileID != nil {
		boundProfile, boundFound, boundErr := l.repo.profileRepo.GetByID(ctx, nil, *bindProfileID)
		if boundErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("查询绑定角色失败: %s", boundErr.ErrorMessage))
		} else if boundFound {
			selectedProfile = boundProfile
		}
	}

	// 构建 user 信息
	var userResp *entity.User
	if requestUser {
		// 查询用户信息
		userIDStr := oldToken.UserID.String()
		user, found, xErr := l.repo.userRepo.Get(ctx, userIDStr)
		if xErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("查询用户信息失败: %s", xErr.ErrorMessage))
		} else if found {
			userResp = user
		}
	}

	return newToken.AccessToken, newToken.ClientToken, selectedProfile, userResp, nil
}

// InvalidateToken 吊销指定游戏令牌。
//
// 仅检查 accessToken，忽略 clientToken。
// 使用条件性原子 UPDATE（WHERE access_token = ? AND status = Valid AND expires_at > NOW），
// 避免 SELECT + UPDATE 之间的 TOCTOU 竞态窗口。无论令牌是否存在或已吊销，均不返回错误。
//
// 参数:
//   - ctx: 上下文对象
//   - accessToken: 访问令牌
func (l *YggdrasilLogic) InvalidateToken(ctx context.Context, accessToken string) *xError.Error {
	l.log.Info(ctx, "InvalidateToken - 吊销游戏令牌")

	_, xErr := l.repo.gameTokenRepo.InvalidateByAccessToken(ctx, nil, accessToken)
	return xErr
}

// SignoutUser 吊销用户所有游戏令牌。
//
// 先验证用户凭证（邮箱/手机号 + 密码），验证通过后吊销该用户的所有有效令牌。
//
// 参数:
//   - ctx: 上下文对象
//   - username: 邮箱或手机号
//   - password: 明文密码
//
// 返回值:
//   - *xError.Error: 认证失败或操作过程中的错误
func (l *YggdrasilLogic) SignoutUser(ctx context.Context, username string, password string) *xError.Error {
	l.log.Info(ctx, "SignoutUser - 吊销用户所有游戏令牌")

	// 查找用户
	user, found, xErr := l.repo.userRepo.GetByEmail(ctx, nil, username)
	if xErr != nil {
		return xErr
	}
	if !found {
		user, found, xErr = l.repo.userRepo.GetByPhone(ctx, nil, username)
		if xErr != nil {
			return xErr
		}
	}
	if !found {
		// 恒定时间比较：即使用户不存在也执行 bcrypt 比较，防止时序侧信道泄露账号存在性
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"), []byte(password))
		return xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.GamePassword), []byte(password)); err != nil {
		return xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
	}

	// 吊销所有有效令牌
	if xErr := l.repo.gameTokenRepo.InvalidateAllByUserID(ctx, nil, user.ID); xErr != nil {
		return xErr
	}

	// 二次扫描兜底：防御并发窗口内新创建的令牌泄漏
	// 在认证完成和吊销执行之间的时间窗口内，用户可能正在执行 Authenticate（创建新令牌）
	// 或 Refresh（刷新旧令牌），导致部分令牌未被此次 Signout 吊销
	count, countErr := l.repo.gameTokenRepo.CountValidByUserID(ctx, nil, user.ID)
	if countErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("Signout 二次扫描查询失败: %s", countErr.ErrorMessage))
	} else if count > 0 {
		l.log.Warn(ctx, fmt.Sprintf("Signout 后检测到 %d 个残留有效令牌，执行二次吊销", count))
		if reErr := l.repo.gameTokenRepo.InvalidateAllByUserID(ctx, nil, user.ID); reErr != nil {
			l.log.Error(ctx, fmt.Sprintf("Signout 二次吊销失败: %s", reErr.ErrorMessage))
		}
	}

	return nil
}

// CreateGameToken 为指定用户创建新的游戏令牌。
//
// 生成随机的 accessToken（UUID 格式），若未提供 clientToken 则自动生成。
// 同时检查用户令牌数量限制，超出时自动吊销最旧令牌。
//
// 参数:
//   - ctx: 上下文对象
//   - userID: 用户 Snowflake ID
//   - clientToken: 客户端提供的令牌标识
//
// 返回值:
//   - *entity.GameToken: 创建的游戏令牌实体
//   - *xError.Error: 创建过程中的错误
func (l *YggdrasilLogic) CreateGameToken(ctx context.Context, userID xSnowflake.SnowflakeID, clientToken string) (*entity.GameToken, *xError.Error) {
	l.log.Info(ctx, "CreateGameToken - 创建游戏令牌")

	// 如果未提供 clientToken，自动生成
	if clientToken == "" {
		clientUUID, err := uuid.NewRandom()
		if err != nil {
			return nil, xError.NewError(ctx, xError.ServerInternalError, "生成客户端令牌失败", true, err)
		}
		clientToken = clientUUID.String()
	}

	// 生成 accessToken
	accessUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "生成访问令牌失败", true, err)
	}
	accessToken := accessUUID.String()

	// 构建令牌实体
	now := time.Now()
	token := &entity.GameToken{
		AccessToken: accessToken,
		ClientToken: clientToken,
		UserID:      userID,
		Status:      entity.GameTokenStatusValid,
		IssuedAt:    now,
		ExpiresAt:   now.Add(time.Duration(bConst.YggdrasilTokenExpireHours) * time.Hour),
	}

	// 通过事务协调层完成配额检查 + 超额吊销 + 创建（原子操作）
	return l.repo.gameTokenTxnRepo.CreateWithQuotaCheck(ctx, userID, token, bConst.YggdrasilTokenMaxPerUser)
}
