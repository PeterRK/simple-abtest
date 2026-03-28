package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/peterrk/simple-abtest/utils"
)

type rateLimitRule struct {
	prefix string
	limit  int64
	window time.Duration
}

var (
	// Keep rate limiting bound to stable business identities.
	// RemoteAddr-based IP limiting is intentionally omitted because this service
	// is commonly deployed behind gateways/reverse proxies, where the direct
	// source address is not a reliable client identifier.
	loginRateLimitByAccount = rateLimitRule{prefix: "login-account", limit: 8, window: 10 * time.Minute}
	updateRateLimitByUser   = rateLimitRule{prefix: "user-update-id", limit: 5, window: 15 * time.Minute}
	deleteRateLimitByUser   = rateLimitRule{prefix: "user-delete-id", limit: 3, window: 30 * time.Minute}
)

func rateLimitKey(rule rateLimitRule, scope string) string {
	return fmt.Sprintf("%srate-limit:%s:%s", redisPrefix, rule.prefix, scope)
}

func normalizeRateLimitScope(scope string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if len(scope) == 0 {
		return "unknown"
	}
	scope = strings.ReplaceAll(scope, ":", "_")
	scope = strings.ReplaceAll(scope, " ", "_")
	return scope
}

func checkRateLimit(ctx *Context, rule rateLimitRule, scope string) (bool, error) {
	key := rateLimitKey(rule, normalizeRateLimitScope(scope))
	ttl := int64(rule.window / time.Second)
	n, err := utils.IncrWithTTL(ctx, rds, key, ttl)
	if err != nil {
		return false, err
	}
	return n <= rule.limit, nil
}

func requireRateLimit(ctx *Context, w http.ResponseWriter, r *http.Request, rule rateLimitRule, scope string) bool {
	allowed, err := checkRateLimit(ctx, rule, scope)
	if err != nil {
		ctx.Errorf("fail to check rate limit [%s]: %v", rule.prefix, err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	if allowed {
		return true
	}
	ctx.Warnf("rate limit exceeded [%s] scope=%s", rule.prefix, normalizeRateLimitScope(scope))
	w.WriteHeader(http.StatusTooManyRequests)
	return false
}
