package logic

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	xAsync "github.com/bamboo-services/bamboo-base-go/plugins/async"
	xEmail "github.com/bamboo-services/bamboo-base-go/plugins/email"
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/models"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	repocache "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/cache"
	repotxn "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	bBucketApi "github.com/phalanx-labs/beacon-bucket-sdk/api"
	bBucket "github.com/phalanx-labs/beacon-bucket-sdk"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	bCtx "github.com/frontleaves-mc/frontleaves-yggleaf/pkg/context"
)

const (
	minReplyLength     = 1
	maxReplyLength     = 5000
	maxAdminNoteLength = 2000
	maxAttachments     = 9
)

// issueRepo 问题数据访问适配器。
//
// 聚合问题相关的各仓储实例，包括问题仓储、回复仓储、附件仓储、类型仓储，
// 缓存层以及事务协调仓储（TxnRepo），供 IssueLogic 统一调用。
type issueRepo struct {
	issueRepo      *repository.IssueRepo           // 问题主表仓储
	replyRepo      *repository.IssueReplyRepo      // 回复仓储
	attachmentRepo *repository.IssueAttachmentRepo // 附件仓储
	issueTypeRepo  *repository.IssueTypeRepo       // 类型仓储
	userRepo       *repository.UserRepo            // 用户仓储
	cache          *repocache.IssueCache           // Redis 缓存层
	txn            *repotxn.IssueTxnRepo           // 事务协调仓储
}

// issueHelper 问题外部服务辅助器。
//
// 封装对象存储（Bucket）客户端等外部依赖，用于处理附件上传等
// 不属于数据库事务范围的外部服务调用。
type issueHelper struct {
	bucket *bBucket.BucketClient // 对象存储客户端
}

// IssueLogic 问题业务逻辑处理者。
//
// 封装了问题反馈系统相关的核心业务逻辑，包括 CRUD、回复、管理员操作、
// 附件管理和类型管理等。通过嵌入匿名的 `logic` 结构体继承基础设施依赖，
// 通过 `issueRepo` 适配器调用 Repository 层完成数据持久化，
// 通过 `issueHelper` 完成对象存储等外部服务调用。
//
// 设计约束：本层不直接操作数据库事务，所有涉及多表写入的事务性操作
// 均委托给 Repository 层的 IssueTxnRepo 完成。对象存储上传在事务外执行。
type IssueLogic struct {
	logic
	repo   issueRepo
	helper issueHelper
}

// NewIssueLogic 创建问题业务逻辑实例。
//
// 初始化并返回 IssueLogic 结构体指针。从上下文中提取必需的依赖项
// （数据库连接、Redis 连接、对象存储客户端），并初始化所有关联的
// Repository 实例及事务协调仓储。
func NewIssueLogic(ctx context.Context) *IssueLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)

	issueRepoInst := repository.NewIssueRepo(db)
	replyRepo := repository.NewIssueReplyRepo(db)
	attachmentRepo := repository.NewIssueAttachmentRepo(db)
	issueTypeRepo := repository.NewIssueTypeRepo(db)
	userRepo := repository.NewUserRepo(db, rdb)

	return &IssueLogic{
		logic: logic{
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "IssueLogic"),
		},
		repo: issueRepo{
			issueRepo:      issueRepoInst,
			replyRepo:      replyRepo,
			attachmentRepo: attachmentRepo,
			issueTypeRepo:  issueTypeRepo,
			userRepo:       userRepo,
			cache:          &repocache.IssueCache{RDB: rdb, TTL: 15 * time.Minute},
			txn: repotxn.NewIssueTxnRepo(
				db, issueRepoInst, replyRepo, attachmentRepo,
			),
		},
		helper: issueHelper{
			bucket: bCtx.MustGetBucket(ctx),
		},
	}
}

// ==================== Bucket Helper Methods ====================

// cacheVerifyFile 将缓存态文件确认为永久态。
func (l *IssueLogic) cacheVerifyFile(ctx context.Context, fileId string) {
	_, err := l.helper.bucket.Normal.CacheVerify(ctx, &bBucketApi.CacheVerifyRequest{
		FileId: fileId,
	})
	if err != nil {
		l.log.Warn(ctx, fmt.Sprintf("CacheVerify 调用失败，文件可能仍为缓存态: %v", err))
	}
}

// resolveAttachmentURL 通过 beacon-bucket SDK 的 Get 方法将 FileID 解析为下载链接。
func (l *IssueLogic) resolveAttachmentURL(ctx context.Context, fileID int64) (string, *xError.Error) {
	fileIdStr := strconv.FormatInt(fileID, 10)
	resp, err := l.helper.bucket.Normal.Get(ctx, &bBucketApi.GetRequest{
		FileId: fileIdStr,
	})
	if err != nil {
		return "", xError.NewError(ctx, xError.ServerInternalError, "获取附件文件信息失败", true, err)
	}
	if resp.GetObj() == nil {
		return "", xError.NewError(ctx, xError.ServerInternalError, "附件文件元数据缺失", true)
	}
	link := resp.GetObj().GetLink()
	if link == "" {
		return "", xError.NewError(ctx, xError.ServerInternalError, "附件下载链接为空", true)
	}
	return link, nil
}

