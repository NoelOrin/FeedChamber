package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"
)

// PresignedPost 表示预签名 POST 上传所需的信息。
type PresignedPost struct {
	URL    string            `json:"url"`    // POST 目标地址
	Fields map[string]string `json:"fields"` // 必须附带的表单字段（含签名、policy 等）
}

// ObjectInfo describes an object returned by List.
type ObjectInfo struct {
	Key        string
	Size       int64
	ModifiedAt time.Time
}

// Provider 定义对象存储抽象层。
// 所有上传场景（本地磁盘 / S3 兼容存储）均通过此接口操作。
type Provider interface {
	// Put 将文件写入存储，返回存储键。
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Get 返回文件读取流。
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete 删除文件。
	Delete(ctx context.Context, key string) error

	// List 返回指定前缀下的对象元数据。
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)

	// PresignPostURL 生成预签名 POST 信息，供前端直传。
	PresignPostURL(ctx context.Context, key string, contentType string, maxSize int64, expires time.Duration) (*PresignedPost, error)

	// PublicURL 返回文件的公开访问地址（若有自定义域名则使用）。
	PublicURL(key string) string
}

// Config 存储配置
type Config struct {
	// Provider 类型：local | s3
	Provider string

	// Local 本地存储配置
	LocalDir string

	// S3 兼容存储配置
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	UseSSL          bool
	CustomDomain    string // 可选自定义域名/CDN域名，用于拼接公开URL
}

// New 根据配置创建对应的存储 Provider。
func New(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "", "local":
		return NewLocalStorage(cfg.LocalDir), nil
	case "s3":
		return NewS3Provider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}

// BuildKey 根据目录前缀和存储文件名构建对象键。
func BuildKey(dir, storedName string) string {
	if dir == "" {
		return storedName
	}
	return dir + "/" + storedName
}

// EncodeKey 对 key 进行 URL 编码，用于生成公开 URL。
func EncodeKey(key string) string {
	return (&url.URL{Path: key}).Path
}
