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

var appSql struct {
	getList *sql.Stmt
	getOne  *sql.Stmt
	create  *sql.Stmt
	update  *sql.Stmt
	remove  *sql.Stmt
	touch   *sql.Stmt
}

func prepareAppSql(db *sql.DB) (err error) {
	appSql.getList, err = db.Prepare(
		"SELECT `app_id`,`name` FROM `application`")
	if err != nil {
		return err
	}
	appSql.getOne, err = db.Prepare(
		"SELECT `name`,`description`,`version` FROM `application` " +
			"WHERE `app_id`=?")
	if err != nil {
		return err
	}
	appSql.create, err = db.Prepare(
		"INSERT INTO `application`(`name`,`description`) VALUES (?,?)")
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

func bindAppOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/app", appCreate)
	router.Handle(http.MethodGet, "/api/app", appGetList)
	router.Handle(http.MethodGet, "/api/app/:id", appGetOne)
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
	rows, err := appSql.getList.Query()
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[app.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var resp []appSummary
	for rows.Next() {
		var rec appSummary
		err := rows.Scan(&rec.Id, &rec.Name)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[app.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp = append(resp, rec)
	}
	utils.HttpReplyJson(w, http.StatusOK, &resp)
}

func appGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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
		appDetail
		Experiment []expSummary `json:"experiment"`
	}{}
	resp.Id = uint32(id)

	err = tx.Stmt(appSql.getOne).QueryRow(id).Scan(
		&resp.Name, &resp.Desc, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[app.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	rows, err := tx.Stmt(expSql.getList).Query(resp.Id)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var exp expSummary
		err = rows.Scan(&exp.Id, &exp.Name, &exp.Status)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[exp.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.Experiment = append(resp.Experiment, exp)
	}

	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func appCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	req := &appDetail{}
	err := utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := utils.SqlCreate(appSql.create, req.Name, req.Desc)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[app.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := req
	resp.Id = uint32(id)
	resp.Version = 0
	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func appUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &appDetail{}
	err = utils.HttpGetJsonArgs(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = uint32(id)

	n, err := utils.SqlModify(appSql.update, req.Name, req.Desc,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[app.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[appUpdate] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	resp := req
	resp.Version++
	utils.HttpReplyJson(w, http.StatusOK, resp)
}

func appDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
	}{}
	if err = utils.HttpGetJsonArgs(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cnt := 0
	err = expSql.count.QueryRow(id).Scan(&cnt)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[exp.count]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		utils.GetLogger().Warnf("try to delete application with experiments: %d", id)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	n, err := utils.SqlModify(appSql.remove, id, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[app.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[appDelete] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func touch(stmt *sql.Stmt, id, version uint32, hint1, hint2 string) int {
	n, err := utils.SqlModify(stmt, version+1, id, version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[%s.touch]: %v", hint1, err)
		return http.StatusInternalServerError
	}
	if n == 0 {
		utils.GetLogger().Warnf("[%s] conflict: %d", hint2, id)
		return http.StatusConflict
	}
	return http.StatusOK
}