// resolveAttachmentURLsBatch 通过 beacon-bucket SDK 的 GetByList 接口批量解析附件下载链接。
func (l *IssueLogic) resolveAttachmentURLsBatch(ctx context.Context, fileIDs []int64) (map[int64]string, *xError.Error) {
	if len(fileIDs) == 0 {
		return make(map[int64]string), nil
	}

	fileIDList := make([]string, 0, len(fileIDs))
	for _, fid := range fileIDs {
		if fid == 0 {
			l.log.Warn(ctx, "resolveAttachmentURLsBatch 发现 fileID=0 的异常数据，将跳过该条目")
			continue
		}
		fileIDList = append(fileIDList, strconv.FormatInt(fid, 10))
	}

	if len(fileIDList) == 0 {
		return make(map[int64]string), nil
	}

	resp, err := l.helper.bucket.Normal.GetByList(ctx, &bBucketApi.GetByListRequest{
		FileIdList: fileIDList,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "批量获取附件文件信息失败", true, err)
	}

	fileInfoList := resp.GetFileInfoList()
	if len(fileInfoList) != len(fileIDList) {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "批量获取附件文件信息返回数量不匹配", true)
	}

	result := make(map[int64]string, len(fileInfoList))
	for i, info := range fileInfoList {
		if info.GetObj() == nil {
			return nil, xError.NewError(ctx, xError.ServerInternalError,
				xError.ErrMessage(fmt.Sprintf("附件文件元数据缺失(fileId=%s)", fileIDList[i])), true)
		}
		link := info.GetObj().GetLink()
		if link == "" {
			return nil, xError.NewError(ctx, xError.ServerInternalError,
				xError.ErrMessage(fmt.Sprintf("附件下载链接为空(fileId=%s)", fileIDList[i])), true)
		}
		originalFid, parseErr := strconv.ParseInt(fileIDList[i], 10, 64)
		if parseErr != nil {
			return nil, xError.NewError(ctx, xError.ServerInternalError, "解析附件文件 ID 失败", true, parseErr)
		}
		result[originalFid] = link
	}

	return result, nil
}

// deleteBucketFile 从对象存储中删除指定文件。
func (l *IssueLogic) deleteBucketFile(ctx context.Context, fileID int64) {
	fileId := strconv.FormatInt(fileID, 10)
	_, err := l.helper.bucket.Normal.Delete(ctx, &bBucketApi.DeleteRequest{
		FileId: fileId,
	})
	if err != nil {
		l.log.Warn(ctx, fmt.Sprintf("Bucket 文件删除失败(fileId=%s)，存在残留风险: %v", fileId, err))
	}
}

// decodeBase64Attachment 解码 Base64 编码的附件数据。
func (l *IssueLogic) decodeBase64Attachment(ctx context.Context, content string) ([]byte, *xError.Error) {
	base64Data := content
	if strings.HasPrefix(content, "data:") {
		idx := strings.Index(content, "base64,")
		if idx == -1 {
			return nil, xError.NewError(ctx, xError.ParameterError, "无效的 base64 MIME 格式：缺少 base64, 标记", true)
		}
		base64Data = content[idx+len("base64,"):]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效的 base64 附件数据", true, err)
	}
	return data, nil
}

// validateAttachmentMIME 校验附件 MIME 类型是否合法。
func (l *IssueLogic) validateAttachmentMIME(mimeType string) bool {
	allowedMIMEs := map[string]bool{
		"image/jpeg":                    true,
		"image/png":                     true,
		"image/gif":                     true,
		"image/webp":                    true,
		"application/pdf":               true,
		"text/plain":                    true,
		"application/zip":               true,
		"application/x-rar-compressed":  true,
		"application/x-rar":             true,
	}
	return allowedMIMEs[mimeType]
}

// ==================== Build Helpers ====================

