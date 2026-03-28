package main

import (
	"bytes"
	"database/sql"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/peterrk/simple-abtest/utils"
	"github.com/redis/go-redis/v9"
)

const (
	sessionTTL   = 30 * time.Minute
	privCacheTTL = 10 * time.Minute
	relationTTL  = uint32((24*time.Hour)/time.Second) * 7

	sessionUidCookieName   = "SESSION_UID"
	sessionTokenCookieName = "SESSION_TOKEN"
)

var authSql struct {
	getExpApp *sql.Stmt
	getLyrExp *sql.Stmt
	getSegLyr *sql.Stmt
	getGrpSeg *sql.Stmt
}

func prepareAuthSql(db *sql.DB) (err error) {
	authSql.getExpApp, err = db.Prepare(
		"SELECT `app_id` FROM `experiment` WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	authSql.getLyrExp, err = db.Prepare(
		"SELECT `exp_id` FROM `exp_layer` WHERE `lyr_id`=?")
	if err != nil {
		return err
	}
	authSql.getSegLyr, err = db.Prepare(
		"SELECT `lyr_id` FROM `exp_segment` WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	authSql.getGrpSeg, err = db.Prepare(
		"SELECT `seg_id` FROM `exp_group` WHERE `grp_id`=?")
	if err != nil {
		return err
	}
	relationCache.expToApp = make(map[uint32]idMark)
	relationCache.lyrToExp = make(map[uint32]idMark)
	relationCache.segToLyr = make(map[uint32]idMark)
	relationCache.grpToSeg = make(map[uint32]idMark)
	return nil
}

type idMark struct {
	id    uint32
	stamp uint32
}

func newIdMark(id uint32) idMark {
	return idMark{id: id, stamp: uint32(time.Now().Unix())}
}

// Because relationship between application, layer, segment, group, config is immutable,
// cache this relation in memeory. This data will not be too big
var relationCache struct {
	lock     sync.Mutex
	expToApp map[uint32]idMark
	lyrToExp map[uint32]idMark
	segToLyr map[uint32]idMark
	grpToSeg map[uint32]idMark
}

func cacheDropOld() {
	stamp := uint32(time.Now().Unix())
	// avoid to hold lock too long time
	cacheDropOldParly(relationCache.expToApp, stamp)
	cacheDropOldParly(relationCache.lyrToExp, stamp)
	cacheDropOldParly(relationCache.segToLyr, stamp)
	cacheDropOldParly(relationCache.grpToSeg, stamp)
}

func cacheDropOldParly(cache map[uint32]idMark, stamp uint32) {
	relationCache.lock.Lock()
	for key, val := range cache {
		if stamp-val.stamp > relationTTL {
			delete(cache, key)
		}
	}
	relationCache.lock.Unlock()
}

type privilegeChecker func(*Context, uint32) (bool, error)

func checkAppPrivilege(ctx *Context, uid, appId uint32, expected privilegeLevel) (bool, error) {
	level, err := getPrivilege(ctx, uid, appId)
	if err != nil {
		return false, err
	}
	return level >= expected, nil
}

func resolveCachedRelation(
	ctx *Context, cache map[uint32]idMark, key uint32, stmt *sql.Stmt,
) (uint32, bool, error) {
	relationCache.lock.Lock()
	val, ok := cache[key]
	if ok {
		cache[key] = newIdMark(val.id)
	}
	relationCache.lock.Unlock()
	if ok {
		return val.id, true, nil
	}

	if err := stmt.QueryRowContext(ctx, key).Scan(&val.id); err != nil {
		if err == sql.ErrNoRows {
			ctx.Debugf("auth relation miss key=%d", key)
			return 0, false, nil
		}
		return 0, false, err
	}

	relationCache.lock.Lock()
	cache[key] = newIdMark(val.id)
	relationCache.lock.Unlock()
	return val.id, true, nil
}

