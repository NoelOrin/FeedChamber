package kvstore

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"feedchamber/internal/metrics"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
)

// natsStore implements Store backed by NATS JetStream KV bucket.
//
// TTL strategy: NATS KV supports per-bucket MaxAge but not per-key TTL.
// To support per-key TTL we use a companion bucket ("{bucket}_ttl") that
// stores the absolute expiry (Unix seconds) for each key. Reads check the
// companion entry first; if the key is expired both entries are purged.
//
// This is safer than the previous "__ttl_" prefix approach because:
//   - The companion bucket is a separate namespace — no risk of collision
//     with user keys or the WatchAll loop.
//   - Get/Exists read the value entry first, then check TTL — if the value
//     is missing we return ErrKeyNotFound immediately (no TOCTOU window
//     between "TTL present" and "value read").
//   - PrefixScan filters out expired keys by checking the TTL companion.
type natsStore struct {
	nc          *nats.Conn
	js          jetstream.JetStream
	kv          jetstream.KeyValue
	ttlKV       jetstream.KeyValue // companion bucket for per-key expiry
	watcher     jetstream.KeyWatcher
	watchCtx    context.Context
	cancelWatch context.CancelFunc
	wg          sync.WaitGroup
}

// encodeNATSKey 将业务 key 编码为 JetStream KV 合法 key。
// JetStream KV 只接受字母、数字、-、_、.，因此使用十六进制保存原始 key。
func encodeNATSKey(key string) string {
	return hex.EncodeToString([]byte(key))
}

func decodeNATSKey(encoded string) (string, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid encoded nats key %q: %w", encoded, err)
	}
	return string(raw), nil
}

// KV operation names — the label vocabulary of
// feedchamber_kv_operation_duration_seconds{op}. Constants plus one list so the
// observe() call sites and the startup seeding cannot drift apart.
const (
	opGet          = "get"
	opSet          = "set"
	opSetNX        = "setnx"
	opDelete       = "delete"
	opExists       = "exists"
	opPrefixScan   = "prefix_scan"
	opDeletePrefix = "delete_prefix"
	opIncr         = "incr"
)

// observedOps is every op name that observe() can record.
var observedOps = []string{
	opGet, opSet, opSetNX, opDelete, opExists, opPrefixScan, opDeletePrefix, opIncr,
}

// NewNATSStore creates a NATS KV Store.
func NewNATSStore(url, bucket string) (Store, error) {
	nc, err := nats.Connect(url,
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.Timeout(5*time.Second),
		// Connection state is a metric, not a log line: with MaxReconnects(-1)
		// a partition never fails the process, so without these handlers the
		// only evidence was a log the operator had to be watching for.
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			metrics.ObserveNATSDisconnected(metrics.NATSClientKV)
			logrus.WithError(err).Warn("kvstore: NATS disconnected")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			metrics.ObserveNATSReconnected(metrics.NATSClientKV)
			logrus.Infof("kvstore: NATS reconnected to %s", c.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	metrics.SeedNATSClient(metrics.NATSClientKV)

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Main KV bucket — MaxAge=0 means keys never expire at the bucket level;
	// per-key TTL is managed via the companion bucket.
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       bucket,
		Description:  "Feed Chamber KV Store",
		MaxValueSize: 1 << 20,
	})
	if err != nil {
		kv, err = js.KeyValue(ctx, bucket)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("nats kv bucket %q: %w", bucket, err)
		}
	}

	// Companion TTL bucket — stores absolute expiry timestamps as decimal strings.
	ttlBucket := bucket + "_ttl"
	ttlKV, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       ttlBucket,
		Description:  "Feed Chamber KV TTL companion",
		MaxValueSize: 32, // only stores Unix-second timestamps
	})
	if err != nil {
		ttlKV, err = js.KeyValue(ctx, ttlBucket)
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("nats kv ttl bucket %q: %w", ttlBucket, err)
		}
	}

	s := &natsStore{
		nc:    nc,
		js:    js,
		kv:    kv,
		ttlKV: ttlKV,
	}

	if err := s.startWatcher(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats kv watch: %w", err)
	}

	seedOperationMetrics()

	return s, nil
}

// ── Watcher ──────────────────────────────────────────