// buildIssueDTO 将 Issue 实体转换为 IssueDTO。isAdmin 控制 AdminNote 是否暴露。
func (l *IssueLogic) buildIssueDTO(ctx context.Context, issue *entity.Issue, replyCount, attachmentCount int, isAdmin bool) (*models.IssueDTO, *xError.Error) {
	typeName := ""
	if issue.IssueType != nil {
		typeName = issue.IssueType.Name
	}
	username := ""
	if issue.User != nil {
		username = issue.User.Username
	}
	dto := &models.IssueDTO{
		ID:              issue.ID,
		UserID:          issue.UserID,
		Username:        username,
		IssueTypeID:     issue.IssueTypeID,
		IssueTypeName:   typeName,
		Title:           issue.Title,
		Content:         issue.Content,
		Status:          issue.Status,
		Priority:        issue.Priority,
		ClosedAt:        issue.ClosedAt,
		ReplyCount:      replyCount,
		AttachmentCount: attachmentCount,
		CreatedAt:       issue.CreatedAt,
		UpdatedAt:       issue.UpdatedAt,
	}
	if isAdmin {
		dto.AdminNote = issue.AdminNote
	}
	return dto, nil
}

// buildAttachmentDTOs 将 IssueAttachment 实体列表转换为 IssueAttachmentDTO 列表。
//
// 使用 GetByList 批量接口一次性解析所有附件下载链接，避免 N+1 RPC 问题。
func (l *IssueLogic) buildAttachmentDTOs(ctx context.Context, attachments []entity.IssueAttachment) ([]models.IssueAttachmentDTO, *xError.Error) {
	if len(attachments) == 0 {
		return []models.IssueAttachmentDTO{}, nil
	}

	fileIDs := make([]int64, len(attachments))
	for i, att := range attachments {
		fileIDs[i] = att.FileID
	}

	urlMap, xErr := l.resolveAttachmentURLsBatch(ctx, fileIDs)
	if xErr != nil {
		return nil, xErr
	}

	responses := make([]models.IssueAttachmentDTO, len(attachments))
	for i, att := range attachments {
		url, _ := urlMap[att.FileID]
		responses[i] = models.IssueAttachmentDTO{
			ID:       att.ID.Int64(),
			FileName: att.FileName,
			FileSize: att.FileSize,
			MimeType: att.MimeType,
			FileURL:  url,
		}
	}
	return responses, nil
}

// ==================== CRUD Methods ====================

// CreateIssue 创建问题反馈。
func (l *IssueLogic) CreateIssue(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	issueTypeID xSnowflake.SnowflakeID,
	title string,
	content string,
	priority bConst.IssuePriority,
) (*models.IssueDTO, *xError.Error) {
	l.log.Info(ctx, "CreateIssue - 创建问题")

	// 校验 IssueTypeID 存在且启用
	itType, found, xErr := l.repo.issueTypeRepo.GetByID(ctx, nil, issueTypeID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ParameterError, "问题类型不存在", true)
	}
	if !itType.IsEnabled {
		return nil, xError.NewError(ctx, xError.ParameterError, "该问题类型已禁用", true)
	}

	issue := &entity.Issue{
		UserID:      userID,
		IssueTypeID: issueTypeID,
		Title:       title,
		Content:     content,
		Priority:    priority,
	}
	created, xErr := l.repo.txn.CreateIssue(ctx, issue)
	if xErr != nil {
		return nil, xErr
	}
	// 写入缓存（失败仅 Warn）
	if setErr := l.repo.cache.Set(ctx, created.ID, issue); setErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("创建 Issue 后写缓存失败(id=%d): %v", created.ID, setErr))
	}
	// 异步通知管理员
	l.notifyIssueCreate(ctx, created)
	return l.buildIssueDTO(ctx, created, 0, 0, false)
}

// GetIssueList 获取当前用户的问题列表（分页，支持筛选）。
func (l *IssueLogic) GetIssueList(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	page int,
	pageSize int,
	status *bConst.IssueStatus,
	priority *bConst.IssuePriority,
	issueTypeID *xSnowflake.SnowflakeID,
) ([]models.IssueDTO, int64, *xError.Error) {
	l.log.Info(ctx, "GetIssueList - 获取用户问题列表")

	issues, total, xErr := l.repo.issueRepo.ListByUserID(ctx, userID, page, pageSize, status, priority, issueTypeID)
	if xErr != nil {
		return nil, 0, xErr
	}

	issueIDs := make([]xSnowflake.SnowflakeID, len(issues))
	for i, issue := range issues {
		issueIDs[i] = issue.ID
	}
	replyCountMap, xErr := l.repo.replyRepo.CountBatchByIssueIDs(ctx, issueIDs)
	if xErr != nil {
		return nil, 0, xErr
	}

	dtos := make([]models.IssueDTO, len(issues))
	for i, issue := range issues {
		replyCount := 0
		if c, ok := replyCountMap[issue.ID]; ok {
			replyCount = int(c)
		}
		dto, buildErr := l.buildIssueDTO(ctx, &issue, replyCount, 0, false)
		if buildErr != nil {
			return nil, 0, buildErr
		}
		dtos[i] = *dto
	}
	return dtos, total, nil
}

