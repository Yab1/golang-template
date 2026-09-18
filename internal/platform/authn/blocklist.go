package authn

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const blockPrefix = "auth:bl:"

type Blocklist struct {
	rdb *redis.Client
}

func NewBlocklist(rdb *redis.Client) *Blocklist {
	return &Blocklist{rdb: rdb}
}

func (b *Blocklist) Ban(ctx context.Context, jti string, until time.Time) error {
	if b == nil || b.rdb == nil || jti == "" {
		return nil
	}
	ttl := time.Until(until)
	if ttl <= 0 {
		return nil
	}
	return b.rdb.Set(ctx, blockPrefix+jti, "1", ttl).Err()
}

func (b *Blocklist) Blocked(ctx context.Context, jti string) (bool, error) {
	if b == nil || b.rdb == nil || jti == "" {
		return false, nil
	}
	n, err := b.rdb.Exists(ctx, blockPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
