package main

import (
	"database/sql"
	"encoding/json"
	"math/rand/v2"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/core"
	"github.com/peterrk/simple-abtest/utils"
)

var expSql struct {
	count   *sql.Stmt
	getList *sql.Stmt
	getOne  *sql.Stmt
	create  *sql.Stmt
	update  *sql.Stmt
	remove  *sql.Stmt
	touch   *sql.Stmt
	shuffle *sql.Stmt
	toggle  *sql.Stmt
}

func prepareExpSql(db *sql.DB) (err error) {
	expSql.count, err = db.Prepare(
		"SELECT COUNT(*) FROM `experiment` WHERE `app_id`=?")
	if err != nil {
		return err
	}
	expSql.getList, err = db.Prepare(
		"SELECT `exp_id`,`name`,`description`,`status`,`version` FROM `experiment` " +
			"WHERE `app_id`=? ORDER BY `exp_id` ASC")
	if err != nil {
		return err
	}
	expSql.getOne, err = db.Prepare(
		"SELECT `app_id`,`name`,`description`,`status`,`filter`,`version` " +
			"FROM `experiment` WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	expSql.create, err = db.Prepare(
		"INSERT INTO `experiment`(`app_id`,`name`,`description`,`seed`) " +
			"VALUES (?,?,?,?)")
	if err != nil {
		return err
	}
	expSql.update, err = db.Prepare(
		"UPDATE `experiment` SET `name`=?,`description`=?,`filter`=?,`version`=? " +
			"WHERE `exp_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	expSql.remove, err = db.Prepare(
		"DELETE FROM `experiment` WHERE `exp_id`=? AND `app_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	expSql.touch, err = db.Prepare(
		"UPDATE `experiment` SET `version`=? WHERE `exp_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	expSql.shuffle, err = db.Prepare(
		"UPDATE `experiment` SET `seed`=? WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	expSql.toggle, err = db.Prepare(
		"UPDATE `experiment` SET `status`=?,`version`=? WHERE `exp_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	return nil
}

type expSummary struct {
	Id      uint32 `json:"id"`
	Status  uint8  `json:"status"`
	Name    string `json:"name"`
	Desc    string `json:"description,omitempty"`
	Version uint32 `json:"version"`
}

type expDetail struct {
	expSummary
	AppId  uint32          `json:"app_id,omitempty"`
	Filter []core.ExprNode `json:"filter,omitempty"`
}

func bindExpOp(router *httprouter.Router) {
	router.Handle(http.MethodPost, "/api/exp", expCreate)
	router.Handle(http.MethodGet, "/api/exp/:id", expGetOne)
	router.Handle(http.MethodPut, "/api/exp/:id", expUpdate)
	router.Handle(http.MethodDelete, "/api/exp/:id", expDelete)

	router.Handle(http.MethodPost, "/api/exp/:id/shuffle", expShuffle)
	router.Handle(http.MethodPut, "/api/exp/:id/status", expSwitch)
}

func expGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireExpPrivilege(ctx, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		expDetail
		Layer []lyrSummary `json:"layer,omitempty"`
	}{}
	resp.Id = id
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(ctx *Context, tx *sql.Tx) int {
		var filter []byte
		err := tx.Stmt(expSql.getOne).QueryRowContext(ctx, id).Scan(
			&resp.AppId, &resp.Name, &resp.Desc, &resp.Status, &filter, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			ctx.Errorf("fail to run sql[exp.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		if len(filter) != 0 {
			err = json.Unmarshal(filter, &resp.Filter)
			if err != nil {
				ctx.Errorf("broken filter json in experiment %d", id)
				return http.StatusInternalServerError
			}
		}

		return queryRows(ctx, "lyr.getList",
			func() (*sql.Rows, error) { return tx.Stmt(lyrSql.getList).QueryContext(ctx, resp.Id) },
			func(rows *sql.Rows) error {
				var lyr lyrSummary
				if err := rows.Scan(&lyr.Id, &lyr.Name); err != nil {
					return err
				}
				resp.Layer = append(resp.Layer, lyr)
				return nil
			})
	}) {
		return
	}
	if len(resp.Layer) > 0 {
		mark := newIdMark(resp.Id)
		relationCache.lock.Lock()
		for i := 0; i < len(resp.Layer); i++ {
			lyrId := resp.Layer[i].Id
			relationCache.lyrToExp[lyrId] = mark
		}
		relationCache.lock.Unlock()
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func expCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expCreate")
	req := &struct {
		AppId  uint32 `json:"app_id"`
		AppVer uint32 `json:"app_ver"`
		expSummary
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if !validName(req.Name, maxExpNameLen) || !validName(req.Name, maxLayerNameLen) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, req.AppId, privilegeReadWrite); !ok {
		return
	}

	var id uint32
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		rawID, err := utils.SqlCreate(ctx, tx.Stmt(expSql.create),
			req.AppId, req.Name, req.Desc, rand.Uint32())
		if err != nil {
			ctx.Errorf("fail to run sql[exp.create]: %v", err)
			return http.StatusInternalServerError
		}
		id = uint32(rawID)

		if _, err = createLayer(ctx, tx, id, req.Name); err != nil {
			return http.StatusInternalServerError
		}
		return touch(ctx, tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app")
	}) {
		return
	}

	resp := &expDetail{}
	resp.Name = req.Name
	resp.Desc = req.Desc
	resp.Id = id
	resp.Status = 0
	resp.Version = 0
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func expUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expUpdate")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireExpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	req := &expDetail{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if !validName(req.Name, maxExpNameLen) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = id

	filter, err := json.Marshal(&req.Filter)
	if err == nil {
		_, err = core.ParseExpr(filter)
	}
	if err != nil {
		ctx.Warnf("illegal filter json: %v", req.Filter)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(ctx, expSql.update, req.Name, req.Desc, filter,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		ctx.Errorf("fail to run sql[exp.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func expDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		AppId   uint32 `json:"app_id"`
		AppVer  uint32 `json:"app_ver"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if _, ok := requireExpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		n, err := utils.SqlModify(ctx, tx.Stmt(expSql.remove), id, req.AppId, req.Version)
		if err != nil {
			ctx.Errorf("fail to run sql[exp.remove]: %v", err)
			return http.StatusInternalServerError
		}
		if n == 0 {
			ctx.Warnf("operation conflict: %d", id)
			return http.StatusConflict
		}
		return touch(ctx, tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app")
	}) {
		return
	}
}

func expShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expShuffle")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireExpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	n, err := utils.SqlModify(ctx, expSql.shuffle, rand.Uint32(), id)
	if err != nil {
		ctx.Errorf("fail to run sql[exp.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ctx.Infof("shuffle experiment %d", id)
}

func expSwitch(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "expSwitch")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		Status  uint8  `json:"status"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if req.Status != 0 && req.Status != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireExpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	n, err := utils.SqlModify(ctx, expSql.toggle, req.Status, req.Version+1, id, req.Version)
	if err != nil {
		ctx.Errorf("fail to run sql[exp.toggle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
	ctx.Infof("toggle experiment %d: %d", id, req.Status)
}