// GetIssueListAdmin 管理员全量分页查询问题列表。
func (l *IssueLogic) GetIssueListAdmin(
	ctx context.Context,
	page int,
	pageSize int,
	status *bConst.IssueStatus,
	priority *bConst.IssuePriority,
	issueTypeID *xSnowflake.SnowflakeID,
	keyword string,
	noFinal bool,
) ([]models.IssueDTO, int64, *xError.Error) {
	l.log.Info(ctx, "GetIssueListAdmin - 管理员查询问题列表")

	issues, total, xErr := l.repo.issueRepo.ListAdmin(ctx, page, pageSize, status, priority, issueTypeID, keyword, noFinal)
	if xErr != nil {
		return nil, 0, xErr
	}

	// 批量查询回复数量，避免 N+1
	issueIDs := make([]xSnowflake.SnowflakeID, len(issues))
	for i, issue := range issues {
		issueIDs[i] = issue.ID
	}
	replyCountMap, xErr := l.repo.replyRepo.CountBatchByIssueIDs(ctx, issueIDs)
	if xErr != nil {
		return nil, 0, xErr
	}

	dtos := make([]models.IssueDTO, len(issues))
	for i, issue := range issues {
		replyCount := 0
		if c, ok := replyCountMap[issue.ID]; ok {
			replyCount = int(c)
		}
		dto, buildErr := l.buildIssueDTO(ctx, &issue, replyCount, 0, true)
		if buildErr != nil {
			return nil, 0, buildErr
		}
		dtos[i] = *dto
	}
	return dtos, total, nil
}

// GetIssueDetail 获取问题详情（含回复和附件）。
func (l *IssueLogic) GetIssueDetail(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	userID xSnowflake.SnowflakeID,
	isAdmin bool,
) (*models.IssueDetailDTO, *xError.Error) {
	l.log.Info(ctx, "GetIssueDetail - 获取问题详情")

	// 优先读缓存
	var issue *entity.Issue
	cachedIssue, cacheErr := l.repo.cache.Get(ctx, issueID)
	if cacheErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("读取 Issue 缓存失败(id=%d): %v", issueID, cacheErr))
		// 缓存失败降级到 DB 查询
	}
	if cachedIssue != nil {
		issue = cachedIssue
	} else {
		found := false
		var xErr *xError.Error
		issue, found, xErr = l.repo.issueRepo.GetByID(ctx, nil, issueID)
		if xErr != nil {
			return nil, xErr
		}
		if !found {
			return nil, xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
		}
		// 回写缓存（失败仅 Warn）
		if setErr := l.repo.cache.Set(ctx, issueID, issue); setErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("写入 Issue 缓存失败(id=%d): %v", issueID, setErr))
		}
	}

	// 补充 IssueType 关联（缓存和 GetByID 均不携带关联数据）
	if issue.IssueType == nil && issue.IssueTypeID != 0 {
		it, found, xErr := l.repo.issueTypeRepo.GetByID(ctx, nil, issue.IssueTypeID)
		if xErr == nil && found {
			issue.IssueType = it
		}
	}

	// 权限校验：非管理员只能查看自己的问题
	if !isAdmin && issue.UserID != userID {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "无权查看该问题", true)
	}

	// 查询回复列表
	replies, replyTotal, xErr := l.repo.replyRepo.ListByIssueID(ctx, issueID, 1, 20)
	if xErr != nil {
		return nil, xErr
	}
	replyDTOs := make([]models.IssueReplyDTO, len(replies))
	for i, r := range replies {
		username := ""
		if r.User != nil {
			username = r.User.Username
		}
		replyDTOs[i] = models.IssueReplyDTO{
			ID:           r.ID,
			IssueID:      r.IssueID,
			UserID:       r.UserID,
			Username:     username,
			Content:      r.Content,
			IsAdminReply: r.IsAdminReply,
			CreatedAt:    r.CreatedAt,
		}
	}

	// 查询附件列表
	attachments, xErr := l.repo.attachmentRepo.ListByIssueID(ctx, issueID)
	if xErr != nil {
		return nil, xErr
	}
	attDTOs, xErr := l.buildAttachmentDTOs(ctx, attachments)
	if xErr != nil {
		return nil, xErr
	}

	issueDTO, xErr := l.buildIssueDTO(ctx, issue, int(replyTotal), len(attDTOs), isAdmin)
	if xErr != nil {
		return nil, xErr
	}

	var issueType entity.IssueType
	if issue.IssueType != nil {
		issueType = *issue.IssueType
	}

	return &models.IssueDetailDTO{
		Issue:       *issueDTO,
		IssueType:   issueType,
		Replies:     replyDTOs,
		Attachments: attDTOs,
	}, nil
}

// ==================== Reply Method ====================

