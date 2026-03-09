package main

import (
	"context"
	"database/sql"
	"net/http"

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
		"SELECT `lyr_id`,`name` FROM `exp_layer` WHERE `exp_id`=? ORDER BY `lyr_id` ASC")
	if err != nil {
		return err
	}
	lyrSql.getOne, err = db.Prepare(
		"SELECT `name`,`version` FROM `exp_layer` " +
			"WHERE `lyr_id`=?")
	if err != nil {
		return err
	}
	lyrSql.create, err = db.Prepare(
		"INSERT INTO `exp_layer`(`exp_id`,`name`) " +
			"VALUES (?,?)")
	if err != nil {
		return err
	}
	lyrSql.update, err = db.Prepare(
		"UPDATE `exp_layer` SET `name`=?,`version`=? " +
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
}

func bindLyrOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/lyr", lyrCreate)
	router.Handle(http.MethodGet, "/api/lyr/:id", lyrGetOne)
	router.Handle(http.MethodPut, "/api/lyr/:id", lyrUpdate)
	router.Handle(http.MethodDelete, "/api/lyr/:id", lyrDelete)

	router.Handle(http.MethodPost, "/api/lyr/:id/rebalance", lyrRebalance)
}

func lyrGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("lyrGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireLyrPrivilege(logger, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		lyrDetail
		Segment []segSummary `json:"segment"`
	}{}
	resp.Id = id
	if !withTx(r.Context(), logger, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(ctx context.Context, tx *sql.Tx) int {
		err := tx.Stmt(lyrSql.getOne).QueryRowContext(ctx, id).Scan(&resp.Name, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			logger.Errorf("fail to run sql[lyr.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		return queryRows(logger, "seg.getList",
			func() (*sql.Rows, error) { return tx.Stmt(segSql.getList).QueryContext(ctx, resp.Id) },
			func(rows *sql.Rows) error {
				var seg segSummary
				if err := rows.Scan(&seg.Id, &seg.Begin, &seg.End, &seg.Version); err != nil {
					return err
				}
				resp.Segment = append(resp.Segment, seg)
				return nil
			})
	}) {
		return
	}

	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func createLayer(ctx context.Context, logger *utils.ContextLogger, tx *sql.Tx, expId uint32, name string) (uint32, error) {
	id, err := utils.SqlCreate(ctx, tx.Stmt(lyrSql.create), expId, name)
	if err != nil {
		logger.Errorf("fail to run sql[lyr.create]: %v", err)
	} else {
		_, err = createDefaultSegment(ctx, logger, tx, uint32(id))
	}
	return uint32(id), err
}

func lyrCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("lyrCreate")
	req := &struct {
		ExpId  uint32 `json:"exp_id"`
		ExpVer uint32 `json:"exp_ver"`
		lyrSummary
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireExpPrivilege(logger, w, r, req.ExpId, privilegeReadWrite); !ok {
		return
	}

	var id uint32
	if !withTx(r.Context(), logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx context.Context, tx *sql.Tx) int {
		var err error
		id, err = createLayer(ctx, logger, tx, req.ExpId, req.Name)
		if err != nil {
			return http.StatusInternalServerError
		}
		return touch(ctx, logger, tx.Stmt(expSql.touch), req.ExpId, req.ExpVer, "exp")
	}) {
		return
	}

	resp := &lyrDetail{}
	resp.Name = req.Name
	resp.Id = id
	resp.Version = 0
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func lyrUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("lyrUpdate")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}

	req := &lyrDetail{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if len(req.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireLyrPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
		return
	}
	req.Id = id

	n, err := utils.SqlModify(r.Context(), lyrSql.update, req.Name,
		req.Version+1, req.Id, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[lyr.update]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func lyrDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("lyrDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		ExpId   uint32 `json:"exp_id"`
		ExpVer  uint32 `json:"exp_ver"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if _, ok := requireLyrPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
		return
	}

	if !withTx(r.Context(), logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx context.Context, tx *sql.Tx) int {
		n, err := utils.SqlModify(ctx, tx.Stmt(lyrSql.remove), id, req.ExpId, req.Version)
		if err != nil {
			logger.Errorf("fail to run sql[lyr.remove]: %v", err)
			return http.StatusInternalServerError
		}
		if n == 0 {
			logger.Warnf("operation conflict: %d", id)
			return http.StatusConflict
		}
		return touch(ctx, logger, tx.Stmt(expSql.touch), req.ExpId, req.ExpVer, "exp")
	}) {
		return
	}
}

func lyrRebalance(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("lyrRebalance")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		Version uint32       `json:"version"`
		Segment []segSummary `json:"segment,omitempty"`
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if len(req.Segment) < 2 ||
		req.Segment[0].Begin != 0 || req.Segment[len(req.Segment)-1].End != 100 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := requireLyrPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
		return
	}
	book := make(map[uint32]uint32)
	for i := 0; i < len(req.Segment); i++ {
		seg := &req.Segment[i]
		if _, got := book[seg.Id]; got || seg.Begin > seg.End {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		book[seg.Id] = seg.Version
	}
	for i := 1; i < len(req.Segment); i++ {
		if req.Segment[i].Begin != req.Segment[i-1].End {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	segment := make([]segSummary, 0, len(req.Segment))
	if code := queryRows(logger, "seg.getList",
		func() (*sql.Rows, error) { return segSql.getList.QueryContext(r.Context(), id) },
		func(rows *sql.Rows) error {
			var seg segSummary
			if err := rows.Scan(&seg.Id, &seg.Begin, &seg.End, &seg.Version); err != nil {
				return err
			}
			segment = append(segment, seg)
			return nil
		}); code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if len(segment) != len(req.Segment) {
		w.WriteHeader(http.StatusConflict)
		return
	}
	for _, seg := range segment {
		if ver, got := book[seg.Id]; !got || ver != seg.Version {
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	if !withTx(r.Context(), logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx context.Context, tx *sql.Tx) int {
		for i := 0; i < len(req.Segment); i++ {
			seg := &req.Segment[i]
			n, err := utils.SqlModify(ctx, tx.Stmt(segSql.adjust), seg.Begin, seg.End, seg.Id)
			if err != nil {
				logger.Errorf("fail to run sql[seg.adjust]: %v", err)
				return http.StatusInternalServerError
			}
			if n == 0 {
				logger.Warnf("operation conflict: %d", id)
				return http.StatusConflict
			}
		}
		return touch(ctx, logger, tx.Stmt(lyrSql.touch), id, req.Version, "lyr")
	}) {
		return
	}
}
