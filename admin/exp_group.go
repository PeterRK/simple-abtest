package main

import (
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
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
	getOne  *sql.Stmt
	create  *sql.Stmt
}

type grpSummary struct {
	Id        uint32 `json:"id"`
	Share     uint32 `json:"share"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default,omitempty"`
	Version   uint32 `json:"version,omitempty"`
}

type grpDetail struct {
	grpSummary
	CfgId    uint32   `json:"cfg_id,omitempty"`
	ForceHit []string `json:"force_hit,omitempty"`
	CfgStamp string   `json:"cfg_stamp,omitempty"`
	Config   string   `json:"config,omitempty"`
}

type cfgSummary struct {
	Id    uint32 `json:"id"`
	Stamp string `json:"stamp,omitempty"`
}

func prepareGrpSql(db *sql.DB) (err error) {
	grpSql.getList, err = db.Prepare(
		"SELECT `grp_id`,`name`,`share`,`is_default`,`version` FROM `exp_group` " +
			"WHERE `seg_id`=? ORDER BY `grp_id` ASC")
	if err != nil {
		return err
	}
	grpSql.getOne, err = db.Prepare("SELECT t1.*," +
		"COALESCE(`stamp`,0),COALESCE(`content`,'') AS `content` FROM " +
		"( SELECT `name`,`share`,`is_default`,`force_hit`," +
		"`version`,`cfg_id` FROM `exp_group` WHERE `grp_id`=? ) t1 " +
		"LEFT JOIN " +
		"( SELECT `cfg_id`,`stamp`,`content` " +
		"FROM `exp_config` ) t2 " +
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
		"SELECT `cfg_id`,`stamp` FROM `exp_config` " +
			"WHERE `grp_id`=? AND stamp>=? ORDER BY `cfg_id` ASC")
	if err != nil {
		return err
	}
	cfgSql.getOne, err = db.Prepare(
		"SELECT `content` FROM `exp_config` WHERE `cfg_id`=? AND `grp_id`=?")
	if err != nil {
		return err
	}
	cfgSql.create, err = db.Prepare(
		"INSERT INTO `exp_config`(`grp_id`,`stamp`,`content`) VALUES (?,?,?)")
	if err != nil {
		return err
	}
	return nil
}

func bindGrpOp(router *httprouter.Router) {
	router.Handle(http.MethodPost, "/api/grp", grpCreate)
	router.Handle(http.MethodGet, "/api/grp/:id", grpGetOne)
	router.Handle(http.MethodPut, "/api/grp/:id", grpUpdate)
	router.Handle(http.MethodDelete, "/api/grp/:id", grpDelete)

	router.Handle(http.MethodGet, "/api/grp/:id/cfg", cfgGetList)
	router.Handle(http.MethodPost, "/api/grp/:id/cfg", cfgCreate)
	router.Handle(http.MethodGet, "/api/grp/:id/cfg/:cid", cfgGetOne)
}

func stampToStr(stamp int64) string {
	if stamp <= 0 {
		return ""
	}
	return time.Unix(stamp, 0).Format(time.DateTime)
}

func grpGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "grpGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &grpDetail{}
	resp.Id = id

	var stamp int64
	var forceHit string
	err := grpSql.getOne.QueryRowContext(ctx, id).Scan(&resp.Name,
		&resp.Share, &resp.IsDefault, &forceHit,
		&resp.Version, &resp.CfgId, &stamp, &resp.Config)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[grp.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	if len(forceHit) != 0 {
		resp.ForceHit = strings.Split(forceHit, ",")
	}
	resp.CfgStamp = stampToStr(stamp)

	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func createDefultGroup(ctx *Context, tx *sql.Tx, segId uint32) (uint32, error) {
	var bitmap [125]byte
	for i := 0; i < 125; i++ {
		bitmap[i] = 0xff
	}
	id, err := utils.SqlCreate(ctx, tx.Stmt(grpSql.create),
		segId, "DEFAULT", 1000, bitmap[:], true)
	if err != nil {
		ctx.Errorf("fail to run sql[grp.create]: %v", err)
	}
	return uint32(id), err
}

func grpCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "grpCreate")
	req := &struct {
		SegId  uint32 `json:"seg_id"`
		SegVer uint32 `json:"seg_ver"`
		grpSummary
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireSegPrivilege(ctx, w, r, req.SegId, privilegeReadWrite); !ok {
		return
	}

	var id uint32
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		var bitmap [125]byte //zeros
		rawID, err := utils.SqlCreate(ctx, tx.Stmt(grpSql.create),
			req.SegId, req.Name, 0, bitmap[:], false)
		if err != nil {
			ctx.Errorf("fail to run sql[grp.create]: %v", err)
			return http.StatusInternalServerError
		}
		id = uint32(rawID)
		return touch(ctx, tx.Stmt(segSql.touch), req.SegId, req.SegVer, "seg")
	}) {
		return
	}

	resp := &grpDetail{}
	resp.Name = req.Name
	resp.Id = id
	resp.Share = 0
	resp.CfgId = 0
	resp.Config = ""
	resp.Version = 0
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func grpUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "grpUpdate")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}

	req := &grpDetail{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}
	req.Id = id

	n, err := utils.SqlModify(ctx, grpSql.update, req.Name,
		strings.Join(req.ForceHit, ","), req.CfgId,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		ctx.Errorf("fail to run sql[grp.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func grpDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "grpDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		SegId   uint32 `json:"seg_id"`
		SegVer  uint32 `json:"seg_ver"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		n, err := utils.SqlModify(ctx, tx.Stmt(grpSql.remove), id, req.SegId, req.Version)
		if err != nil {
			ctx.Errorf("fail to run sql[grp.remove]: %v", err)
			return http.StatusInternalServerError
		}
		if n == 0 {
			ctx.Warnf("operation conflict: %d", id)
			return http.StatusConflict
		}
		return touch(ctx, tx.Stmt(segSql.touch), req.SegId, req.SegVer, "seg")
	}) {
		return
	}
}

func cfgGetList(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "cfgGetList")
	grpId, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, grpId, privilegeReadOnly); !ok {
		return
	}
	query := r.URL.Query()
	var begin int64
	var err error
	if str := query.Get("begin"); len(str) != 0 {
		begin, err = strconv.ParseInt(str, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	var resp []cfgSummary
	code := queryRows(ctx, "cfg.getList",
		func() (*sql.Rows, error) { return cfgSql.getList.QueryContext(ctx, grpId, begin) },
		func(rows *sql.Rows) error {
			var id uint32
			var stamp int64
			if err := rows.Scan(&id, &stamp); err != nil {
				return err
			}
			resp = append(resp, cfgSummary{
				Id:    id,
				Stamp: stampToStr(stamp),
			})
			return nil
		})
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &resp)
}

func cfgGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "cfgGetOne")
	grpId, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, grpId, privilegeReadOnly); !ok {
		return
	}
	cfgId, ok := parseUintParam(w, p, "cid")
	if !ok {
		return
	}

	var content string
	err := cfgSql.getOne.QueryRowContext(ctx, cfgId, grpId).Scan(&content)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[cfg.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	ctx.Debugf("get config by id: %d", cfgId)
	w.Header().Set("Content-Type", "text/plain")
	w.Write(utils.UnsafeStringToBytes(content))
}

func cfgCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "cfgCreate")
	grpId, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireGrpPrivilege(ctx, w, r, grpId, privilegeReadWrite); !ok {
		return
	}

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	stamp := time.Now().Unix()
	id, err := utils.SqlCreate(ctx, cfgSql.create, grpId,
		stamp, utils.UnsafeBytesToString(raw))
	if err != nil {
		ctx.Errorf("fail to run sql[cfg.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &cfgSummary{
		Id:    uint32(id),
		Stamp: stampToStr(stamp),
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}
