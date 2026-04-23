package sync

import "time"

// FileMetadata 文件元数据。
type FileMetadata struct {
	Path string `json:"path"` // 相对路径，如 "mods/jei-1.20.1.jar"
	Name string `json:"name"` // 文件名，如 "jei-1.20.1.jar"
	Hash string `json:"hash"` // "sha256:<hex>"
	Size int64  `json:"size"` // 文件大小（字节）
}

// SyncMetadataResponse 元数据响应。
type SyncMetadataResponse struct {
	Files     []FileMetadata `json:"files"`
	Total     int            `json:"total"`
	ScannedAt time.Time      `json:"scanned_at"`
}
