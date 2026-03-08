package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var userSql struct {
	create *sql.Stmt
	update *sql.Stmt
	remove *sql.Stmt

	getSalt   *sql.Stmt
	getByName *sql.Stmt
}

var privSql struct {
	update *sql.Stmt
	remove *sql.Stmt

	getListByApp *sql.Stmt
	getListByUid *sql.Stmt
	getOne       *sql.Stmt
	getUidByName *sql.Stmt

	getExpApp *sql.Stmt
	getLyrExp *sql.Stmt
	getSegLyr *sql.Stmt
	getGrpSeg *sql.Stmt
}

const (
	sessionTTL        = 30 * time.Minute
	privilegeCacheTTL = 10 * time.Minute
)

type privilegeLevel uint8

func prepareUserSql(db *sql.DB) (err error) {
	userSql.create, err = db.Prepare(
		"INSERT INTO `user`(`name`,`slat`,`password`) VALUES (?,?,?)")
	if err != nil {
		return err
	}
	userSql.update, err = db.Prepare(
		"UPDATE `user` SET `password`=? WHERE `uid`=?")
	if err != nil {
		return err
	}
	userSql.remove, err = db.Prepare(
		"DELETE FROM `user` WHERE `uid`=?")
	if err != nil {
		return err
	}
	userSql.getSalt, err = db.Prepare(
		"SELECT `slat` FROM `user` WHERE `uid`=?")
	if err != nil {
		return err
	}
	userSql.getByName, err = db.Prepare(
		"SELECT `uid`,`slat`,`password` FROM `user` WHERE `name`=?")
	if err != nil {
		return err
	}

	privSql.update, err = db.Prepare(
		"INSERT INTO `privilege`(`uid`,`app_id`,`privilege`,`grant_by`) " +
			"VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE " +
			"`privilege`=?,`grant_by`=?")
	if err != nil {
		return err
	}
	privSql.remove, err = db.Prepare(
		"DELETE FROM `privilege` WHERE `uid`=? AND `app_id`=?")
	if err != nil {
		return err
	}
	privSql.getListByApp, err = db.Prepare("SELECT " +
		" t2.name AS `user`, t1.privilege, COALESCE(t3.name,'') AS `grantor` " +
		"FROM " +
		"( SELECT `uid`,`privilege`,`grant_by`,`update_time`" +
		"  FROM `privilege` WHERE `app_id`=? ) t1 " +
		"INNER JOIN " +
		"( SELECT `uid`,`name` FROM `user` ) t2 " +
		"ON t1.uid = t2.uid " +
		"LEFT JOIN " +
		"( SELECT `uid`,`name` FROM `user` ) t3 " +
		"ON t1.grant_by = t3.uid " +
		"ORDER BY t1.update_time DESC")
	if err != nil {
		return err
	}
	privSql.getListByUid, err = db.Prepare(
		"SELECT `app_id`,`privilege` FROM `privilege` WHERE `uid`=?")
	if err != nil {
		return err
	}
	privSql.getOne, err = db.Prepare(
		"SELECT `privilege` FROM `privilege` WHERE `uid`=? AND `app_id`=?")
	if err != nil {
		return err
	}
	privSql.getUidByName, err = db.Prepare(
		"SELECT `uid` FROM `user` WHERE `name`=?")
	if err != nil {
		return err
	}

	privSql.getExpApp, err = db.Prepare(
		"SELECT `app_id` FROM `experiment` WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	privSql.getLyrExp, err = db.Prepare(
		"SELECT `exp_id` FROM `exp_layer` WHERE `lyr_id`=?")
	if err != nil {
		return err
	}
	privSql.getSegLyr, err = db.Prepare(
		"SELECT `lyr_id` FROM `exp_segment` WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	privSql.getGrpSeg, err = db.Prepare(
		"SELECT `seg_id` FROM `exp_group` WHERE `grp_id`=?")
	if err != nil {
		return err
	}
	relationCache.expToApp = make(map[uint32]uint32)
	relationCache.lyrToExp = make(map[uint32]uint32)
	relationCache.segToLyr = make(map[uint32]uint32)
	relationCache.grpToSeg = make(map[uint32]uint32)
	return nil
}

func bindUserOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/user", userCreate)
	router.Handle(http.MethodPost, "/api/user/login", userLogin)
	router.Handle(http.MethodPut, "/api/user/:id", userUpdate)
	router.Handle(http.MethodDelete, "/api/user/:id", userDelete)

	router.Handle(http.MethodPost, "/api/app/:id/privilege", appGrantPrivilege)
	router.Handle(http.MethodGet, "/api/app/:id/privilege", appGetPrivilege)
}

const (
	privilegeNoAccess  privilegeLevel = 0
	privilegeReadOnly  privilegeLevel = 1
	privilegeReadWrite privilegeLevel = 2
	privilegeAdmin     privilegeLevel = 3
)

func makeSessionKey(uid uint32) string {
	return fmt.Sprintf("%s%d", sessionPrefix, uid)
}