// ReplyIssue 追加回复。
func (l *IssueLogic) ReplyIssue(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	userID xSnowflake.SnowflakeID,
	content string,
	isAdmin bool,
) (*models.IssueReplyDTO, *xError.Error) {
	l.log.Info(ctx, "ReplyIssue - 追加回复")

	if len(content) < minReplyLength || len(content) > maxReplyLength {
		return nil, xError.NewError(ctx, xError.ParameterError,
			xError.ErrMessage(fmt.Sprintf("回复内容长度必须在 %d~%d 字符之间", minReplyLength, maxReplyLength)), true)
	}

	// 权限校验：非管理员只能回复自己的问题
	if !isAdmin {
		_, found, xErr := l.repo.issueRepo.GetByIDAndUserID(ctx, nil, issueID, userID)
		if xErr != nil {
			return nil, xErr
		}
		if !found {
			return nil, xError.NewError(ctx, xError.ParameterError, "问题不存在或无权操作", true)
		}
	} else {
		_, found, xErr := l.repo.issueRepo.GetByID(ctx, nil, issueID)
		if xErr != nil {
			return nil, xErr
		}
		if !found {
			return nil, xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
		}
	}

	reply := &entity.IssueReply{
		IssueID:      issueID,
		UserID:       userID,
		Content:      content,
		IsAdminReply: isAdmin,
	}

	created, xErr := l.repo.txn.CreateReplyAndUpdateTimestamp(ctx, reply, issueID)
	if xErr != nil {
		return nil, xErr
	}

	// 清除缓存（回复更新了 issue.updated_at）
	if delErr := l.repo.cache.Del(ctx, issueID); delErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("删除 Issue 缓存失败(id=%d): %v", issueID, delErr))
	}


	var replyUser *entity.User
	if u, found, _ := l.repo.userRepo.Get(ctx, userID.String()); found {
		replyUser = u
	}

	username := ""
	if replyUser != nil {
		username = replyUser.Username
	}

	// 异步通知回复
	if replyUser != nil {
		l.notifyIssueReply(ctx, issueID, replyUser, content, isAdmin)
	}

	return &models.IssueReplyDTO{
		ID:           created.ID,
		IssueID:      created.IssueID,
		UserID:       created.UserID,
		Username:     username,
		Content:      created.Content,
		IsAdminReply: created.IsAdminReply,
		CreatedAt:    created.CreatedAt,
	}, nil
}

// ==================== Admin Operations ====================

// UpdateStatus 修改问题状态（含流转校验）。
func (l *IssueLogic) UpdateStatus(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	targetStatus bConst.IssueStatus,
) *xError.Error {
	l.log.Info(ctx, "UpdateStatus - 修改问题状态")

	issue, found, xErr := l.repo.issueRepo.GetByID(ctx, nil, issueID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
	}

	// 记录原状态
	oldStatus := issue.Status

	if !issue.Status.IsValidTransition(targetStatus) {
		return xError.NewError(ctx, xError.ParameterError,
			xError.ErrMessage(fmt.Sprintf("不允许从 [%s] 转换到 [%s]", issue.Status, targetStatus)), true)
	}

	result := l.repo.txn.UpdateStatusWithCloseTime(ctx, issueID, targetStatus)
	// 清除缓存（状态变更）
	if result == nil {
		if delErr := l.repo.cache.Del(ctx, issueID); delErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("删除 Issue 缓存失败(id=%d): %v", issueID, delErr))
		}
		// 异步通知状态变更
		l.notifyIssueStatus(ctx, issue, oldStatus, targetStatus)
	}
	return result
}

// UpdatePriority 修改优先级。
func (l *IssueLogic) UpdatePriority(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	priority bConst.IssuePriority,
) *xError.Error {
	l.log.Info(ctx, "UpdatePriority - 修改优先级")

	_, found, xErr := l.repo.issueRepo.GetByID(ctx, nil, issueID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
	}

	result := l.repo.issueRepo.UpdatePriority(ctx, nil, issueID, priority)
	// 清除缓存（优先级变更）
	if result == nil {
		if delErr := l.repo.cache.Del(ctx, issueID); delErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("删除 Issue 缓存失败(id=%d): %v", issueID, delErr))
		}
	}
	return result
}

// UpdateNote 更新内部备注。
func (l *IssueLogic) UpdateNote(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	note string,
) *xError.Error {
	l.log.Info(ctx, "UpdateNote - 更新内部备注")

	if len(note) > maxAdminNoteLength {
		return xError.NewError(ctx, xError.ParameterError,
			xError.ErrMessage(fmt.Sprintf("内部备注长度不能超过 %d 字符", maxAdminNoteLength)), true)
	}

	_, found, xErr := l.repo.issueRepo.GetByID(ctx, nil, issueID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
	}

	result := l.repo.issueRepo.UpdateAdminNote(ctx, nil, issueID, note)
	// 清除缓存（备注变更）
	if result == nil {
		if delErr := l.repo.cache.Del(ctx, issueID); delErr != nil {
			l.log.Warn(ctx, fmt.Sprintf("删除 Issue 缓存失败(id=%d): %v", issueID, delErr))
		}
	}
	return result
}

