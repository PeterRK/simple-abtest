package main

import (
	"database/sql"
	"math/rand/v2"
	"net/http"

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
	Id      uint32 `json:"id"`
	Begin   uint32 `json:"begin"`
	End     uint32 `json:"end"`
	Version uint32 `json:"version,omitempty"`
}

func prepareSegSql(db *sql.DB) (err error) {
	segSql.getList, err = db.Prepare(
		"SELECT `seg_id`,`range_begin`,`range_end`,`version` FROM `exp_segment` " +
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
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireSegPrivilege(logger, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		segSummary
		Group []grpSummary `json:"group,omitempty"`
	}{}
	resp.Id = id
	if !withTx(logger, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(tx *sql.Tx) int {
		err := tx.Stmt(segSql.getOne).QueryRow(id).Scan(
			&resp.Begin, &resp.End, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			logger.Errorf("fail to run sql[seg.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		rows, err := tx.Stmt(grpSql.getList).Query(resp.Id)
		if err != nil {
			logger.Errorf("fail to run sql[grp.getList]: %v", err)
			return http.StatusInternalServerError
		}
		defer rows.Close()

		for rows.Next() {
			var grp grpSummary
			err = rows.Scan(&grp.Id, &grp.Name, &grp.Share, &grp.IsDefault, &grp.Version)
			if err != nil {
				logger.Errorf("fail to run sql[grp.getList]: %v", err)
				return http.StatusInternalServerError
			}
			resp.Group = append(resp.Group, grp)
		}
		if err = rows.Err(); err != nil {
			logger.Errorf("fail to iterate sql[grp.getList]: %v", err)
			return http.StatusInternalServerError
		}
		return http.StatusOK
	}) {
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
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if _, ok := requireLyrPrivilege(logger, w, r, req.LyrId, privilegeReadWrite); !ok {
		return
	}

	var id uint32
	if !withTx(logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(tx *sql.Tx) int {
		rawID, err := utils.SqlCreate(tx.Stmt(segSql.create),
			req.LyrId, 100, rand.Uint32())
		if err != nil {
			logger.Errorf("fail to run sql[seg.create]: %v", err)
			return http.StatusInternalServerError
		}
		id = uint32(rawID)

		if _, err = createDefultGroup(logger, tx, id); err != nil {
			return http.StatusInternalServerError
		}
		return touch(logger, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
	}) {
		return
	}

	resp := &segSummary{
		Id:      id,
		Begin:   100,
		End:     100,
		Version: 0,
	}
	utils.HttpReplyJsonWithLog(logger, w, http.StatusOK, resp)
}

func segDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		LyrId   uint32 `json:"lyr_id"`
		LyrVer  uint32 `json:"lyr_ver"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if _, ok := requireSegPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
		return
	}

	if !withTx(logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(tx *sql.Tx) int {
		n, err := utils.SqlModify(tx.Stmt(segSql.remove), id, req.LyrId, req.Version)
		if err != nil {
			logger.Errorf("fail to run sql[seg.remove]: %v", err)
			return http.StatusInternalServerError
		}
		if n == 0 {
			logger.Warnf("operation conflict: %d", id)
			return http.StatusConflict
		}
		return touch(logger, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
	}) {
		return
	}
}

func segShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	logger := utils.NewContextLogger("segShuffle")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireSegPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
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
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
		GrpId   uint32 `json:"grp_id"`
		Share   uint32 `json:"share"`
	}{}
	if !getJsonArgs(logger, w, r, req) {
		return
	}
	if _, ok := requireSegPrivilege(logger, w, r, id, privilegeReadWrite); !ok {
		return
	}

	var dftId uint32
	type part struct {
		share  uint32
		bitmap [125]byte
	}
	var dft, grp part
	var tmp []byte
	var err error
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

	if !withTx(logger, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(tx *sql.Tx) int {
		adjust := func(grpId, share uint32, bitmap []byte) int {
			n, err := utils.SqlModify(tx.Stmt(grpSql.adjust), share, bitmap[:], grpId)
			if err != nil {
				logger.Errorf("fail to run sql[grp.adjust]: %v", err)
				return http.StatusInternalServerError
			}
			if n != 1 {
				logger.Warnf("operation conflict: %d", id)
				return http.StatusConflict
			}
			return http.StatusOK
		}
		code := adjust(dftId, dft.share, dft.bitmap[:])
		if code != http.StatusOK {
			return code
		}
		code = adjust(req.GrpId, grp.share, grp.bitmap[:])
		if code != http.StatusOK {
			return code
		}
		return touch(logger, tx.Stmt(segSql.touch), id, req.Version, "seg")
	}) {
		return
	}
}
