package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
)

var lyrSql struct {
	getList *sql.Stmt
	getOne  *sql.Stmt
	create  *sql.Stmt
	update  *sql.Stmt
	remove  *sql.Stmt
	touch   *sql.Stmt
}

func prepareLyrSql(db *sql.DB) (err error) {
	lyrSql.getList, err = db.Prepare(
		"SELECT `lyr_id`,`name` FROM `exp_layer` WHERE `exp_id`=?")
	if err != nil {
		return err
	}
	lyrSql.getOne, err = db.Prepare(
		"SELECT `name`,`description`,`version` FROM `exp_layer` " +
			"WHERE `lyr_id`=?")
	if err != nil {
		return err
	}
	lyrSql.create, err = db.Prepare(
		"INSERT INTO `exp_layer`(`exp_id`,`name`,`description`) " +
			"VALUES (?,?,?)")
	if err != nil {
		return err
	}
	lyrSql.update, err = db.Prepare(
		"UPDATE `exp_layer` SET `name`=?,`description`=?,`version`=? " +
			"WHERE `lyr_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	lyrSql.remove, err = db.Prepare(
		"DELETE FROM `exp_layer` WHERE `lyr_id`=? AND `exp_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	lyrSql.touch, err = db.Prepare(
		"UPDATE `exp_layer` SET `version`=? WHERE `lyr_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	return nil
}

type lyrSummary struct {
	Id   uint32 `json:"id"`
	Name string `json:"name"`
}

type lyrDetail struct {
	lyrSummary
	Version uint32 `json:"version"`
	Desc    string `json:"description,omitempty"`
}

func bindLyrOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/lyr", lyrCreate)
	router.Handle(http.MethodGet, "/api/lyr/:id", lyrGetOne)
	router.Handle(http.MethodPut, "/api/lyr/:id", lyrUpdate)
	router.Handle(http.MethodDelete, "/api/lyr/:id", lyrDelete)

	router.Handle(http.MethodPost, "/api/lyr/:id/rebalance", lyrRebalance)
}

func lyrGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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
		lyrDetail
		Segment []segSummary `json:"segment"`
	}{}
	resp.Id = uint32(id)

	err = tx.Stmt(lyrSql.getOne).QueryRow(id).Scan(
		&resp.Name, &resp.Desc, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[lyr.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	rows, err := tx.Stmt(segSql.getList).Query(resp.Id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[seg.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var seg segSummary
		err = rows.Scan(&seg.Id, &seg.Begin, &seg.End)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[seg.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.Segment = append(resp.Segment, seg)
	}

	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func createLayer(tx *sql.Tx, expId uint32, name, desc string) (uint32, error) {
	id, err := utils.SqlCreate(tx.Stmt(lyrSql.create), expId, name, desc)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[lyr.create]: %v", err)
	} else {
		_, err = createDefaultSegment(tx, uint32(id))
	}
	return uint32(id), err
}

func lyrCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	req := &struct {
		ExpId  uint32 `json:"exp_id"`
		ExpVer uint32 `json:"exp_ver"`
		lyrDetail
	}{}
	err := utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
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

	id, err := createLayer(tx, req.ExpId, req.Name, req.Desc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	code := touch(tx.Stmt(expSql.touch), req.ExpId, req.ExpVer, "exp", "lyrCreate")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		utils.GetLogger().Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &req.lyrDetail
	resp.Id = uint32(id)
	resp.Version = 0
	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func lyrUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &lyrDetail{}
	err = utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = uint32(id)

	n, err := utils.SqlModify(lyrSql.update, req.Name, req.Desc,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[lyr.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[lyrUpdate] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	resp := req
	resp.Version++
	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func lyrDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		ExpId   uint32 `json:"exp_id"`
		ExpVer  uint32 `json:"exp_ver"`
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

	n, err := utils.SqlModify(tx.Stmt(lyrSql.remove), id, req.ExpId, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[lyr.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[lyrDelete] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	code := touch(tx.Stmt(expSql.touch), req.ExpId, req.ExpVer, "exp", "lyrDelete")
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

func lyrRebalance(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		Version uint32       `json:"version"`
		Segment []segSummary `json:"segment"`
	}{}
	err = utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Segment) < 2 ||
		req.Segment[0].Begin != 0 || req.Segment[len(req.Segment)-1].End != 100 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	set := make(map[uint32]bool)
	for i := 0; i < len(req.Segment); i++ {
		seg := &req.Segment[i]
		if set[seg.Id] || seg.Begin > seg.End {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		set[seg.Id] = true
	}
	for i := 1; i < len(req.Segment); i++ {
		if req.Segment[i].Begin != req.Segment[i-1].End {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	rows, err := segSql.getList.Query(id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[seg.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cnt := 0
	for rows.Next() {
		var seg segSummary
		err = rows.Scan(&seg.Id, &seg.Begin, &seg.End)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[seg.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !set[seg.Id] {
			w.WriteHeader(http.StatusConflict)
			return
		}
		cnt++
	}
	if cnt != len(req.Segment) {
		w.WriteHeader(http.StatusConflict)
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

	for i := 0; i < len(req.Segment); i++ {
		seg := &req.Segment[i]
		n, err := utils.SqlModify(tx.Stmt(segSql.adjust), seg.Begin, seg.End, seg.Id)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[seg.adjust]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if n == 0 {
			utils.GetLogger().Warnf("[lyrRebalance] conflict: %d", id)
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	code := touch(tx.Stmt(lyrSql.touch), uint32(id), req.Version, "lyr", "lyrRebalance")
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