func resolveAppByExp(ctx *Context, expId uint32) (uint32, bool, error) {
	return resolveCachedRelation(ctx, relationCache.expToApp, expId, authSql.getExpApp)
}

func resolveExpByLyr(ctx *Context, lyrId uint32) (uint32, bool, error) {
	return resolveCachedRelation(ctx, relationCache.lyrToExp, lyrId, authSql.getLyrExp)
}

func resolveLyrBySeg(ctx *Context, segId uint32) (uint32, bool, error) {
	return resolveCachedRelation(ctx, relationCache.segToLyr, segId, authSql.getSegLyr)
}

func resolveSegByGrp(ctx *Context, grpId uint32) (uint32, bool, error) {
	return resolveCachedRelation(ctx, relationCache.grpToSeg, grpId, authSql.getGrpSeg)
}

func checkExpPrivilege(ctx *Context, uid, expId uint32, expected privilegeLevel) (bool, error) {
	appId, ok, err := resolveAppByExp(ctx, expId)
	if err != nil {
		return false, err
	}
	if !ok {
		ctx.Debugf("auth exp privilege unresolved exp=%d", expId)
		return false, nil
	}
	return checkAppPrivilege(ctx, uid, appId, expected)
}

func checkLyrPrivilege(ctx *Context, uid, lyrId uint32, expected privilegeLevel) (bool, error) {
	expId, ok, err := resolveExpByLyr(ctx, lyrId)
	if err != nil {
		return false, err
	}
	if !ok {
		ctx.Debugf("auth layer privilege unresolved layer=%d", lyrId)
		return false, nil
	}
	return checkExpPrivilege(ctx, uid, expId, expected)
}

func checkSegPrivilege(ctx *Context, uid, segId uint32, expected privilegeLevel) (bool, error) {
	lyrId, ok, err := resolveLyrBySeg(ctx, segId)
	if err != nil {
		return false, err
	}
	if !ok {
		ctx.Debugf("auth segment privilege unresolved seg=%d", segId)
		return false, nil
	}
	return checkLyrPrivilege(ctx, uid, lyrId, expected)
}

func checkGrpPrivilege(ctx *Context, uid, grpId uint32, expected privilegeLevel) (bool, error) {
	segId, ok, err := resolveSegByGrp(ctx, grpId)
	if err != nil {
		return false, err
	}
	if !ok {
		ctx.Debugf("auth group privilege unresolved grp=%d", grpId)
		return false, nil
	}
	return checkSegPrivilege(ctx, uid, segId, expected)
}

func getPrivilege(ctx *Context, uid, appId uint32) (privilegeLevel, error) {
	key := makePrivilegeKey(uid)
	field := strconv.FormatUint(uint64(appId), 10)

	if txt, err := rds.HGet(ctx, key, field).Result(); err == nil {
		level, convErr := strconv.Atoi(txt)
		if convErr == nil {
			return privilegeLevel(level), nil
		}
	} else if err != redis.Nil {
		ctx.Warnf("fail to get privilege cache for uid=%d app_id=%d: %v", uid, appId, err)
	}

	level := int(privilegeNoAccess)
	err := privSql.getOne.QueryRowContext(ctx, uid, appId).Scan(&level)
	if err != nil {
		if err != sql.ErrNoRows {
			return 0, err
		}
		level = int(privilegeNoAccess)
	}

	pipe := rds.Pipeline()
	pipe.HSet(ctx, key, field, strconv.Itoa(level))
	pipe.Expire(ctx, key, privCacheTTL)
	if _, cacheErr := pipe.Exec(ctx); cacheErr != nil {
		ctx.Warnf("fail to set privilege cache for uid=%d app_id=%d: %v", uid, appId, cacheErr)
	}

	return privilegeLevel(level), nil
}

