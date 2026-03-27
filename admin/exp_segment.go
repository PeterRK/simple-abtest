package main

import (
	"database/sql"
	"math/rand/v2"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
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
	Version uint32 `json:"version"`
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

func bindSegOp(router *httprouter.Router) {
	router.Handle(http.MethodPost, "/api/seg", segCreate)
	router.Handle(http.MethodGet, "/api/seg/:id", segGetOne)
	router.Handle(http.MethodDelete, "/api/seg/:id", segDelete)

	router.Handle(http.MethodPost, "/api/seg/:id/shuffle", segShuffle)
	router.Handle(http.MethodPost, "/api/seg/:id/rebalance", segRebalance)
}

func segGetOne(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "segGetOne")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireSegPrivilege(ctx, w, r, id, privilegeReadOnly); !ok {
		return
	}

	resp := &struct {
		segSummary
		Group []grpSummary `json:"group,omitempty"`
	}{}
	resp.Id = id
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	}, func(ctx *Context, tx *sql.Tx) int {
		err := tx.Stmt(segSql.getOne).QueryRowContext(ctx, id).Scan(
			&resp.Begin, &resp.End, &resp.Version)
		if err != nil {
			if err == sql.ErrNoRows {
				return http.StatusNotFound
			}
			ctx.Errorf("fail to run sql[seg.getOne]: %v", err)
			return http.StatusInternalServerError
		}

		return queryRows(ctx, "grp.getList",
			func() (*sql.Rows, error) { return tx.Stmt(grpSql.getList).QueryContext(ctx, resp.Id) },
			func(rows *sql.Rows) error {
				var grp grpSummary
				if err := rows.Scan(&grp.Id, &grp.Name, &grp.Share, &grp.IsDefault, &grp.Version); err != nil {
					return err
				}
				resp.Group = append(resp.Group, grp)
				return nil
			})
	}) {
		return
	}
	if len(resp.Group) > 0 {
		mark := newIdMark(resp.Id)
		relationCache.lock.Lock()
		for i := 0; i < len(resp.Group); i++ {
			grpId := resp.Group[i].Id
			relationCache.grpToSeg[grpId] = mark
		}
		relationCache.lock.Unlock()
	}

	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func createDefaultSegment(ctx *Context, tx *sql.Tx, lyrId uint32) (uint32, error) {
	id, err := utils.SqlCreate(ctx, tx.Stmt(segSql.create), lyrId, 0, rand.Uint32())
	if err != nil {
		ctx.Errorf("fail to run sql[seg.create]: %v", err)
	} else {
		_, err = createDefultGroup(ctx, tx, uint32(id))
	}
	return uint32(id), err
}

func segCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "segCreate")
	req := &struct {
		LyrId  uint32 `json:"lyr_id"`
		LyrVer uint32 `json:"lyr_ver"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if _, ok := requireLyrPrivilege(ctx, w, r, req.LyrId, privilegeReadWrite); !ok {
		return
	}

	var id uint32
	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		rawID, err := utils.SqlCreate(ctx, tx.Stmt(segSql.create),
			req.LyrId, 100, rand.Uint32())
		if err != nil {
			ctx.Errorf("fail to run sql[seg.create]: %v", err)
			return http.StatusInternalServerError
		}
		id = uint32(rawID)

		if _, err = createDefultGroup(ctx, tx, id); err != nil {
			return http.StatusInternalServerError
		}
		return touch(ctx, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
	}) {
		return
	}

	resp := &segSummary{
		Id:      id,
		Begin:   100,
		End:     100,
		Version: 0,
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func segDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "segDelete")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		LyrId   uint32 `json:"lyr_id"`
		LyrVer  uint32 `json:"lyr_ver"`
		Version uint32 `json:"version"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if _, ok := requireSegPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		n, err := utils.SqlModify(ctx, tx.Stmt(segSql.remove), id, req.LyrId, req.Version)
		if err != nil {
			ctx.Errorf("fail to run sql[seg.remove]: %v", err)
			return http.StatusInternalServerError
		}
		if n == 0 {
			ctx.Warnf("operation conflict: %d", id)
			return http.StatusConflict
		}
		return touch(ctx, tx.Stmt(lyrSql.touch), req.LyrId, req.LyrVer, "lyr")
	}) {
		return
	}
}

func segShuffle(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "segShuffle")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	if _, ok := requireSegPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
		return
	}

	n, err := utils.SqlModify(ctx, segSql.shuffle, rand.Uint32(), id)
	if err != nil {
		ctx.Errorf("fail to run sql[seg.shuffle]: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if n == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ctx.Infof("shuffle segment %d", id)
}

func segRebalance(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "segRebalance")
	id, ok := parseUintParam(w, p, "id")
	if !ok {
		return
	}
	req := &struct {
		Version uint32 `json:"version"`
		GrpId   uint32 `json:"grp_id"`
		Share   uint32 `json:"share"`
	}{}
	if !getJsonArgsWithLog(ctx, w, r, req) {
		return
	}
	if _, ok := requireSegPrivilege(ctx, w, r, id, privilegeReadWrite); !ok {
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
	err = grpSql.getDft.QueryRowContext(ctx, id).Scan(&dftId, &dft.share, &tmp)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Debugf("segment rebalance conflict: default group missing seg=%d", id)
			w.WriteHeader(http.StatusConflict)
		} else {
			ctx.Errorf("fail to run sql[grp.getDft]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	} else if len(tmp) != 125 {
		ctx.Debugf("segment rebalance failed: invalid default bitmap length seg=%d len=%d", id, len(tmp))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copy(dft.bitmap[:], tmp)
	if dftId == req.GrpId {
		ctx.Warn("can not rebalance default group")
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = grpSql.getMap.QueryRowContext(ctx, req.GrpId, id).Scan(&grp.share, &tmp)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.Debugf("segment rebalance conflict: group missing seg=%d grp=%d", id, req.GrpId)
			w.WriteHeader(http.StatusConflict)
		} else {
			ctx.Errorf("fail to run sql[grp.getMap]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	} else if len(tmp) != 125 {
		ctx.Debugf("segment rebalance failed: invalid group bitmap length seg=%d grp=%d len=%d", id, req.GrpId, len(tmp))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	copy(grp.bitmap[:], tmp)
	total := dft.share + grp.share
	if req.Share > total {
		ctx.Warnf("no enough slots: %d > %d", req.Share, dft.share+grp.share)
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
		ctx.Errorf("broken group share: %d & %d", dftId, req.GrpId)
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

	if !withTx(ctx, w, &sql.TxOptions{
		Isolation: sql.LevelReadUncommitted,
	}, func(ctx *Context, tx *sql.Tx) int {
		adjust := func(grpId, share uint32, bitmap []byte) int {
			n, err := utils.SqlModify(ctx, tx.Stmt(grpSql.adjust), share, bitmap[:], grpId)
			if err != nil {
				ctx.Errorf("fail to run sql[grp.adjust]: %v", err)
				return http.StatusInternalServerError
			}
			if n != 1 {
				ctx.Warnf("operation conflict: %d", id)
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
		return touch(ctx, tx.Stmt(segSql.touch), id, req.Version, "seg")
	}) {
		return
	}
}
