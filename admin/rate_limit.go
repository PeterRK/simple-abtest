package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type rateLimitRule struct {
	prefix string
	limit  int64
	window time.Duration
}

var (
	loginRateLimitByIP      = rateLimitRule{prefix: "login-ip", limit: 20, window: 5 * time.Minute}
	loginRateLimitByAccount = rateLimitRule{prefix: "login-account", limit: 8, window: 10 * time.Minute}
	updateRateLimitByIP     = rateLimitRule{prefix: "user-update-ip", limit: 10, window: 10 * time.Minute}
	updateRateLimitByUser   = rateLimitRule{prefix: "user-update-id", limit: 5, window: 15 * time.Minute}
	deleteRateLimitByIP     = rateLimitRule{prefix: "user-delete-ip", limit: 6, window: 15 * time.Minute}
	deleteRateLimitByUser   = rateLimitRule{prefix: "user-delete-id", limit: 3, window: 30 * time.Minute}
)

func rateLimitKey(rule rateLimitRule, scope string) string {
	return fmt.Sprintf("%srate-limit:%s:%s", redisPrefix, rule.prefix, scope)
}

func getRequestAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && len(host) != 0 {
		return host
	}
	if len(r.RemoteAddr) != 0 {
		return r.RemoteAddr
	}
	return "unknown"
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
	n, err := rds.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err := rds.Expire(ctx, key, rule.window).Err(); err != nil {
			return false, err
		}
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
	ctx.Warnf("rate limit exceeded [%s] scope=%s addr=%s", rule.prefix, normalizeRateLimitScope(scope), getRequestAddr(r))
	w.WriteHeader(http.StatusTooManyRequests)
	return false
}
