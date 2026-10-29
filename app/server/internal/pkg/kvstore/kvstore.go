package kvstore

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrKeyNotFound 查询不存在或已过期的 key 时返回
var ErrKeyNotFound = errors.New("key not found")

// kvChangeChannel 是 KV 变更通知的内部 channel 名。
const kvChangeChannel = "__kv_change__"

// KVChangeChannel 返回 KV 变更通知的 channel 名，供调用方 Subscribe 使用。
//
//	ch, _ := store.Subscribe(ctx, kvstore.KVChangeChannel())
func KVChangeChannel() string { return kvChangeChannel }

// Config KV 存储配置，NATS JetStream 是唯一后端。
type Config struct {
	NATSURL    string // NATS 连接地址，可由上层内嵌 NATS 提供
	NATSBucket string // NATS_BUCKET，默认 "orin_kv"
}

// Store 支持 TTL 的 KV 存储抽象。
// 唯一实现是 NATS JetStream KV。
type Store interface {
	// ── 基础 KV ─────────────────────────────────────

	// Get 获取 key 的值。不存在或已过期返回 ErrKeyNotFound。
	Get(ctx context.Context, key string) (string, error)

	// Set 设置 key 的值。ttl=0 表示不过期。
	Set(ctx context.Context, key, value string, ttl time.Duration) error

	// SetNX 仅在 key 不存在时设置。返回 (true, nil) 表示设置成功。
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)

	// Delete 删除单个 key。
	Delete(ctx context.Context, key string) error

	// Exists 检查 key 是否存在且未过期。
	Exists(ctx context.Context, key string) (bool, error)

	// ── 批量操作 ────────────────────────────────────

	// PrefixScan 返回所有以 prefix 开头且未过期的 key-value 对。
	PrefixScan(ctx context.Context, prefix string) (map[string]string, error)

	// DeletePrefix 删除所有以 prefix 开头的 key。
	DeletePrefix(ctx context.Context, prefix string) error

	// ── 原子计数器 ──────────────────────────────────

	// Incr 原子自增 1，返回自增后的值。key 不存在时初始化为 1。
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy 原子自增 delta（可为负数），返回自增后的值。
	IncrBy(ctx context.Context, key string, delta int64) (int64, error)

	// ── Pub/Sub ─────────────────────────────────────

	// Publish 向 channel 发布消息。
	Publish(ctx context.Context, channel string, payload []byte) error

	// Subscribe 订阅 channel，返回只读消息 channel。调用方必须 cancel ctx 来退订。
	Subscribe(ctx context.Context, channel string) (<-chan []byte, error)

	// ── 生命周期 ────────────────────────────────────

	// Close 释放连接和后台资源。
	Close() error
}

// New 根据配置创建 NATS KV Store，连接失败直接报错。
func New(cfg Config) (Store, error) {
	if cfg.NATSURL == "" {
		return nil, errors.New("kvstore: NATS URL is required")
	}
	if cfg.NATSBucket == "" {
		cfg.NATSBucket = "orin_kv"
	}
	store, err := NewNATSStore(cfg.NATSURL, cfg.NATSBucket)
	if err != nil {
		return nil, err
	}
	logrus.Infof("KV Store: NATS JetStream (%s / %s)", cfg.NATSURL, cfg.NATSBucket)
	return store, nil
}
