package main

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
)

var grpSql struct {
	getList *sql.Stmt
	getOne  *sql.Stmt
	create  *sql.Stmt
	update  *sql.Stmt
	remove  *sql.Stmt

	getDft *sql.Stmt
	getMap *sql.Stmt
	adjust *sql.Stmt
}

var cfgSql struct {
	getList *sql.Stmt
	create  *sql.Stmt
}

type grpSummary struct {
	Id        uint32 `json:"id"`
	Share     uint32 `json:"share"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default,omitempty"`
}

type grpDetail struct {
	grpSummary
	Version  uint32   `json:"version"`
	CfgId    uint32   `json:"cfg_id,omitempty"`
	ForceHit []string `json:"force_hit,omitempty"`
	Config   string   `json:"config,omitempty"`
}

type cfgSummary struct {
	Id      uint32 `json:"id"`
	Content string `json:"config"`
}

func prepareGrpSql(db *sql.DB) (err error) {
	grpSql.getList, err = db.Prepare(
		"SELECT `grp_id`,`name`,`share`,`is_default` FROM `exp_group` " +
			"WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	grpSql.getOne, err = db.Prepare("SELECT t1.*," +
		"COALESCE(`content`,'') AS `content` FROM " +
		"(SELECT `name`,`share`,`is_default`,`force_hit`," +
		"`version`,`cfg_id` FROM `exp_group` WHERE `grp_id`=? ) t1 " +
		"LEFT JOIN " +
		"( SELECT `cfg_id`,`content` FROM `exp_config` ) t2 " +
		"ON t1.cfg_id = t2.cfg_id")
	if err != nil {
		return err
	}
	grpSql.create, err = db.Prepare(
		"INSERT INTO `exp_group`(`seg_id`,`name`," +
			"`share`,`bitmap`,`is_default`) VALUES (?,?,?,?,?)")
	if err != nil {
		return err
	}
	grpSql.update, err = db.Prepare(
		"UPDATE `exp_group` SET `name`=?,`force_hit`=?," +
			"`cfg_id`=?,`version`=? WHERE `grp_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	grpSql.remove, err = db.Prepare(
		"DELETE FROM `exp_group` WHERE `grp_id`=? AND `seg_id`=? AND " +
			"`version`=? AND `share`=0 AND `is_default`=0")
	if err != nil {
		return err
	}

	grpSql.getDft, err = db.Prepare(
		"SELECT `grp_id`,`share`,`bitmap` FROM `exp_group` " +
			"WHERE `seg_id`=? AND `is_default`=1")
	if err != nil {
		return err
	}
	grpSql.getMap, err = db.Prepare(
		"SELECT `share`,`bitmap` FROM `exp_group` " +
			"WHERE `grp_id`=? AND `seg_id`=?")
	if err != nil {
		return err
	}
	grpSql.adjust, err = db.Prepare(
		"UPDATE `exp_group` SET `share`=?,`bitmap`=? WHERE `grp_id`=?")
	if err != nil {
		return err
	}

	cfgSql.getList, err = db.Prepare(
		"SELECT `cfg_id`,`content` FROM `exp_config` " +
			"WHERE `grp_id`=? AND create_time>=?")
	if err != nil {
		return err
	}
	cfgSql.create, err = db.Prepare(
		"INSERT INTO `exp_config`(`grp_id`,`content`) VALUES (?,?)")
	if err != nil {
		return err
	}
	return nil
}

func bindGrpOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/grp", grpCreate)
	router.Handle(http.MethodGet, "/api/grp/:id", grpGetOne)
	router.Handle(http.MethodPut, "/api/grp/:id", grpUpdate)
	router.Handle(http.MethodDelete, "/api/grp/:id", grpDelete)

	router.Handle(http.MethodGet, "/api/grp/:id/cfg", cfgGetList)
	router.Handle(http.MethodPost, "/api/grp/:id/cfg", cfgCreate)
}

func grpGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp := &grpDetail{}
	resp.Id = uint32(id)

	var forceHit string
	err = grpSql.getOne.QueryRow(id).Scan(&resp.Name,
		&resp.Share, &resp.IsDefault, &forceHit,
		&resp.Version, &resp.CfgId, &resp.Config)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[grp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if len(forceHit) != 0 {
		resp.ForceHit = strings.Split(forceHit, ",")
	}

	utils.HttpReplyJsonWithLog(w, http.StatusOK, resp)
}

func createDefultGroup(tx *sql.Tx, segId uint32) (uint32, error) {
	var bitmap [125]byte
	for i := 0; i < 125; i++ {
		bitmap[i] = 0xff
	}
	id, err := utils.SqlCreate(tx.Stmt(grpSql.create),
		segId, "DEFAULT", 1000, bitmap[:], true)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[grp.create]: %v", err)
	}
	return uint32(id), err
}

func grpCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	req := &struct {
		SegId  uint32 `json:"seg_id"`
		SegVer uint32 `json:"seg_ver"`
		grpSummary
	}{}
	err := utils.HttpGetJsonArgsWithLog(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	})
	if err != nil {
		utils.GetLogger().Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var bitmap [125]byte //zeros
	id, err := utils.SqlCreate(tx.Stmt(grpSql.create),
		req.SegId, req.Name, 0, bitmap[:], false)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[grp.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	code := touch(tx.Stmt(segSql.touch), req.SegId, req.SegVer, "seg", "grpCreate")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		utils.GetLogger().Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &grpDetail{}
	resp.Name = req.Name
	resp.Id = uint32(id)
	resp.Share = 0
	resp.CfgId = 0
	resp.Config = ""
	resp.Version = 0
	utils.HttpReplyJsonWithLog(w, http.StatusOK, resp)
}

func grpUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &grpDetail{}
	err = utils.HttpGetJsonArgsWithLog(r, req)
	if err != nil || len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Id = uint32(id)

	n, err := utils.SqlModify(grpSql.update, req.Name,
		strings.Join(req.ForceHit, ","), req.CfgId,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[grp.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[grpUpdate] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	resp := &grpDetail{}
	resp.Id = uint32(id)

	var forceHit string
	err = grpSql.getOne.QueryRow(id).Scan(&resp.Name,
		&resp.Share, &resp.IsDefault, &forceHit,
		&resp.Version, &resp.CfgId, &resp.Config)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			utils.GetLogger().Errorf("fail to run sql[grp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if len(forceHit) != 0 {
		resp.ForceHit = strings.Split(forceHit, ",")
	}

	utils.HttpReplyJsonWithLog(w, http.StatusOK, resp)
}

func grpDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		SegId   uint32 `json:"seg_id"`
		SegVer  uint32 `json:"seg_ver"`
		Version uint32 `json:"version"`
	}{}
	if err = utils.HttpGetJsonArgsWithLog(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	})
	if err != nil {
		utils.GetLogger().Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	n, err := utils.SqlModify(tx.Stmt(grpSql.remove), id, req.SegId, req.Version)
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[grp.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		utils.GetLogger().Warnf("[grpDelete] conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	code := touch(tx.Stmt(segSql.touch), req.SegId, req.SegVer, "seg", "grpDelete")
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

func cfgGetList(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	grpId, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	query := r.URL.Query()
	var begin int64
	if str := query.Get("begin"); len(str) != 0 {
		begin, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	rows, err := cfgSql.getList.Query(grpId, time.Unix(begin, 0))
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[cfg.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var resp []cfgSummary
	for rows.Next() {
		var rec cfgSummary
		err := rows.Scan(&rec.Id, &rec.Content)
		if err != nil {
			utils.GetLogger().Errorf("fail to run sql[cfg.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp = append(resp, rec)
	}
	utils.HttpReplyJsonWithLog(w, http.StatusOK, &resp)
}

func cfgCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	grpId, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id, err := utils.SqlCreate(cfgSql.create, grpId, utils.UnsafeBytesToString(raw))
	if err != nil {
		utils.GetLogger().Errorf("fail to run sql[cfg.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &cfgSummary{Id: uint32(id)}
	utils.HttpReplyJsonWithLog(w, http.StatusOK, resp)
}