func (s *natsStore) startWatcher() error {
	watchCtx, cancel := context.WithCancel(context.Background())
	s.watchCtx = watchCtx
	s.cancelWatch = cancel
	w, err := s.kv.WatchAll(watchCtx)
	if err != nil {
		cancel()
		return err
	}
	s.watcher = w

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for entry := range w.Updates() {
			if entry == nil {
				continue
			}
			encodedKey := entry.Key()
			key, err := decodeNATSKey(encodedKey)
			if err != nil {
				continue
			}
			var op string
			switch entry.Operation() {
			case jetstream.KeyValuePut:
				op = "insert"
			case jetstream.KeyValueDelete, jetstream.KeyValuePurge:
				op = "delete"
			default:
				continue
			}
			payload, _ := json.Marshal(map[string]string{
				"op": op, "key": key, "value": string(entry.Value()),
			})
			// Publish via NATS core pub/sub so all instances receive it.
			_ = s.nc.Publish(natsChannelPrefix+kvChangeChannel, payload)
		}
	}()

	return nil
}

// ── TTL helpers ──────────────────────────────────────

// isExpired checks the TTL companion bucket. Returns (expired bool).
// If expired, both the main key and its TTL marker are purged.
func (s *natsStore) isExpired(ctx context.Context, key string) bool {
	encodedKey := encodeNATSKey(key)
	ttlEntry, err := s.ttlKV.Get(ctx, encodedKey)
	if err != nil {
		return false // no TTL marker → never expires
	}
	expiresAt, err := strconv.ParseInt(string(ttlEntry.Value()), 10, 64)
	if err != nil || expiresAt == 0 {
		return false
	}
	if time.Now().Unix() >= expiresAt {
		// Purge both entries; errors are non-fatal — the key is logically
		// expired regardless of whether the physical delete succeeds.
		_ = s.kv.Delete(ctx, encodedKey)
		_ = s.ttlKV.Delete(ctx, encodedKey)
		return true
	}
	return false
}

// setTTLMarker writes (or clears) the TTL companion entry for a key.
func (s *natsStore) setTTLMarker(ctx context.Context, key string, ttl time.Duration) error {
	encodedKey := encodeNATSKey(key)
	if ttl > 0 {
		expiresAt := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
		_, err := s.ttlKV.Put(ctx, encodedKey, []byte(expiresAt))
		return err
	}
	// No TTL — remove any stale marker.
	_ = s.ttlKV.Delete(ctx, encodedKey)
	return nil
}

// ── 基础 KV ──────────────────────────────────────────

func (s *natsStore) Get(ctx context.Context, key string) (val string, err error) {
	defer observe(opGet, &err)()

	// Check expiry first — avoids returning stale values.
	if s.isExpired(ctx, key) {
		return "", ErrKeyNotFound
	}

	entry, err := s.kv.Get(ctx, encodeNATSKey(key))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("nats get %q: %w", key, err)
	}
	return string(entry.Value()), nil
}

func (s *natsStore) Set(ctx context.Context, key, value string, ttl time.Duration) (err error) {
	defer observe(opSet, &err)()

	encodedKey := encodeNATSKey(key)
	if _, err := s.kv.Put(ctx, encodedKey, []byte(value)); err != nil {
		return fmt.Errorf("nats set %q: %w", key, err)
	}
	if err := s.setTTLMarker(ctx, key, ttl); err != nil {
		// Non-fatal: the key is set, TTL marker is best-effort.
		// A missing TTL marker means the key never expires, which is
		// safer than deleting the already-written main key.
		logrus.Warnf("nats set TTL marker for %q: %v", key, err)
	}
	return nil
}

func (s *natsStore) SetNX(ctx context.Context, key, value string, ttl time.Duration) (created bool, err error) {
	defer observe(opSetNX, &err)()

	// If the key exists but is expired, purge it so Create can succeed.
	if s.isExpired(ctx, key) {
		// isExpired already deleted both entries; fall through to Create.
	}

	encodedKey := encodeNATSKey(key)
	_, err = s.kv.Create(ctx, encodedKey, []byte(value))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return false, nil
		}
		return false, fmt.Errorf("nats setnx %q: %w", key, err)
	}
	// Write TTL marker after key creation. If this fails the key exists
	// without an expiry — safer than rolling back the Create (which could
	// race with another concurrent Create).
	if err := s.setTTLMarker(ctx, key, ttl); err != nil {
		logrus.Warnf("nats setnx TTL marker for %q: %v", key, err)
	}
	return true, nil
}

func (s *natsStore) Delete(ctx context.Context, key string) (err error) {
	defer observe(opDelete, &err)()

	encodedKey := encodeNATSKey(key)
	_ = s.ttlKV.Delete(ctx, encodedKey)
	if err := s.kv.Delete(ctx, encodedKey); err != nil {
		return fmt.Errorf("nats delete %q: %w", key, err)
	}
	return nil
}

