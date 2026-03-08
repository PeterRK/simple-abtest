package main

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
	"github.com/prometheus/client_golang/prometheus"
)

var segSql struct {
	getList *sql.Stmt
	getOne  *sql.Stmt
	create  *sql.Stmt
	remove  *sql.Stmt
	touch   *sql.Stmt
	shuffle *sql.Stmt
	adjust  *sql.Stmt
}

type segSummary struct {
	Id    uint32 `json:"id"`
	Begin uint32 `json:"begin"`
	End   uint32 `json:"end"`
}

type segDetail struct {
	segSummary
	Version uint32 `json:"version"`
}

func prepareSegSql(db *sql.DB) (err error) {
	segSql.getList, err = db.Prepare(
		"SELECT `seg_id`,`range_begin`,`range_end` FROM `exp_segment` " +
			"WHERE `lyr_id`=? ORDER BY `seg_id` ASC")
	if err != nil {
		return err
	}
	segSql.getOne, err = db.Prepare(
		"SELECT `range_begin`,`range_end`,`version` FROM `exp_segment` " +
			"WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	segSql.create, err = db.Prepare(
		"INSERT INTO `exp_segment`(`lyr_id`,`range_begin`,`range_end`,`seed`) " +
			"VALUES (?,?,100,?)")
	if err != nil {
		return err
	}
	segSql.remove, err = db.Prepare(
		"DELETE FROM `exp_segment` WHERE `seg_id`=? AND `lyr_id`=? AND " +
			"`version`=? AND `range_begin`=`range_end`")
	if err != nil {
		return err
	}
	segSql.touch, err = db.Prepare(
		"UPDATE `exp_segment` SET `version`=? WHERE `seg_id`=? AND `version`=?")
	if err != nil {
		return err
	}
	segSql.shuffle, err = db.Prepare(
		"UPDATE `exp_segment` SET `seed`=? WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	segSql.adjust, err = db.Prepare(
		"UPDATE `exp_segment` SET `range_begin`=?,`range_end`=? " +
			"WHERE `seg_id`=?")
	if err != nil {
		return err
	}
	return nil
}

func bindSegOp(router *httprouter.Router, registry *prometheus.Registry) {
	router.Handle(http.MethodPost, "/api/seg", segCreate)
	router.Handle(http.MethodGet, "/api/seg/:id", segGetOne)
	router.Handle(http.MethodDelete, "/api/seg/:id", segDelete)

	router.Handle(http.MethodPost, "/api/seg/:id/shuffle", segShuffle)
	router.Handle(http.MethodPost, "/api/seg/:id/rebalance", segRebalance)
}

func segGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segGetOne")
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
		segDetail
		Group []grpSummary `json:"group,omitempty"`
	}{}
	resp.Id = uint32(id)

	err = tx.Stmt(segSql.getOne).QueryRow(id).Scan(
		&resp.Begin, &resp.End, &resp.Version)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			logger.Errorf("fail to run sql[seg.getOne]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	rows, err := tx.Stmt(grpSql.getList).Query(resp.Id)
	if err != nil {
		logger.Errorf("fail to run sql[grp.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var grp grpSummary
		err = rows.Scan(&grp.Id, &grp.Name, &grp.Share, &grp.IsDefault)
		if err != nil {
			logger.Errorf("fail to run sql[grp.getList]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp.Group = append(resp.Group, grp)
	}
	if err := rows.Err(); err != nil {
		logger.Errorf("fail to iterate sql[grp.getList]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func createDefaultSegment(logger *utils.ContextLogger, tx *sql.Tx, lyrId uint32) (uint32, error) {
	id, err := utils.SqlCreate(tx.Stmt(segSql.create), lyrId, 0, rand.Uint32())
	if err != nil {
		logger.Errorf("fail to run sql[seg.create]: %v", err)
	} else {
		_, err = createDefultGroup(logger, tx, uint32(id))
	}
	return uint32(id), err
}

func segCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segCreate")
	req := &struct {
		LyrId  uint32 `json:"lyr_id"`
		LyrVer uint32 `json:"lyr_ver"`
	}{}
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil {
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

	id, err := utils.SqlCreate(tx.Stmt(segSql.create),
		req.LyrId, 100, rand.Uint32())
	if err != nil {
		logger.Errorf("fail to run sql[seg.create]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = createDefultGroup(logger, tx, uint32(id)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	code := touch(logger, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	if err = tx.Commit(); err != nil {
		logger.Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := &segDetail{}
	resp.Id = uint32(id)
	resp.Begin = 100
	resp.End = 100
	resp.Version = 0
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func segDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segDelete")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		LyrId   uint32 `json:"lyr_id"`
		LyrVer  uint32 `json:"lyr_ver"`
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

	n, err := utils.SqlModify(tx.Stmt(segSql.remove), id, req.LyrId, req.Version)
	if err != nil {
		logger.Errorf("fail to run sql[seg.remove]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		w.WriteHeader(http.StatusConflict)
		return
	}

	code := touch(logger, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
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

func segShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segShuffle")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	n, err := utils.SqlModify(segSql.shuffle, rand.Uint32(), id)
	if err != nil {
		logger.Errorf("fail to run sql[seg.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	logger.Infof("shuffle segment %d", id)
}

func segRebalance(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segRebalance")
	id, err := strconv.ParseUint(p.ByName("id"), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
		GrpId   uint32 `json:"grp_id"`
		Share   uint32 `json:"share"`
	}{}
	if err = utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var dftId uint32
	type part struct {
		share  uint32
		bitmap [125]byte
	}
	var dft, grp part
	var tmp []byte
	err = grpSql.getDft.QueryRow(id).Scan(&dftId, &dft.share, &tmp)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusConflict)
		} else {
			logger.Errorf("fail to run sql[grp.getDft]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	} else if len(tmp) != 125 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copy(dft.bitmap[:], tmp)
	if dftId == req.GrpId {
		logger.Warn("can not rebalance default group")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = grpSql.getMap.QueryRow(req.GrpId, id).Scan(&grp.share, &tmp)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusConflict)
		} else {
			logger.Errorf("fail to run sql[grp.getMap]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	} else if len(tmp) != 125 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copy(grp.bitmap[:], tmp)
	total := dft.share + grp.share
	if req.Share > total {
		logger.Warnf("no enough slots: %d > %d", req.Share, dft.share+grp.share)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	slots := make([]uint16, 0, total)
	for i := uint16(0); i < 125; i++ {
		for j := uint16(0); j < 8; j++ {
			mask := byte(1 << j)
			if (dft.bitmap[i]&mask) != 0 || (grp.bitmap[i]&mask) != 0 {
				slots = append(slots, (i<<3)|j)
			}
		}
		dft.bitmap[i] = 0
		grp.bitmap[i] = 0
	}
	if len(slots) != int(total) {
		logger.Errorf("broken group share: %d & %d", dftId, req.GrpId)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	grp.share = req.Share
	dft.share = total - req.Share

	fill := func(a, b *part) {
		// a.share >= b.share
		var xs utils.Xorshift
		xs.Init(rand.Uint32())
		for i := a.share; i < total; i++ {
			j := xs.Next() % (i + 1)
			slots[i], slots[j] = slots[j], slots[i]
		}
		for k := uint32(0); k < a.share; k++ {
			i, j := slots[k]>>3, slots[k]&7
			a.bitmap[i] |= byte(1 << j)
		}
		for k := a.share; k < total; k++ {
			i, j := slots[k]>>3, slots[k]&7
			b.bitmap[i] |= byte(1 << j)
		}
	}

	if grp.share > dft.share {
		fill(&grp, &dft)
	} else {
		fill(&dft, &grp)
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

	adjust := func(grpId, share uint32, bitmap []byte) bool {
		n, err := utils.SqlModify(tx.Stmt(grpSql.adjust), share, bitmap[:], grpId)
		if err != nil {
			logger.Errorf("fail to run sql[grp.adjust]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return false
		} else if n != 1 {
			logger.Warnf("operation conflict: %d", id)
			w.WriteHeader(http.StatusConflict)
			return false
		}
		return true
	}
	if !adjust(dftId, dft.share, dft.bitmap[:]) ||
		!adjust(req.GrpId, grp.share, grp.bitmap[:]) {
		return
	}

	code := touch(logger, tx.Stmt(segSql.touch), uint32(id), req.Version, "seg")
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
