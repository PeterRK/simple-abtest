package main

import (
	"database/sql"
	"net/http"

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
	logger := utils.NewContextLogger("appGetList")
	uid, ok := verifySession(logger, w, r)
	if !ok {
		return
	}

	resp := make([]appSummary, 0)
	code := queryRows(logger, "app.getList",
		func() (*sql.Rows, error) { return appSql.getList.Query(uid) },
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
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, &resp)
}

func appGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(logger, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		appDetail
		Experiment []expSummary `json:"experiment,omitempty"`
	}{}
	resp.Id = id
	if !withTx(logger, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(tx *sql.Tx) int {
		err := tx.Stmt(appSql.getOne).QueryRow(id).Scan(
			&resp.Name, &resp.Desc, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			logger.Errorf("fail to run sql[app.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		return queryRows(logger, "exp.getList",
			func() (*sql.Rows, error) { return tx.Stmt(expSql.getList).Query(resp.Id) },
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

	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func appCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appCreate")
	uid, ok := verifySession(logger, w, r)
	if !ok {
		return
	}

	req := &appDetail{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var appId uint32
	if !withTx(logger, w, nil, func(tx *sql.Tx) int {
		id, err := utils.SqlCreate(tx.Stmt(appSql.create), req.Name, req.Desc)
		if err != nil {
			logger.Errorf("fail to run sql[app.create]: %v", err)
			return http.StatusInternalServerError
		}
		appId = uint32(id)
		if _, err = utils.SqlModify(tx.Stmt(privSql.update),
			uid, appId, privilegeAdmin, uid,
			privilegeAdmin, uid); err != nil {
			logger.Errorf("fail to run sql[priv.update]: %v", err)
			return http.StatusInternalServerError
		}
		return http.StatusOK
	}) {
		return
	}

	resp := req
	resp.Id = appId
	resp.Version = 0
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func appUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appUpdate")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(logger, w, r, id, privilegeAdmin); !ok {
		return
	}

	req := &appDetail{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = id

	n, err := utils.SqlModify(appSql.update, req.Name, req.Desc,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[app.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func appDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("appDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(logger, w, r, id, privilegeAdmin); !ok {
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}

	cnt := 0
	err := expSql.count.QueryRow(id).Scan(&cnt)
	if err != nil {
		logger.Errorf("fail to run sql[exp.count]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		logger.Warnf("try to delete application with experiments: %d", id)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	n, err := utils.SqlModify(appSql.remove, id, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[app.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}
