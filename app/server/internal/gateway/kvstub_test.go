package gateway

import (
	"context"
	"errors"
	"time"

	"orinforum/internal/pkg/kvstore"
)

// errUnusedKV marks a kvstore.Store verb that a given test double never
// exercises. It is an error rather than a panic so an unexpected call shows up
// as a failed assertion instead of killing the test binary.
var errUnusedKV = errors.New("kvstore method not implemented by this test double")

// baseKV is embedded by the gateway test doubles to supply the parts of
// kvstore.Store they do not model. Deps.KV is typed as kvstore.Store, so a
// double must satisfy the whole interface; embedding keeps each stub to the
// verbs it actually cares about. Methods defined on the embedding type shadow
// these ones.
type baseKV struct{}

func (baseKV) Get(context.Context, string) (string, error) { return "", kvstore.ErrKeyNotFound }
func (baseKV) Set(context.Context, string, string, time.Duration) error {
	return errUnusedKV
}
func (baseKV) SetNX(context.Context, string, string, time.Duration) (bool, error) {
	return false, errUnusedKV
}
func (baseKV) Delete(context.Context, string) error { return errUnusedKV }
func (baseKV) Exists(context.Context, string) (bool, error) {
	return false, errUnusedKV
}
func (baseKV) PrefixScan(context.Context, string) (map[string]string, error) {
	return nil, errUnusedKV
}
func (baseKV) DeletePrefix(context.Context, string) error  { return errUnusedKV }
func (baseKV) Incr(context.Context, string) (int64, error) { return 0, errUnusedKV }
func (baseKV) IncrBy(context.Context, string, int64) (int64, error) {
	return 0, errUnusedKV
}
func (baseKV) Publish(context.Context, string, []byte) error { return errUnusedKV }
func (baseKV) Subscribe(context.Context, string) (<-chan []byte, error) {
	return nil, errUnusedKV
}
func (baseKV) Close() error { return nil }
