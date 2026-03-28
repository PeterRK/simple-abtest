package utils

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strings"

	"github.com/redis/go-redis/v9"
)

// incrWithTTL atomically increments a counter and sets its TTL on first creation.
// Returns the counter value after increment.
var incrWithTTLScript = `
	local n = redis.call('INCR', KEYS[1])
	if n == 1 then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	return n
`

var incrWithTTLSHA string

func init() {
	sum := sha1.Sum([]byte(incrWithTTLScript))
	incrWithTTLSHA = hex.EncodeToString(sum[:])
}

func IncrWithTTL(ctx context.Context, db *redis.Client, key string, ttlSeconds int64) (int64, error) {
	keys := []string{key}
	n, err := db.EvalSha(ctx, incrWithTTLSHA, keys, ttlSeconds).Int64()
	if err != nil && strings.HasPrefix(err.Error(), "NOSCRIPT") {
		n, err = db.Eval(ctx, incrWithTTLScript, keys, ttlSeconds).Int64()
	}
	return n, err
}

type RedisConfig struct {
	Address  string `json:"address" yaml:"address"`
	Password string `json:"password" yaml:"password"`
	PoolSize int    `json:"pool_size" yaml:"pool_size"`
	IdleSize int    `json:"idle_size" yaml:"idle_size"`
}

func newRedisClient(cfg *RedisConfig, ping bool) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.IdleSize,
	})
	if !ping {
		return c, nil
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func NewRedisClient(cfg *RedisConfig) *redis.Client {
	c, _ := newRedisClient(cfg, false)
	return c
}

func NewRedisClientWithCheck(cfg *RedisConfig) (*redis.Client, error) {
	return newRedisClient(cfg, true)
}