// ==================== Attachment Upload/Delete ====================

// UploadAttachment 上传附件（完全对齐 LibraryLogic.UploadSkin 模式）。
func (l *IssueLogic) UploadAttachment(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	userID xSnowflake.SnowflakeID,
	isAdmin bool,
	fileName string,
	content string,
	mimeType string,
) (*models.IssueAttachmentDTO, *xError.Error) {
	l.log.Info(ctx, "UploadAttachment - 上传附件")

	issue, found, xErr := l.repo.issueRepo.GetByID(ctx, nil, issueID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ParameterError, "问题不存在", true)
	}

	// 权限校验：非管理员只能给自己的问题上传附件
	if !isAdmin && issue.UserID != userID {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "无权向该问题上传附件", true)
	}

	count, xErr := l.repo.attachmentRepo.CountByIssueID(ctx, issueID)
	if xErr != nil {
		return nil, xErr
	}
	if int(count) >= maxAttachments {
		return nil, xError.NewError(ctx, xError.ResourceExhausted,
			xError.ErrMessage(fmt.Sprintf("单问题最多上传 %d 个附件", maxAttachments)), true)
	}

	data, xErr := l.decodeBase64Attachment(ctx, content)
	if xErr != nil {
		return nil, xErr
	}

	// 文件大小限制：单文件不超过 10MB
	const maxAttachmentSize = 10 * 1024 * 1024
	if int64(len(data)) > maxAttachmentSize {
		return nil, xError.NewError(ctx, xError.ParameterError, "文件大小不能超过 10MB", true)
	}

	if !l.validateAttachmentMIME(mimeType) {
		return nil, xError.NewError(ctx, xError.ParameterError, xError.ErrMessage("不支持的文件类型: "+mimeType), true)
	}

	// 校验 Issue 附件 Bucket 配置
	issueBucketId := xEnv.GetEnvString(bConst.EnvBucketIssueBucketId, "")
	issuePathId := xEnv.GetEnvString(bConst.EnvBucketIssuePathId, "")
	if issueBucketId == "" || issuePathId == "" {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "Issue 附件存储配置缺失，请联系管理员", true)
	}

	uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
		BucketId:      issueBucketId,
		PathId:        issuePathId,
		ContentBase64: content,
	})
	if err != nil {
		return nil, mapBucketError(ctx, "上传附件失败", err)
	}

	fileID, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "解析文件 ID 失败", true, err)
	}

	attachment := &entity.IssueAttachment{
		IssueID:  issueID,
		FileID:   fileID,
		FileName: fileName,
		FileSize: int64(len(data)),
		MimeType: mimeType,
	}

	created, xErr := l.repo.attachmentRepo.Create(ctx, nil, attachment)
	if xErr != nil {
		return nil, xErr
	}

	l.cacheVerifyFile(ctx, uploadResp.FileId)

	var fileURL string
	if uploadResp.GetObj() != nil && uploadResp.GetObj().GetLink() != "" {
		fileURL = uploadResp.GetObj().GetLink()
	} else {
		fileURL, xErr = l.resolveAttachmentURL(ctx, created.FileID)
		if xErr != nil {
			return nil, xErr
		}
	}

	return &models.IssueAttachmentDTO{
		ID:       created.ID.Int64(),
		FileName: created.FileName,
		FileSize: created.FileSize,
		MimeType: created.MimeType,
		FileURL:  fileURL,
	}, nil
}

// DeleteAttachment 删除附件（含完整权限校验）。
func (l *IssueLogic) DeleteAttachment(
	ctx context.Context,
	attachmentID xSnowflake.SnowflakeID,
	userID xSnowflake.SnowflakeID,
	isAdmin bool,
) *xError.Error {
	l.log.Info(ctx, "DeleteAttachment - 删除附件")

	att, found, xErr := l.repo.attachmentRepo.GetByID(ctx, nil, attachmentID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ParameterError, "附件不存在", true)
	}

	// 通过关联的问题 ID 校验归属权
	issue, issueFound, xErr := l.repo.issueRepo.GetByID(ctx, nil, att.IssueID)
	if xErr != nil {
		return xErr
	}
	if !issueFound {
		return xError.NewError(ctx, xError.ParameterError, "关联问题不存在", true)
	}
	if !isAdmin && issue.UserID != userID {
		return xError.NewError(ctx, xError.PermissionDenied, "无权删除该附件", true)
	}

	if xErr := l.repo.attachmentRepo.DeleteByID(ctx, nil, attachmentID); xErr != nil {
		return xErr
	}

	l.deleteBucketFile(ctx, att.FileID)
	// 清除缓存（附件变更）
	if delErr := l.repo.cache.Del(ctx, att.IssueID); delErr != nil {
		l.log.Warn(ctx, fmt.Sprintf("删除 Issue 缓存失败(id=%d): %v", att.IssueID, delErr))
	}
	return nil
}