func (s *natsStore) Exists(ctx context.Context, key string) (exists bool, err error) {
	defer observe(opExists, &err)()

	if s.isExpired(ctx, key) {
		return false, nil
	}
	_, err = s.kv.Get(ctx, encodeNATSKey(key))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("nats exists %q: %w", key, err)
	}
	return true, nil
}

// ── 批量操作 ─────────────────────────────────────────

func (s *natsStore) PrefixScan(ctx context.Context, prefix string) (res map[string]string, err error) {
	defer observe(opPrefixScan, &err)()

	keys, err := s.kv.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("nats prefix keys %q: %w", prefix, err)
	}
	result := make(map[string]string)
	for _, encodedKey := range keys {
		k, err := decodeNATSKey(encodedKey)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		// Skip expired keys.
		if s.isExpired(ctx, k) {
			continue
		}
		entry, err := s.kv.Get(ctx, encodedKey)
		if err != nil {
			continue // key may have been deleted between Keys() and Get()
		}
		result[k] = string(entry.Value())
	}
	return result, nil
}

func (s *natsStore) DeletePrefix(ctx context.Context, prefix string) (err error) {
	defer observe(opDeletePrefix, &err)()

	keys, err := s.kv.Keys(ctx)
	if err != nil {
		return fmt.Errorf("nats delete prefix keys %q: %w", prefix, err)
	}
	for _, encodedKey := range keys {
		k, err := decodeNATSKey(encodedKey)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		_ = s.ttlKV.Delete(ctx, encodedKey)
		_ = s.kv.Delete(ctx, encodedKey)
	}
	return nil
}

// ── 原子计数器 ───────────────────────────────────────

const casMaxRetries = 50

func (s *natsStore) incrCAS(ctx context.Context, key string, delta int64) (val int64, err error) {
	defer observe(opIncr, &err)()

	encodedKey := encodeNATSKey(key)
	for i := 0; i < casMaxRetries; i++ {
		entry, err := s.kv.Get(ctx, encodedKey)
		if err != nil {
			if errors.Is(err, jetstream.ErrKeyNotFound) {
				initVal := strconv.FormatInt(delta, 10)
				_, err := s.kv.Create(ctx, encodedKey, []byte(initVal))
				if err == nil {
					return delta, nil
				}
				if errors.Is(err, jetstream.ErrKeyExists) {
					// Another writer created it first — retry the CAS loop.
					continue
				}
				return 0, fmt.Errorf("nats incr create %q: %w", key, err)
			}
			return 0, fmt.Errorf("nats incr get %q: %w", key, err)
		}
		current, err := strconv.ParseInt(string(entry.Value()), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("nats incr parse %q: %w", key, err)
		}
		newVal := current + delta
		_, err = s.kv.Update(ctx, encodedKey, []byte(strconv.FormatInt(newVal, 10)), entry.Revision())
		if err == nil {
			return newVal, nil
		}
		if errors.Is(err, jetstream.ErrKeyExists) {
			// Revision mismatch — another writer updated first, retry.
			continue
		}
		return 0, fmt.Errorf("nats incr update %q: %w", key, err)
	}
	metrics.KVCasExhaustedTotal.Inc()
	return 0, fmt.Errorf("nats incr %q: CAS retry exhausted (%d attempts)", key, casMaxRetries)
}

func (s *natsStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.incrCAS(ctx, key, 1)
}

func (s *natsStore) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	return s.incrCAS(ctx, key, delta)
}

// ── Pub/Sub (NATS core pub/sub) ─────────────────────────

const natsChannelPrefix = "nats.kv."

func (s *natsStore) Publish(_ context.Context, channel string, payload []byte) error {
	subject := natsChannelPrefix + channel
	return s.nc.Publish(subject, payload)
}

func (s *natsStore) Subscribe(ctx context.Context, channel string) (<-chan []byte, error) {
	out := make(chan []byte, 64)
	subject := natsChannelPrefix + channel

	natsSub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		select {
		case out <- msg.Data:
		default:
		}
	})
	if err != nil {
		close(out)
		return nil, fmt.Errorf("nats subscribe %q: %w", subject, err)
	}

	go func() {
		<-ctx.Done()
		if err := natsSub.Unsubscribe(); err != nil {
			logrus.Warnf("nats unsubscribe %q: %v", subject, err)
		}
		close(out)
	}()

	return out, nil
}

// ── 生命周期 ──────────────────────────────────────

func (s *natsStore) Close() error {
	if s.cancelWatch != nil {
		s.cancelWatch()
	}
	if s.watcher != nil {
		s.watcher.Stop()
	}
	s.wg.Wait()
	if s.nc != nil {
		s.nc.Close()
	}
	return nil
}
