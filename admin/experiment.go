package main

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"net/http"
	"strconv"

	json "github.com/goccy/go-json"
	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/core"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
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
		"SELECT `exp_id`,`name`,`status` FROM `experiment` " +
			"WHERE `app_id`=?")
	if err != nil {
		return err
	}
	expSql.getOne, err = db.Prepare(
		"SELECT `name`,`description`,`status`,`filter`,`version` " +
			"FROM `experiment` WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	expSql.create, err = db.Prepare(
		"INSERT INTO `experiment`(`app_id`,`name`,`description`,`seed`,`filter`) " +
			"VALUES (?,?,?,?,?)")
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
		"UPDATE `experiment` SET `status`=? WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	return nil
}

type expSummary struct {
	Id     uint32 `json:"id"`
	Status uint8  `json:"status"`
	Name   string `json:"name"`
}

type expDetail struct {
	expSummary
	Version uint32          `json:"version"`
	Desc    string          `json:"description,omitempty"`
	Filter  []core.ExprNode `json:"filter,omitempty"`
}

func bindExpOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/exp", expCreate)
	router.Handle(http.MethodGet, "/api/exp/:id", expGetOne)
	router.Handle(http.MethodPut, "/api/exp/:id", expUpdate)
	router.Handle(http.MethodDelete, "/api/exp/:id", expDelete)

	router.Handle(http.MethodPost, "/api/exp/:id/shuffle", expShuffle)
	router.Handle(http.MethodPut, "/api/exp/:id/switch", expSwitch)
}

func expGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		utils.GetLogger().Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	resp := &struct {
		expDetail
		Layer []lyrSummary `json:"layer"`
	}{}
	resp.Id = uint32(id)

	var filter []byte
	err = tx.Stmt(expSql.getOne).QueryRow(id).Scan(
		&resp.Name, &resp.Desc, &resp.Status, &filter, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[exp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	err = json.Unmarshal(filter, &resp.Filter)
	if err != nil {
		utils.GetLogger().Errorf("broken filter json in experiment %d", id)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := tx.Stmt(lyrSql.getList).Query(resp.Id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[lyr.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var lyr lyrSummary
		err = rows.Scan(&lyr.Id, &lyr.Name)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[lyr.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.Layer = append(resp.Layer, lyr)
	}

	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func expCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	req := &struct {
		AppId  uint32 `json:"app_id"`
		AppVer uint32 `json:"app_ver"`
		expDetail
	}{}
	err := utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	filter, err := json.Marshal(&req.Filter)
	if err == nil {
		_, err = core.ParseExpr(filter)
	}
	if err != nil {
		utils.GetLogger().Warn("illegal filter json: %v", req.Filter)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted, // 依赖乐观锁
	})
	if err != nil {
		utils.GetLogger().Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	id, err := utils.SqlCreate(tx.Stmt(expSql.create),
		req.AppId, req.Name, req.Desc, rand.Uint32(), filter)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = createLayer(tx, uint32(id), req.Name, ""); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	code := touch(tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app", "expCreate")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		utils.GetLogger().Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &req.expDetail
	resp.Id = uint32(id)
	resp.Status = 0
	resp.Version = 0
	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func expUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &expDetail{}
	err = utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = uint32(id)

	filter, err := json.Marshal(&req.Filter)
	if err == nil {
		_, err = core.ParseExpr(filter)
	}
	if err != nil {
		utils.GetLogger().Warn("illegal filter json: %v", req.Filter)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.update, req.Name, req.Desc, filter,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[expUpdate] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	resp := &expDetail{}
	resp.Id = uint32(id)

	err = expSql.getOne.QueryRow(id).Scan(
		&resp.Name, &resp.Desc, &resp.Status, &filter, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[exp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	err = json.Unmarshal(filter, &resp.Filter)
	if err != nil {
		utils.GetLogger().Errorf("broken filter json in experiment %d", id)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func expDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		AppId   uint32 `json:"app_id"`
		AppVer  uint32 `json:"app_ver"`
		Version uint32 `json:"version"`
	}{}
	if err = utils.HttpGetJsonArgs(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted, // 依赖乐观锁
	})
	if err != nil {
		utils.GetLogger().Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	n, err := utils.SqlModify(tx.Stmt(expSql.remove), id, req.AppId, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[expDelete] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	code := touch(tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app", "expDelete")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		utils.GetLogger().Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func expShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.shuffle, rand.Uint32(), id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	utils.GetLogger().Infof("suffle experiment %d", id)
}

func expSwitch(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		Status  uint8  `json:"status"`
		Version uint32 `json:"version"`
	}{}
	if err = utils.HttpGetJsonArgs(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.shuffle, rand.Uint32(), id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	utils.GetLogger().Infof("toggle experiment %d: ", id, req.Status)
}
