package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
)

var userSql struct {
	create *sql.Stmt
	update *sql.Stmt
	remove *sql.Stmt

	getByName *sql.Stmt
	getByUid  *sql.Stmt
}

var privSql struct {
	update *sql.Stmt
	remove *sql.Stmt

	getListByApp *sql.Stmt
	getListByUid *sql.Stmt
	getOne       *sql.Stmt
	getUidByName *sql.Stmt
}

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
	userSql.getByName, err = db.Prepare(
		"SELECT `uid`,`slat`,`password` FROM `user` WHERE `name`=?")
	if err != nil {
		return err
	}
	userSql.getByUid, err = db.Prepare(
		"SELECT `slat`,`password` FROM `user` WHERE `uid`=?")
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

type privilegeLevel uint8

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

// hashPassword intentionally uses sha256(password+salt) for this project.
// Threat model: admin service is for trusted intranet use only.
// If exposure scope changes (internet-facing / stronger compliance), migrate to Argon2id/Bcrypt.
func hashPassword(password string, salt []byte) [32]byte {
	h := sha256.New()
	h.Write(utils.UnsafeStringToBytes(password))
	h.Write(salt)
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	return sum
}

func getUserCredentialByUid(ctx *Context, uid uint32) ([]byte, []byte, error) {
	var (
		salt     []byte
		password []byte
	)
	if err := userSql.getByUid.QueryRowContext(ctx, uid).Scan(&salt, &password); err != nil {
		return nil, nil, err
	}
	return salt, password, nil
}

func verifyUserPasswordByUid(ctx *Context, uid uint32, raw string) (int, []byte) {
	salt, password, err := getUserCredentialByUid(ctx, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return http.StatusNotFound, nil
		}
		ctx.Errorf("fail to run sql[user.getByUid]: %v", err)
		return http.StatusInternalServerError, nil
	}
	digest := hashPassword(raw, salt)
	if !bytes.Equal(digest[:], password) {
		ctx.Debugf("user password mismatch uid=%d", uid)
		return http.StatusUnauthorized, nil
	}
	return http.StatusOK, salt
}

func userCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "userCreate")
	req := &struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if len(req.Name) == 0 || len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		ctx.Errorf("fail to generate random salt: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	digest := hashPassword(req.Password, salt)

	id, err := utils.SqlCreate(ctx, userSql.create, req.Name, salt, digest[:])
	if err != nil {
		if utils.IsMysqlDuplicateError(err) {
			ctx.Debugf("user create conflict: duplicate name=%q", req.Name)
			w.WriteHeader(http.StatusConflict)
		} else {
			ctx.Errorf("fail to run sql[user.create]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	uid := uint32(id)
	token, err := initSession(ctx, uid)
	if err != nil {
		ctx.Errorf("fail to init session token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &struct {
		Uid   uint32 `json:"uid"`
		Token string `json:"token"`
	}{Uid: uid, Token: token})
}

func userLogin(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "userLogin")
	req := &struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if len(req.Name) == 0 || len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var (
		uid      uint32
		salt     []byte
		password []byte
	)
	err := userSql.getByName.QueryRowContext(ctx, req.Name).Scan(&uid, &salt, &password)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Debugf("user login rejected: unknown name=%q", req.Name)
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			ctx.Errorf("fail to run sql[user.getByName]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	digest := hashPassword(req.Password, salt)
	if !bytes.Equal(digest[:], password) {
		ctx.Debugf("user login rejected: password mismatch name=%q uid=%d", req.Name, uid)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token, err := initSession(ctx, uid)
	if err != nil {
		ctx.Errorf("fail to init session token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &struct {
		Uid   uint32 `json:"uid"`
		Token string `json:"token"`
	}{Uid: uid, Token: token})
}

func userUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "userUpdate")
	target, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}

	req := &struct {
		Password    string `json:"password"`
		NewPassword string `json:"new_password"`
	}{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if len(req.Password) == 0 || len(req.NewPassword) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	code, salt := verifyUserPasswordByUid(ctx, target, req.Password)
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}

	newDigest := hashPassword(req.NewPassword, salt)
	n, err := utils.SqlModify(ctx, userSql.update, newDigest[:], target)
	if err != nil {
		ctx.Errorf("fail to run sql[user.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func userDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "userDelete")
	target, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}

	req := &struct {
		Password string `json:"password"`
	}{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if len(req.Password) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	code, _ := verifyUserPasswordByUid(ctx, target, req.Password)
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}

	n, err := utils.SqlModify(ctx, userSql.remove, target)
	if err != nil {
		ctx.Errorf("fail to run sql[user.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

type appPrivilege struct {
	Name      string `json:"name"`
	Privilege int    `json:"privilege"`
	Grantor   string `json:"grantor"`
}

func appGetPrivilege(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appGetPrivilege")
	appId, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, appId, privilegeAdmin); !ok {
		return
	}

	resp := make([]appPrivilege, 0)
	code := queryRows(ctx, "priv.getListByApp",
		func() (*sql.Rows, error) { return privSql.getListByApp.QueryContext(ctx, appId) },
		func(rows *sql.Rows) error {
			var rec appPrivilege
			if err := rows.Scan(&rec.Name, &rec.Privilege, &rec.Grantor); err != nil {
				return err
			}
			resp = append(resp, rec)
			return nil
		})
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &resp)
}

func appGrantPrivilege(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appGrantPrivilege")
	appId, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	self, ok := requireSession(ctx, w, r)
	if !ok {
		return
	}

	req := &struct {
		Name      string `json:"name"`
		Privilege int    `json:"privilege"`
	}{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if len(req.Name) == 0 ||
		req.Privilege < int(privilegeNoAccess) || req.Privilege > int(privilegeAdmin) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	allowed, err := checkAppPrivilege(ctx, self, appId, privilegeAdmin)
	if err != nil {
		ctx.Errorf("fail to check privilege: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !allowed {
		ctx.Debugf("privilege grant rejected: no admin privilege uid=%d app=%d", self, appId)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var target uint32
	err = privSql.getUidByName.QueryRowContext(ctx, req.Name).Scan(&target)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[priv.getUidByName]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if req.Privilege == int(privilegeNoAccess) {
		if _, err := utils.SqlModify(ctx, privSql.remove, target, appId); err != nil {
			ctx.Errorf("fail to run sql[priv.remove]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		if _, err := utils.SqlModify(ctx, privSql.update,
			target, appId, req.Privilege, self,
			req.Privilege, self); err != nil {
			ctx.Errorf("fail to run sql[priv.update]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if err := rds.Del(ctx, makePrivilegeKey(target)).Err(); err != nil {
		ctx.Warnf("fail to clear privilege cache: %v", err)
	}
}