func makePrivilegeKey(uid uint32) string {
	return fmt.Sprintf("%s%d", privilegePrefix, uid)
}

func hashPassword(password string, salt []byte) [32]byte {
	h := sha256.New()
	h.Write(utils.UnsafeStringToBytes(password))
	h.Write(salt)
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	return sum
}

func issueSession(uid uint32) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	ctx := context.Background()
	if err := rds.Set(ctx, makeSessionKey(uid), token, sessionTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func verifySession(logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request) (uint32, bool) {
	uidText := r.Header.Get("SESSION_UID")
	token := r.Header.Get("SESSION_TOKEN")
	if len(uidText) == 0 || len(token) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}

	uid64, err := strconv.ParseUint(uidText, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	uid := uint32(uid64)

	ctx := context.Background()
	key := makeSessionKey(uid)
	cached, err := rds.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			logger.Errorf("fail to get session token in redis: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return 0, false
	}
	if subtle.ConstantTimeCompare([]byte(cached), []byte(token)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return 0, false
	}
	if err := rds.Expire(ctx, key, sessionTTL).Err(); err != nil {
		logger.Errorf("fail to renew session token in redis: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 0, false
	}
	return uid, true
}

func checkAppPrivilege(uid, appId uint32, expected privilegeLevel) (bool, error) {
	level, err := getPrivilege(uid, appId)
	if err != nil {
		return false, err
	}
	return level >= expected, nil
}

// Because relationship between application, layer, segment, group, config is immutable,
// cache this relation in memeory. This data will not be too big
var relationCache struct {
	lock     sync.RWMutex
	expToApp map[uint32]uint32
	lyrToExp map[uint32]uint32
	segToLyr map[uint32]uint32
	grpToSeg map[uint32]uint32
}

func resolveCachedRelation(cache map[uint32]uint32, key uint32, stmt *sql.Stmt) (uint32, bool, error) {
	relationCache.lock.RLock()
	val, ok := cache[key]
	relationCache.lock.RUnlock()
	if ok {
		return val, true, nil
	}

	if err := stmt.QueryRow(key).Scan(&val); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}

	relationCache.lock.Lock()
	cache[key] = val
	relationCache.lock.Unlock()
	return val, true, nil
}

func resolveAppByExp(expId uint32) (uint32, bool, error) {
	return resolveCachedRelation(relationCache.expToApp, expId, privSql.getExpApp)
}

func resolveExpByLyr(lyrId uint32) (uint32, bool, error) {
	return resolveCachedRelation(relationCache.lyrToExp, lyrId, privSql.getLyrExp)
}

func resolveLyrBySeg(segId uint32) (uint32, bool, error) {
	return resolveCachedRelation(relationCache.segToLyr, segId, privSql.getSegLyr)
}

func resolveSegByGrp(grpId uint32) (uint32, bool, error) {
	return resolveCachedRelation(relationCache.grpToSeg, grpId, privSql.getGrpSeg)
}

func checkExpPrivilege(uid, expId uint32, expected privilegeLevel) (bool, error) {
	appId, ok, err := resolveAppByExp(expId)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return checkAppPrivilege(uid, appId, expected)
}

func checkLyrPrivilege(uid, lyrId uint32, expected privilegeLevel) (bool, error) {
	expId, ok, err := resolveExpByLyr(lyrId)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return checkExpPrivilege(uid, expId, expected)
}

func checkSegPrivilege(uid, segId uint32, expected privilegeLevel) (bool, error) {
	lyrId, ok, err := resolveLyrBySeg(segId)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return checkLyrPrivilege(uid, lyrId, expected)
}

func checkGrpPrivilege(uid, grpId uint32, expected privilegeLevel) (bool, error) {
	segId, ok, err := resolveSegByGrp(grpId)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return checkSegPrivilege(uid, segId, expected)
}

func getPrivilege(uid, appId uint32) (privilegeLevel, error) {
	ctx := context.Background()
	key := makePrivilegeKey(uid)
	field := strconv.FormatUint(uint64(appId), 10)

	if txt, err := rds.HGet(ctx, key, field).Result(); err == nil {
		level, convErr := strconv.Atoi(txt)
		if convErr == nil {
			return privilegeLevel(level), nil
		}
	} else if err != redis.Nil {
		utils.GetLogger().Warnf("fail to get privilege cache for uid=%d app_id=%d: %v", uid, appId, err)
	}

	level := int(privilegeNoAccess)
	err := privSql.getOne.QueryRow(uid, appId).Scan(&level)
	if err != nil {
		if err != sql.ErrNoRows {
			return 0, err
		}
		err = nil
		level = int(privilegeNoAccess)
	}

	pipe := rds.Pipeline()
	pipe.HSet(ctx, key, field, strconv.Itoa(level))
	pipe.Expire(ctx, key, privilegeCacheTTL)
	if _, cacheErr := pipe.Exec(ctx); cacheErr != nil {
		utils.GetLogger().Warnf("fail to set privilege cache for uid=%d app_id=%d: %v", uid, appId, cacheErr)
	}

	return privilegeLevel(level), nil
}

func userCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("userCreate")
	req := &struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}{}
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil || len(req.Name) == 0 || len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		logger.Errorf("fail to generate random salt: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	digest := hashPassword(req.Password, salt)

	id, err := utils.SqlCreate(userSql.create, req.Name, salt, digest[:])
	if err != nil {
		if utils.IsMysqlDuplicateError(err) {
			w.WriteHeader(http.StatusConflict)
		} else {
			logger.Errorf("fail to run sql[user.create]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	uid := uint32(id)
	token, err := issueSession(uid)
	if err != nil {
		logger.Errorf("fail to issue session token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, &struct {
		Uid   uint32 `json:"uid"`
		Token string `json:"token"`
	}{Uid: uid, Token: token})
}

func userLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("userLogin")
	req := &struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}{}
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil || len(req.Name) == 0 || len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var (
		uid      uint32
		salt     []byte
		password []byte
	)
	err := userSql.getByName.QueryRow(req.Name).Scan(&uid, &salt, &password)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			logger.Errorf("fail to run sql[user.getByName]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	digest := hashPassword(req.Password, salt)
	if subtle.ConstantTimeCompare(digest[:], password) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := issueSession(uid)
	if err != nil {
		logger.Errorf("fail to issue session token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, &struct {
		Uid   uint32 `json:"uid"`
		Token string `json:"token"`
	}{Uid: uid, Token: token})
}

func userUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("userUpdate")
	target, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, ok := requireSelf(logger, w, r, uint32(target)); !ok {
		return
	}

	req := &struct {
		Password string `json:"password"`
	}{}
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil || len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var salt []byte
	err = userSql.getSalt.QueryRow(target).Scan(&salt)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			logger.Errorf("fail to run sql[user.getSalt]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	digest := hashPassword(req.Password, salt)
	n, err := utils.SqlModify(userSql.update, digest[:], target)
	if err != nil {
		logger.Errorf("fail to run sql[user.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func userDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("userDelete")
	target, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, ok := requireSelf(logger, w, r, uint32(target)); !ok {
		return
	}

	n, err := utils.SqlModify(userSql.remove, target)
	if err != nil {
		logger.Errorf("fail to run sql[user.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx := context.Background()
	if err := rds.Del(ctx, makeSessionKey(uint32(target))).Err(); err != nil {
		logger.Warnf("fail to clear session cache: %v", err)
	}
	if err := rds.Del(ctx, makePrivilegeKey(uint32(target))).Err(); err != nil {
		logger.Warnf("fail to clear privilege cache: %v", err)
	}
}

type appPrivilege struct {
	Name      string `json:"name"`
	Privilege int    `json:"privilege"`
	Grantor   string `json:"grantor"`
}

func appGetPrivilege(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appGetPrivilege")
	appId, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireAppPrivilege(logger, w, r, uint32(appId), privilegeAdmin); !ok {
		return
	}

	rows, err := privSql.getListByApp.Query(appId)
	if err != nil {
		logger.Errorf("fail to run sql[priv.getListByApp]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	resp := make([]appPrivilege, 0)
	for rows.Next() {
		var rec appPrivilege
		if err = rows.Scan(&rec.Name, &rec.Privilege, &rec.Grantor); err != nil {
			logger.Errorf("fail to run sql[priv.getListByApp]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp = append(resp, rec)
	}
	if err = rows.Err(); err != nil {
		logger.Errorf("fail to iterate sql[priv.getListByApp]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, &resp)
}

func appGrantPrivilege(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appGrantPrivilege")
	appId, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	grantBy, ok := requireSession(logger, w, r)
	if !ok {
		return
	}

	req := &struct {
		Name      string `json:"name"`
		Privilege int    `json:"privilege"`
	}{}
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil || len(req.Name) == 0 ||
		req.Privilege < int(privilegeNoAccess) || req.Privilege > int(privilegeAdmin) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var targetUid uint32
	err = privSql.getUidByName.QueryRow(req.Name).Scan(&targetUid)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			logger.Errorf("fail to run sql[priv.getUidByName]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	allowed, err := checkAppPrivilege(grantBy, uint32(appId), privilegeAdmin)
	if err != nil {
		logger.Errorf("fail to check privilege: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if req.Privilege == int(privilegeNoAccess) {
		if _, err := utils.SqlModify(privSql.remove, targetUid, appId); err != nil {
			logger.Errorf("fail to run sql[priv.remove]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := utils.SqlModify(privSql.update,
			targetUid, appId, req.Privilege, grantBy,
			req.Privilege, grantBy); err != nil {
			logger.Errorf("fail to run sql[priv.update]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	ctx := context.Background()
	if err := rds.Del(ctx, makePrivilegeKey(targetUid)).Err(); err != nil {
		logger.Warnf("fail to clear privilege cache: %v", err)
	}
}
