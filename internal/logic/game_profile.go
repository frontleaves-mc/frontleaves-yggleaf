package logic

import (
	"context"
	"regexp"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/utility"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/utility/ctxutil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	gameProfileNameMinLength = 3
	gameProfileNameMaxLength = 16
)

var gameProfileNameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type gameProfileRepo struct {
	profile  *repository.GameProfileRepo
	quota    *repository.GameProfileQuotaRepo
	quotaLog *repository.GameProfileQuotaLogRepo
}

type GameProfileLogic struct {
	logic
	repo gameProfileRepo
}

func NewGameProfileLogic(ctx context.Context) *GameProfileLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)
	return &GameProfileLogic{
		logic: logic{
			db:  db,
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "GameProfileLogic"),
		},
		repo: gameProfileRepo{
			profile:  repository.NewGameProfileRepo(db),
			quota:    repository.NewGameProfileQuotaRepo(db),
			quotaLog: repository.NewGameProfileQuotaLogRepo(db),
		},
	}
}

func (l *GameProfileLogic) AddGameProfile(ctx *gin.Context, userID xSnowflake.SnowflakeID, name string) (*entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "AddGameProfile - 新增游戏档案")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	normalizedName, xErr := validateGameProfileName(ctx, name)
	if xErr != nil {
		return nil, xErr
	}

	profileUUID, err := uuid.NewV7()
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "生成游戏档案 UUID 失败", true, err)
	}

	profile := &entity.GameProfile{
		UserID: userID,
		UUID:   profileUUID,
		Name:   normalizedName,
	}

	var createdProfile *entity.GameProfile
	var bizErr *xError.Error

	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		quota, found, xErr := l.repo.quota.GetByUserID(ctx.Request.Context(), tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户游戏档案配额不存在", true)
			return bizErr
		}
		if quota.Used >= quota.Total {
			bizErr = xError.NewError(ctx, xError.ResourceExhausted, "游戏档案配额不足", true)
			return bizErr
		}

		uuidExisted, xErr := l.repo.profile.ExistsByUUID(ctx.Request.Context(), tx, profile.UUID.String())
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if uuidExisted {
			bizErr = xError.NewError(ctx, xError.DataConflict, "UUID 已存在", true)
			return bizErr
		}

		nameExisted, xErr := l.repo.profile.ExistsByNameExceptID(ctx.Request.Context(), tx, profile.Name, xSnowflake.SnowflakeID(0))
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if nameExisted {
			bizErr = xError.NewError(ctx, xError.DataConflict, "用户名已存在", true)
			return bizErr
		}

		createdProfile, xErr = l.repo.profile.Create(ctx.Request.Context(), tx, profile)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		beforeUsed := quota.Used
		afterUsed := quota.Used + 1
		xErr = l.repo.quota.UpdateUsed(ctx.Request.Context(), tx, quota.ID, afterUsed)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		_, xErr = l.repo.quotaLog.Create(
			ctx.Request.Context(),
			tx,
			userID,
			entityType.ObTypeAddGameProfile,
			1,
			beforeUsed,
			quota.Total,
			xUtil.Ptr(createdProfile.ID),
			xUtil.Ptr("创建游戏档案"),
		)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "新增游戏档案失败", true, err)
	}
	return createdProfile, nil
}

func (l *GameProfileLogic) ChangeUsername(ctx *gin.Context, userID xSnowflake.SnowflakeID, profileID xSnowflake.SnowflakeID, newName string) (*entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "ChangeUsername - 修改游戏档案用户名")

	profile, found, xErr := l.repo.profile.GetByIDAndUserID(ctx.Request.Context(), nil, profileID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏档案不存在", true)
	}

	normalizedName, xErr := validateGameProfileName(ctx, newName)
	if xErr != nil {
		return nil, xErr
	}
	if profile.Name == normalizedName {
		return profile, nil
	}

	nameExisted, xErr := l.repo.profile.ExistsByNameExceptID(ctx.Request.Context(), nil, normalizedName, profile.ID)
	if xErr != nil {
		return nil, xErr
	}
	if nameExisted {
		return nil, xError.NewError(ctx, xError.DataConflict, "用户名已存在", true)
	}

	updatedProfile, xErr := l.repo.profile.UpdateName(ctx.Request.Context(), nil, profile.ID, normalizedName)
	if xErr != nil {
		return nil, xErr
	}
	return updatedProfile, nil
}

func validateGameProfileName(ctx *gin.Context, name string) (string, *xError.Error) {
	normalizedName := strings.TrimSpace(name)
	if len(normalizedName) < gameProfileNameMinLength || len(normalizedName) > gameProfileNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效用户名长度：必须在 3-16 个字符之间", true)
	}
	if !gameProfileNameRegex.MatchString(normalizedName) {
		return "", xError.NewError(ctx, xError.ParameterError, "无效用户名格式：只允许字母、数字和下划线", true)
	}
	return normalizedName, nil
}
