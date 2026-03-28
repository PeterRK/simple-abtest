package main

import (
	"database/sql"
	"math"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/sign"
	"github.com/peterrk/simple-abtest/utils"
)

var appSql struct {
	getList  *sql.Stmt
	getOne   *sql.Stmt
	getToken *sql.Stmt
	create   *sql.Stmt
	update   *sql.Stmt
	remove   *sql.Stmt
	touch    *sql.Stmt
}

func prepareAppSql(db *sql.DB) (err error) {
	appSql.getList, err = db.Prepare("SELECT t2.* FROM " +
		"( SELECT `app_id` FROM `privilege` WHERE `uid`=? ) t1 " +
		"INNER JOIN " +
		"( SELECT `app_id`,`name` FROM `application` ) t2 " +
		"ON t1.app_id = t2.app_id ORDER BY t2.app_id ASC")
	if err != nil {
		return err
	}
	appSql.getOne, err = db.Prepare(
		"SELECT `name`,`description`,`version` FROM `application` " +
			"WHERE `app_id`=?")
	if err != nil {
		return err
	}
	appSql.getToken, err = db.Prepare(
		"SELECT `access_token` FROM `application` WHERE `app_id`=?")
	if err != nil {
		return err
	}
	appSql.create, err = db.Prepare(
		"INSERT INTO `application`(`name`,`description`,`access_token`) VALUES (?,?,?)")
	if err != nil {
		return err
	}
	appSql.update, err = db.Prepare(
		"UPDATE `application` SET `name`=?,`description`=?,`version`=? " +
			"WHERE `app_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	appSql.remove, err = db.Prepare(
		"DELETE FROM `application` WHERE `app_id`=? AND `version`=?")
	if err != nil {
		return err
	}

	appSql.touch, err = db.Prepare(
		"UPDATE `application` SET `version`=? WHERE `app_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	return nil
}

func bindAppOp(router *httprouter.Router) {
	router.Handle(http.MethodPost, "/api/app", appCreate)
	router.Handle(http.MethodGet, "/api/app", appGetList)
	router.Handle(http.MethodGet, "/api/app/:id", appGetOne)
	router.Handle(http.MethodPost, "/api/app/:id/token", appIssueToken)
	router.Handle(http.MethodPut, "/api/app/:id", appUpdate)
	router.Handle(http.MethodDelete, "/api/app/:id", appDelete)
}

type appSummary struct {
	Id   uint32 `json:"id"`
	Name string `json:"name"`
}

type appDetail struct {
	appSummary
	Version uint32 `json:"version"`
	Desc    string `json:"description,omitempty"`
}

func appGetList(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appGetList")
	uid, ok := verifySession(ctx, w, r)
	if !ok {
		return
	}

	resp := make([]appSummary, 0)
	code := queryRows(ctx, "app.getList",
		func() (*sql.Rows, error) { return appSql.getList.QueryContext(ctx, uid) },
		func(rows *sql.Rows) error {
			var rec appSummary
			if err := rows.Scan(&rec.Id, &rec.Name); err != nil {
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

func appGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		appDetail
		Experiment []expSummary `json:"experiment,omitempty"`
	}{}
	resp.Id = id
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(ctx *Context, tx *sql.Tx) int {
		err := tx.Stmt(appSql.getOne).QueryRowContext(ctx, id).Scan(
			&resp.Name, &resp.Desc, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			ctx.Errorf("fail to run sql[app.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		return queryRows(ctx, "exp.getList",
			func() (*sql.Rows, error) { return tx.Stmt(expSql.getList).QueryContext(ctx, resp.Id) },
			func(rows *sql.Rows) error {
				var exp expSummary
				if err := rows.Scan(&exp.Id, &exp.Name, &exp.Desc, &exp.Status, &exp.Version); err != nil {
					return err
				}
				resp.Experiment = append(resp.Experiment, exp)
				return nil
			})
	}) {
		return
	}

	if len(resp.Experiment) > 0 {
		mark := newIdMark(resp.Id)
		relationCache.lock.Lock()
		for i := 0; i < len(resp.Experiment); i++ {
			expId := resp.Experiment[i].Id
			relationCache.expToApp[expId] = mark
		}
		relationCache.lock.Unlock()
	}

	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func appCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appCreate")
	uid, ok := verifySession(ctx, w, r)
	if !ok {
		return
	}

	req := &appDetail{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if !validName(req.Name, maxAppNameLen) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	token, err := utils.GenRandomToken()
	if err != nil {
		ctx.Debug("app create failed: generate access token")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var appId uint32
	if !withTx(ctx, w, nil, func(ctx *Context, tx *sql.Tx) int {
		id, err := utils.SqlCreate(ctx, tx.Stmt(appSql.create), req.Name, req.Desc, token)
		if err != nil {
			ctx.Errorf("fail to run sql[app.create]: %v", err)
			return http.StatusInternalServerError
		}
		appId = uint32(id)
		if _, err = utils.SqlModify(ctx, tx.Stmt(privSql.update),
			uid, appId, privilegeAdmin, uid,
			privilegeAdmin, uid); err != nil {
			ctx.Errorf("fail to run sql[priv.update]: %v", err)
			return http.StatusInternalServerError
		}
		return http.StatusOK
	}) {
		return
	}

	resp := req
	resp.Id = appId
	resp.Version = 0
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func appIssueToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appIssueToken")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, id, privilegeAdmin); !ok {
		return
	}

	req := &struct {
		TTL uint32 `json:"ttl_seconds"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	now := time.Now().Unix()
	expireAt64 := now + int64(req.TTL)
	if req.TTL == 0 || expireAt64 > math.MaxUint32 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var signingSecret string
	err := appSql.getToken.QueryRowContext(ctx, id).Scan(&signingSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[app.getToken]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	expireAt := uint32(expireAt64)
	token := sign.BuildPublicToken(signingSecret, id, expireAt)
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &struct {
		Token    string `json:"token"`
		ExpireAt string `json:"expire_at"`
	}{
		Token:    token,
		ExpireAt: time.Unix(expireAt64, 0).Format(time.DateTime),
	})
}

func appUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appUpdate")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, id, privilegeAdmin); !ok {
		return
	}

	req := &appDetail{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if !validName(req.Name, maxAppNameLen) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = id

	n, err := utils.SqlModify(ctx, appSql.update, req.Name, req.Desc,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		ctx.Errorf("fail to run sql[app.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func appDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "appDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, id, privilegeAdmin); !ok {
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}

	cnt := 0
	err := expSql.count.QueryRowContext(ctx, id).Scan(&cnt)
	if err != nil {
		ctx.Errorf("fail to run sql[exp.count]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		ctx.Warnf("try to delete application with experiments: %d", id)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	n, err := utils.SqlModify(ctx, appSql.remove, id, req.Version)
	if err != nil {
		ctx.Errorf("fail to run sql[app.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}
