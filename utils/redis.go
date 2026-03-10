package utils

import (
	"context"

	"github.com/redis/go-redis/v9"
)

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
