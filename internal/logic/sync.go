package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	apiSync "github.com/frontleaves-mc/frontleaves-yggleaf/api/sync"
)

const (
	baseDir             = "minecraft_client"
	modsSubDir          = "mods"
	configSubDir        = "config"
	scriptsSubDir       = "scripts"
	resourcepacksSubDir = "resourcepacks"
	shaderpacksSubDir  = "shaderpacks"
	taczSubDir          = "tacz"
	extendsSubDir       = "extends"
)

// SyncLogic 模组同步业务逻辑。
type SyncLogic struct {
	ctx      context.Context
	log      *xLog.LogNamedLogger
	basePath string
}

// NewSyncLogic 创建 SyncLogic 实例，自动确保 minecraft_client 目录存在。
func NewSyncLogic(ctx context.Context) *SyncLogic {
	l := &SyncLogic{
		ctx:      ctx,
		log:      xLog.WithName(xLog.NamedCONT, "SyncLogic"),
		basePath: baseDir,
	}
	l.ensureBaseDir()
	return l
}

// ensureBaseDir 确保 minecraft_client 目录存在。
func (l *SyncLogic) ensureBaseDir() {
	if err := os.MkdirAll(l.basePath, 0o755); err != nil {
		l.log.Error(l.ctx, fmt.Sprintf("创建 minecraft_client 目录失败: %v", err))
	}
}

// ScanMods 扫描 mods 目录下所有 .jar 文件，支持按 mode 筛选子目录。
// mode: "server" 扫描 mods/server，"client" 扫描 mods/client，"all" 合并扫描两者。
func (l *SyncLogic) ScanMods(mode string) ([]apiSync.FileMetadata, error) {
	var allFiles []apiSync.FileMetadata

	subDirs := l.modsSubDirs(mode)
	for _, sub := range subDirs {
		dirPath := filepath.Join(l.basePath, sub)
		files, err := l.scanDirectory(dirPath, sub, false)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
	}

	return allFiles, nil
}

// modsSubDirs 根据 mode 返回需要扫描的 mods 子目录列表。
func (l *SyncLogic) modsSubDirs(mode string) []string {
	switch mode {
	case "server":
		return []string{filepath.Join(modsSubDir, "server")}
	case "client":
		return []string{filepath.Join(modsSubDir, "client")}
	default: // "all" 或空
		return []string{
			filepath.Join(modsSubDir, "server"),
			filepath.Join(modsSubDir, "client"),
		}
	}
}

// ScanConfig 递归扫描 config 目录下所有文件。
func (l *SyncLogic) ScanConfig() ([]apiSync.FileMetadata, error) {
	configPath := filepath.Join(l.basePath, configSubDir)
	return l.scanDirectory(configPath, configSubDir, true)
}

// ScanScripts 扫描 scripts 目录下所有文件（不递归子目录）。
func (l *SyncLogic) ScanScripts() ([]apiSync.FileMetadata, error) {
	scriptsPath := filepath.Join(l.basePath, scriptsSubDir)
	return l.scanDirectory(scriptsPath, scriptsSubDir, false)
}

// ScanResourcepacks 递归扫描 resourcepacks 目录下所有文件。
func (l *SyncLogic) ScanResourcepacks() ([]apiSync.FileMetadata, error) {
	rpPath := filepath.Join(l.basePath, resourcepacksSubDir)
	return l.scanDirectory(rpPath, resourcepacksSubDir, true)
}

// ScanShaderpacks 递归扫描 shaderpacks 目录下所有文件。
func (l *SyncLogic) ScanShaderpacks() ([]apiSync.FileMetadata, error) {
	spPath := filepath.Join(l.basePath, shaderpacksSubDir)
	return l.scanDirectory(spPath, shaderpacksSubDir, true)
}

// ScanTacz 扫描 tacz 目录下所有 .zip 文件（不递归子目录）。
func (l *SyncLogic) ScanTacz() ([]apiSync.FileMetadata, error) {
	taczPath := filepath.Join(l.basePath, taczSubDir)
	return l.scanDirectory(taczPath, taczSubDir, false)
}

// ScanExtends 递归扫描 extends 目录下所有文件。
func (l *SyncLogic) ScanExtends() ([]apiSync.FileMetadata, error) {
	extPath := filepath.Join(l.basePath, extendsSubDir)
	return l.scanDirectory(extPath, extendsSubDir, true)
}

// scanDirectory 扫描指定目录，生成文件元数据列表。
func (l *SyncLogic) scanDirectory(dirPath, prefix string, recursive bool) ([]apiSync.FileMetadata, error) {
	var files []apiSync.FileMetadata

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, xError.NewError(l.ctx, xError.ServerInternalError, xError.ErrMessage(fmt.Sprintf("读取 %s 目录失败", prefix)), true, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if recursive {
				subFiles, err := l.scanDirectory(filepath.Join(dirPath, entry.Name()), filepath.Join(prefix, entry.Name()), true)
				if err != nil {
					return nil, err
				}
				files = append(files, subFiles...)
			}
			continue
		}

		// mods 目录及其子目录只处理 .jar 文件
		if strings.HasPrefix(prefix, modsSubDir) && !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			continue
		}

		// tacz 目录只处理 .zip 文件
		if strings.HasPrefix(prefix, taczSubDir) && !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		hash, err := l.computeFileHash(fullPath)
		if err != nil {
			continue
		}

		files = append(files, apiSync.FileMetadata{
			Path: filepath.Join(prefix, entry.Name()),
			Name: entry.Name(),
			Hash: "sha256:" + hash,
			Size: info.Size(),
		})
	}

	return files, nil
}

// computeFileHash 计算文件 SHA-256 哈希。
func (l *SyncLogic) computeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DownloadFile 根据相对路径打开文件并返回文件流。
func (l *SyncLogic) DownloadFile(relPath string) (*os.File, int64, error) {
	cleaned := filepath.Clean(relPath)
	if strings.Contains(cleaned, "..") {
		return nil, 0, xError.NewError(l.ctx, xError.ParameterError, "非法路径", true, nil)
	}

	if !strings.HasPrefix(cleaned, modsSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, configSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, scriptsSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, resourcepacksSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, shaderpacksSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, taczSubDir+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, extendsSubDir+string(filepath.Separator)) {
		return nil, 0, xError.NewError(l.ctx, xError.ParameterError, "路径必须以 mods/、config/、scripts/、resourcepacks/、shaderpacks/、tacz/ 或 extends/ 开头", true, nil)
	}

	fullPath := filepath.Join(l.basePath, cleaned)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, xError.NewError(l.ctx, xError.NotFound, "文件不存在", true, err)
		}
		return nil, 0, xError.NewError(l.ctx, xError.ServerInternalError, "读取文件失败", true, err)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, xError.NewError(l.ctx, xError.ServerInternalError, "打开文件失败", true, err)
	}

	return f, info.Size(), nil
}
