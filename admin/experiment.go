package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"strconv"

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
		"SELECT `exp_id`,`name`,`description`,`status`,`version` FROM `experiment` " +
			"WHERE `app_id`=? ORDER BY `exp_id` ASC")
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
	Filter []core.ExprNode `json:"filter,omitempty"`
}

func bindExpOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/exp", expCreate)
	router.Handle(http.MethodGet, "/api/exp/:id", expGetOne)
	router.Handle(http.MethodPut, "/api/exp/:id", expUpdate)
	router.Handle(http.MethodDelete, "/api/exp/:id", expDelete)

	router.Handle(http.MethodPost, "/api/exp/:id/shuffle", expShuffle)
	router.Handle(http.MethodPut, "/api/exp/:id/status", expSwitch)
}

func expGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expGetOne")
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
		logger.Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	resp := &struct {
		expDetail
		Layer []lyrSummary `json:"layer,omitempty"`
	}{}
	resp.Id = uint32(id)

	var filter []byte
	err = tx.Stmt(expSql.getOne).QueryRow(id).Scan(
		&resp.Name, &resp.Desc, &resp.Status, &filter, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			logger.Errorf("fail to run sql[exp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if len(filter) != 0 {
		err = json.Unmarshal(filter, &resp.Filter)
		if err != nil {
			logger.Errorf("broken filter json in experiment %d", id)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	rows, err := tx.Stmt(lyrSql.getList).Query(resp.Id)
	if err != nil {
		logger.Errorf("fail to run sql[lyr.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var lyr lyrSummary
		err = rows.Scan(&lyr.Id, &lyr.Name)
		if err != nil {
			logger.Errorf("fail to run sql[lyr.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.Layer = append(resp.Layer, lyr)
	}
	if err := rows.Err(); err != nil {
		logger.Errorf("fail to iterate sql[lyr.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func expCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expCreate")
	req := &struct {
		AppId  uint32 `json:"app_id"`
		AppVer uint32 `json:"app_ver"`
		expSummary
	}{}
	err := utils.HttpGetJsonArgsWithLog(logger, r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	})
	if err != nil {
		logger.Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	id, err := utils.SqlCreate(tx.Stmt(expSql.create),
		req.AppId, req.Name, req.Desc, rand.Uint32())
	if err != nil {
		logger.Errorf("fail to run sql[exp.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = createLayer(logger, tx, uint32(id), req.Name); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	code := touch(logger, tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		logger.Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &expDetail{}
	resp.Name = req.Name
	resp.Desc = req.Desc
	resp.Id = uint32(id)
	resp.Status = 0
	resp.Version = 0
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func expUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expUpdate")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &expDetail{}
	err = utils.HttpGetJsonArgsWithLog(logger, r, req)
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
		logger.Warnf("illegal filter json: %v", req.Filter)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.update, req.Name, req.Desc, filter,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[exp.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func expDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expDelete")
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
	if err = utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	})
	if err != nil {
		logger.Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	n, err := utils.SqlModify(tx.Stmt(expSql.remove), id, req.AppId, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[exp.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	code := touch(logger, tx.Stmt(appSql.touch), req.AppId, req.AppVer, "app")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		logger.Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func expShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expShuffle")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.shuffle, rand.Uint32(), id)
	if err != nil {
		logger.Errorf("fail to run sql[exp.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	logger.Infof("shuffle experiment %d", id)
}

func expSwitch(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("expSwitch")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		Status  uint8  `json:"status"`
		Version uint32 `json:"version"`
	}{}
	if err = utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Status != 0 && req.Status != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(expSql.toggle, req.Status, req.Version+1, id, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[exp.toggle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
	logger.Infof("toggle experiment %d: %d", id, req.Status)
}
