package handler

import (
	"fmt"
	"path/filepath"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiSync "github.com/frontleaves-mc/frontleaves-yggleaf/api/sync"
	"github.com/gin-gonic/gin"
)

// ==================== Mods Metadata ====================

// ModsMetadata 获取 mods 目录文件元数据
//
// @Summary     [公开] Mods 元数据
// @Description 扫描服务端 mods 目录下所有 .jar 文件，返回文件列表及 SHA-256 哈希
// @Tags        同步接口
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=apiSync.SyncMetadataResponse} "获取成功"
// @Failure     500 {object} xBase.BaseResponse "服务器内部错误"
// @Router      /sync/mods/metadata [GET]
func (h *SyncHandler) ModsMetadata(ctx *gin.Context) {
	h.log.Info(ctx, "ModsMetadata - 获取 mods 元数据")

	files, xErr := h.service.syncLogic.ScanMods()
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取成功", apiSync.SyncMetadataResponse{
		Files:     files,
		Total:     len(files),
		ScannedAt: time.Now(),
	})
}

// ==================== Config Metadata ====================

// ConfigMetadata 获取 config 目录文件元数据
//
// @Summary     [公开] Config 元数据
// @Description 递归扫描服务端 config 目录下所有文件，返回文件列表及 SHA-256 哈希
// @Tags        同步接口
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=apiSync.SyncMetadataResponse} "获取成功"
// @Failure     500 {object} xBase.BaseResponse "服务器内部错误"
// @Router      /sync/config/metadata [GET]
func (h *SyncHandler) ConfigMetadata(ctx *gin.Context) {
	h.log.Info(ctx, "ConfigMetadata - 获取 config 元数据")

	files, xErr := h.service.syncLogic.ScanConfig()
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取成功", apiSync.SyncMetadataResponse{
		Files:     files,
		Total:     len(files),
		ScannedAt: time.Now(),
	})
}

// ==================== Scripts Metadata ====================

// ScriptsMetadata 获取 scripts 目录文件元数据
//
// @Summary     [公开] Scripts 元数据
// @Description 扫描服务端 scripts 目录下所有文件，返回文件列表及 SHA-256 哈希
// @Tags        同步接口
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=apiSync.SyncMetadataResponse} "获取成功"
// @Failure     500 {object} xBase.BaseResponse "服务器内部错误"
// @Router      /sync/scripts/metadata [GET]
func (h *SyncHandler) ScriptsMetadata(ctx *gin.Context) {
	h.log.Info(ctx, "ScriptsMetadata - 获取 scripts 元数据")

	files, xErr := h.service.syncLogic.ScanScripts()
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取成功", apiSync.SyncMetadataResponse{
		Files:     files,
		Total:     len(files),
		ScannedAt: time.Now(),
	})
}

// ==================== File Download ====================

// Download 下载指定文件
//
// @Summary     [公开] 下载文件
// @Description 根据相对路径下载 mods 或 config 下的指定文件，支持流式传输
// @Tags        同步接口
// @Produce     application/octet-stream
// @Param       path query string true "文件相对路径（如 mods/jei-1.20.1.jar）"
// @Success     200 {file} file "文件流"
// @Failure     400 {object} xBase.BaseResponse "参数错误或非法路径"
// @Failure     404 {object} xBase.BaseResponse "文件不存在"
// @Router      /sync/download [GET]
func (h *SyncHandler) Download(ctx *gin.Context) {
	h.log.Info(ctx, "Download - 下载文件")

	relPath := ctx.Query("path")
	if relPath == "" {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "path 参数不能为空", true, nil))
		return
	}

	f, size, xErr := h.service.syncLogic.DownloadFile(relPath)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	defer f.Close()

	fileName := filepath.Base(relPath)
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	ctx.Header("Content-Length", fmt.Sprintf("%d", size))
	ctx.DataFromReader(200, size, "application/octet-stream", f, nil)
}