// ==================== Type Management ====================

// ListIssueTypes 获取所有启用的类型列表（公开接口）。
func (l *IssueLogic) ListIssueTypes(ctx context.Context) ([]models.IssueTypeDTO, *xError.Error) {
	l.log.Info(ctx, "ListIssueTypes - 获取启用的类型列表")

	list, xErr := l.repo.issueTypeRepo.ListEnabled(ctx)
	if xErr != nil {
		return nil, xErr
	}

	dtos := make([]models.IssueTypeDTO, len(list))
	for i, it := range list {
		dtos[i] = models.IssueTypeDTO{
			ID:          it.ID,
			Name:        it.Name,
			Description: it.Description,
			SortOrder:   it.SortOrder,
			IsEnabled:   it.IsEnabled,
		}
	}
	return dtos, nil
}

// CreateIssueType 创建问题类型（管理员）。
func (l *IssueLogic) CreateIssueType(ctx context.Context, name string, description string, sortOrder int) (*models.IssueTypeDTO, *xError.Error) {
	l.log.Info(ctx, "CreateIssueType - 创建问题类型")

	it := &entity.IssueType{
		Name:        name,
		Description: description,
		SortOrder:   sortOrder,
		IsEnabled:   true,
	}

	created, xErr := l.repo.issueTypeRepo.Create(ctx, nil, it)
	if xErr != nil {
		return nil, xErr
	}

	return &models.IssueTypeDTO{
		ID:          created.ID,
		Name:        created.Name,
		Description: created.Description,
		SortOrder:   created.SortOrder,
		IsEnabled:   created.IsEnabled,
	}, nil
}

// UpdateIssueType 编辑问题类型（管理员）。
func (l *IssueLogic) UpdateIssueType(
	ctx context.Context,
	id xSnowflake.SnowflakeID,
	name *string,
	description *string,
	sortOrder *int,
	isEnabled *bool,
) (*models.IssueTypeDTO, *xError.Error) {
	l.log.Info(ctx, "UpdateIssueType - 编辑问题类型")

	it, found, xErr := l.repo.issueTypeRepo.GetByID(ctx, nil, id)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ParameterError, "问题类型不存在", true)
	}

	if name != nil {
		it.Name = *name
	}
	if description != nil {
		it.Description = *description
	}
	if sortOrder != nil {
		it.SortOrder = *sortOrder
	}
	if isEnabled != nil {
		it.IsEnabled = *isEnabled
	}

	updated, xErr := l.repo.issueTypeRepo.Update(ctx, nil, it)
	if xErr != nil {
		return nil, xErr
	}

	return &models.IssueTypeDTO{
		ID:          updated.ID,
		Name:        updated.Name,
		Description: updated.Description,
		SortOrder:   updated.SortOrder,
		IsEnabled:   updated.IsEnabled,
	}, nil
}

// DeleteIssueType 删除问题类型（管理员）。
func (l *IssueLogic) DeleteIssueType(ctx context.Context, id xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteIssueType - 删除问题类型")

	return l.repo.issueTypeRepo.DeleteByID(ctx, nil, id)
}

// ==================== Notification Helper Methods ====================

// notifyIssueCreate 异步通知管理员有新问题创建。
func (l *IssueLogic) notifyIssueCreate(ctx context.Context, issue *entity.Issue) {
	xAsync.Async(ctx, func(asyncCtx context.Context) {
		// 获取管理员邮箱列表
		db := xCtxUtil.MustGetDB(asyncCtx)
		rdb := xCtxUtil.MustGetRDB(asyncCtx)
		userRepo := repository.NewUserRepo(db, rdb)

		emails, xErr := userRepo.ListAdminEmails(asyncCtx)
		if xErr != nil || len(emails) == 0 {
			return
		}

		// 获取创建者信息
		issueRepo := repository.NewIssueRepo(db)
		_, found, _ := issueRepo.GetByID(asyncCtx, nil, issue.ID)

		username := ""
		issueTypeName := ""
		if found {
			// 需要单独查询 User 和 IssueType（GetByID 没有 Preload）
			user, userFound, _ := userRepo.Get(asyncCtx, issue.UserID.String())
			if userFound {
				username = user.Username
			}
			issueTypeRepo := repository.NewIssueTypeRepo(db)
			it, itFound, _ := issueTypeRepo.GetByID(asyncCtx, nil, issue.IssueTypeID)
			if itFound {
				issueTypeName = it.Name
			}
		}

		// 构建前端 URL
		frontendURL := xEnv.GetEnvString(bConst.EnvFrontendURL, "")
		issueURL := frontendURL + "/user/issues/" + issue.ID.String()

		// 发送邮件
		emailClient := xCtxUtil.MustGetEmailClient(asyncCtx)
		err := emailClient.SendTemplate(asyncCtx, &xEmail.Message{
			To:       emails,
			Subject:  "新问题反馈: " + issue.Title,
			Template: "issue_create",
			TemplateData: map[string]string{
				"Title":     issue.Title,
				"Username":  username,
				"IssueType": issueTypeName,
				"Priority":  string(issue.Priority),
				"IssueURL":  issueURL,
			},
		})
		if err != nil {
			l.log.Warn(asyncCtx, fmt.Sprintf("通知管理员新问题失败(id=%d): %v", issue.ID, err))
		}
	})
}