func initSession(ctx *Context, uid uint32) (string, error) {
	token, err := utils.GenRandomToken()
	if err != nil {
		return "", err
	}
	if err := rds.Set(ctx, makeSessionKey(uid),
		token, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func buildSessionCookie(r *http.Request, name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   isSecureRequest(r),
		MaxAge:   int(sessionTTL / time.Second),
	}
}

func setSessionCookies(w http.ResponseWriter, r *http.Request, uid uint32, token string) {
	http.SetCookie(w, buildSessionCookie(r, sessionUidCookieName, strconv.FormatUint(uint64(uid), 10)))
	http.SetCookie(w, buildSessionCookie(r, sessionTokenCookieName, token))
}

func verifySession(ctx *Context, w http.ResponseWriter, r *http.Request) (uint32, bool) {
	uidCookie, err := r.Cookie(sessionUidCookieName)
	if err != nil {
		ctx.Debug("auth session missing uid cookie")
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}
	tokenCookie, err := r.Cookie(sessionTokenCookieName)
	if err != nil {
		ctx.Debug("auth session missing token cookie")
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}
	uidText := uidCookie.Value
	token := tokenCookie.Value

	uid64, err := strconv.ParseUint(uidText, 10, 32)
	if err != nil {
		ctx.Debugf("auth session invalid uid=%q", uidText)
		w.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	uid := uint32(uid64)

	rawCtx := ctx.Context
	key := makeSessionKey(uid)
	cached, err := rds.Get(rawCtx, key).Result()
	if err != nil {
		if err == redis.Nil {
			ctx.Debugf("auth session cache miss uid=%d", uid)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			ctx.Errorf("fail to get session token in redis: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return 0, false
	}
	if !bytes.Equal([]byte(cached), []byte(token)) {
		ctx.Debugf("auth session token mismatch uid=%d", uid)
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}
	if err := rds.Expire(rawCtx, key, sessionTTL).Err(); err != nil {
		ctx.Errorf("fail to renew session token in redis: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 0, false
	}
	setSessionCookies(w, r, uid, token)
	return uid, true
}

func requireSession(ctx *Context, w http.ResponseWriter, r *http.Request) (uint32, bool) {
	uid, ok := verifySession(ctx, w, r)
	if !ok {
		return 0, false
	}
	return uid, true
}

func requireSelf(
	ctx *Context, w http.ResponseWriter, r *http.Request, target uint32,
) (uint32, bool) {
	uid, ok := requireSession(ctx, w, r)
	if !ok {
		return 0, false
	}
	if uid != target {
		ctx.Debugf("auth self rejected uid=%d target=%d", uid, target)
		w.WriteHeader(http.StatusForbidden)
		return 0, false
	}
	return uid, true
}

func requirePrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	checker privilegeChecker,
) (uint32, bool) {
	uid, ok := requireSession(ctx, w, r)
	if !ok {
		return 0, false
	}

	allowed, err := checker(ctx, uid)
	if err != nil {
		ctx.Errorf("fail to check privilege: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 0, false
	}
	if !allowed {
		ctx.Debugf("auth privilege rejected uid=%d", uid)
		w.WriteHeader(http.StatusForbidden)
		return 0, false
	}
	return uid, true
}

func requireAppPrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	appId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(ctx, w, r, func(raw *Context, uid uint32) (bool, error) {
		return checkAppPrivilege(raw, uid, appId, expected)
	})
}

func requireExpPrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	expId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(ctx, w, r, func(raw *Context, uid uint32) (bool, error) {
		return checkExpPrivilege(raw, uid, expId, expected)
	})
}

func requireLyrPrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	lyrId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(ctx, w, r, func(raw *Context, uid uint32) (bool, error) {
		return checkLyrPrivilege(raw, uid, lyrId, expected)
	})
}

func requireSegPrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	segId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(ctx, w, r, func(raw *Context, uid uint32) (bool, error) {
		return checkSegPrivilege(raw, uid, segId, expected)
	})
}

func requireGrpPrivilege(
	ctx *Context, w http.ResponseWriter, r *http.Request,
	grpId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(ctx, w, r, func(raw *Context, uid uint32) (bool, error) {
		return checkGrpPrivilege(raw, uid, grpId, expected)
	})
}