// notifyIssueReply 异步通知相关方有新回复。
func (l *IssueLogic) notifyIssueReply(ctx context.Context, issueID xSnowflake.SnowflakeID, replyUser *entity.User, content string, isAdminReply bool) {
	xAsync.Async(ctx, func(asyncCtx context.Context) {
		db := xCtxUtil.MustGetDB(asyncCtx)
		rdb := xCtxUtil.MustGetRDB(asyncCtx)

		// 获取 Issue 信息
		issueRepo := repository.NewIssueRepo(db)
		issue, found, xErr := issueRepo.GetByID(asyncCtx, nil, issueID)
		if xErr != nil || !found {
			return
		}

		var targetEmail string

		if isAdminReply {
			// 管理员回复 → 通知 Issue 作者（玩家）
			userRepo := repository.NewUserRepo(db, rdb)
			user, userFound, _ := userRepo.Get(asyncCtx, issue.UserID.String())
			if !userFound || user.Email == nil {
				return
			}
			targetEmail = *user.Email
		} else {
			// 玩家回复 → 通知最后回复的管理员
			replyRepo := repository.NewIssueReplyRepo(db)
			adminReply, adminFound, _ := replyRepo.GetLatestAdminReply(asyncCtx, issueID)
			if !adminFound || adminReply.User == nil || adminReply.User.Email == nil {
				return
			}
			targetEmail = *adminReply.User.Email
		}

		// 构建前端 URL
		frontendURL := xEnv.GetEnvString(bConst.EnvFrontendURL, "")
		issueURL := frontendURL + "/user/issues/" + issueID.String()

		// 截断回复内容（200 字符）
		displayContent := content
		runes := []rune(content)
		if len(runes) > 200 {
			displayContent = string(runes[:200]) + "..."
		}

		// 发送邮件
		emailClient := xCtxUtil.MustGetEmailClient(asyncCtx)
		err := emailClient.SendTemplate(asyncCtx, &xEmail.Message{
			To:       []string{targetEmail},
			Subject:  "问题回复通知: " + issue.Title,
			Template: "issue_reply",
			TemplateData: map[string]string{
				"Title":        issue.Title,
				"ReplyUser":    replyUser.Username,
				"Content":      displayContent,
				"IsAdminReply": strconv.FormatBool(isAdminReply),
				"IssueURL":     issueURL,
			},
		})
		if err != nil {
			l.log.Warn(asyncCtx, fmt.Sprintf("通知回复失败(issueID=%d): %v", issueID, err))
		}
	})
}

// notifyIssueStatus 异步通知 Issue 作者状态变更。
func (l *IssueLogic) notifyIssueStatus(ctx context.Context, issue *entity.Issue, oldStatus, newStatus bConst.IssueStatus) {
	xAsync.Async(ctx, func(asyncCtx context.Context) {
		db := xCtxUtil.MustGetDB(asyncCtx)
		rdb := xCtxUtil.MustGetRDB(asyncCtx)

		// 获取 Issue 作者的邮箱
		userRepo := repository.NewUserRepo(db, rdb)
		user, found, _ := userRepo.Get(asyncCtx, issue.UserID.String())
		if !found || user.Email == nil {
			return
		}

		// 构建前端 URL
		frontendURL := xEnv.GetEnvString(bConst.EnvFrontendURL, "")
		issueURL := frontendURL + "/user/issues/" + issue.ID.String()

		// 发送邮件
		emailClient := xCtxUtil.MustGetEmailClient(asyncCtx)
		err := emailClient.SendTemplate(asyncCtx, &xEmail.Message{
			To:       []string{*user.Email},
			Subject:  "问题状态变更: " + issue.Title,
			Template: "issue_status",
			TemplateData: map[string]string{
				"Title":     issue.Title,
				"OldStatus": oldStatus.ChineseName(),
				"NewStatus": newStatus.ChineseName(),
				"IssueURL":  issueURL,
			},
		})
		if err != nil {
			l.log.Warn(asyncCtx, fmt.Sprintf("通知状态变更失败(issueID=%d): %v", issue.ID, err))
		}
	})
}
