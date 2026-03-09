package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/peterrk/simple-abtest/engine/core"
)

type mysqlSource struct {
	client *sql.DB
	stmts  struct {
		getApp *sql.Stmt
		getExp *sql.Stmt
		getLyr *sql.Stmt
		getSeg *sql.Stmt
		getGrp *sql.Stmt
	}
}

func (s *mysqlSource) Close() {
	// just close client, stmt will be released automatically
	s.client.Close()
}

// CreateMySQLSource opens a MySQL-backed Source using the given DSN.
func CreateMySQLSource(config string) (Source, error) {
	client, err := sql.Open("mysql", config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	s := &mysqlSource{client: client}

	s.stmts.getApp, err = s.client.Prepare(
		"SELECT `app_id`,`access_token` FROM `application`")
	if err != nil {
		return nil, err
	}

	s.stmts.getExp, err = s.client.Prepare(
		"SELECT `exp_id`,`app_id`,`seed`,`filter` FROM `experiment` " +
			"WHERE `status` = 1 ORDER BY `exp_id` ASC")
	if err != nil {
		return nil, err
	}

	s.stmts.getLyr, err = s.client.Prepare("SELECT t2.* FROM " +
		"( SELECT `exp_id` FROM `experiment` WHERE `status`=1 ) t1 " +
		"INNER JOIN " +
		"( SELECT `lyr_id`,`exp_id`,`name` FROM `exp_layer` ) t2 " +
		"ON t1.exp_id = t2.exp_id ORDER BY t2.`lyr_id` ASC")
	if err != nil {
		return nil, err
	}

	s.stmts.getSeg, err = s.client.Prepare("SELECT t3.* FROM " +
		"( SELECT `exp_id` FROM `experiment` WHERE `status`=1 ) t1 " +
		"INNER JOIN " +
		"( SELECT `lyr_id`,`exp_id` FROM `exp_layer` ) t2 " +
		"ON t1.exp_id = t2.exp_id " +
		"INNER JOIN " +
		"( SELECT `seg_id`,`lyr_id`,`range_begin`,`range_end`,`seed` FROM `exp_segment` ) t3 " +
		"ON t2.lyr_id = t3.lyr_id ORDER BY t3.`seg_id` ASC")
	if err != nil {
		return nil, err
	}

	s.stmts.getGrp, err = s.client.Prepare(
		"SELECT `grp_id`,t3.seg_id,`name`,`bitmap`,`force_hit`," +
			"COALESCE(`content`,'') AS `content` FROM " +
			"( SELECT `exp_id` FROM `experiment` WHERE `status`=1 ) t1 " +
			"INNER JOIN " +
			"( SELECT `lyr_id`,`exp_id` FROM `exp_layer` ) t2 " +
			"ON t1.exp_id = t2.exp_id " +
			"INNER JOIN " +
			"( SELECT `seg_id`,`lyr_id` FROM `exp_segment` ) t3 " +
			"ON t2.lyr_id = t3.lyr_id " +
			"INNER JOIN " +
			"( SELECT `grp_id`,`seg_id`,`name`,`bitmap`,`force_hit`,`cfg_id` FROM `exp_group` ) t4 " +
			"ON t3.seg_id = t4.seg_id " +
			"LEFT JOIN " +
			"( SELECT `cfg_id`,`content` FROM `exp_config` ) t5 " +
			"ON t4.cfg_id = t5.cfg_id ORDER BY t4.`grp_id` ASC")
	if err != nil {
		return nil, err
	}

	return s, nil
}

type group struct {
	name     string
	bitmap   []byte
	config   string
	forceHit []string
}

type segment struct {
	begin  uint32
	end    uint32
	seed   uint32
	groups []uint32
}

type layer struct {
	name     string
	segments []uint32
}

type experiment struct {
	filter []byte
	seed   uint32
	layers []uint32
}

type application struct {
	token       string
	experiments []uint32
}

var (
	errBrokenData      = errors.New("broken data")
	errConsistencyLost = errors.New("consistency lost")
)

func (s *mysqlSource) getApplications(tx *sql.Tx, apps map[uint32]*application) error {
	rows, err := tx.Stmt(s.stmts.getApp).Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var appId uint32
		app := &application{}
		err = rows.Scan(&appId, &app.token)
		if err != nil {
			return err
		}
		apps[appId] = app
	}
	return rows.Err()
}

func (s *mysqlSource) getExperiment(tx *sql.Tx, apps map[uint32]*application, exps map[uint32]*experiment) error {
	rows, err := tx.Stmt(s.stmts.getExp).Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var expId, appId uint32
		exp := &experiment{}
		err = rows.Scan(&expId, &appId, &exp.seed, &exp.filter)
		if err != nil {
			return err
		}
		app := apps[appId]
		if app == nil {
			return errConsistencyLost
		}
		app.experiments = append(app.experiments, expId)
		exps[expId] = exp
	}
	return rows.Err()
}

func (s *mysqlSource) getLayer(tx *sql.Tx, exps map[uint32]*experiment, lyrs map[uint32]*layer) error {
	rows, err := tx.Stmt(s.stmts.getLyr).Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var lyrId, expId uint32
		lyr := &layer{}
		err = rows.Scan(&lyrId, &expId, &lyr.name)
		if err != nil {
			return err
		}
		exp := exps[expId]
		if exp == nil {
			return errConsistencyLost
		}
		exp.layers = append(exp.layers, lyrId)
		lyrs[lyrId] = lyr
	}
	return rows.Err()
}

func (s *mysqlSource) getSegment(tx *sql.Tx, lyrs map[uint32]*layer, segs map[uint32]*segment) error {
	rows, err := tx.Stmt(s.stmts.getSeg).Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var segId, lyrId uint32
		seg := &segment{}
		err = rows.Scan(&segId, &lyrId, &seg.begin, &seg.end, &seg.seed)
		if err != nil {
			return err
		}
		lyr := lyrs[lyrId]
		if lyr == nil {
			return errConsistencyLost
		}
		lyr.segments = append(lyr.segments, segId)
		segs[segId] = seg
	}
	return rows.Err()
}

func (s *mysqlSource) getGroup(tx *sql.Tx, segs map[uint32]*segment, grps map[uint32]*group) error {
	rows, err := tx.Stmt(s.stmts.getGrp).Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var grpId, segId uint32
		var forceHit string
		grp := &group{}
		err = rows.Scan(&grpId, &segId, &grp.name, &grp.bitmap, &forceHit, &grp.config)
		if err != nil {
			return err
		} else if len(grp.bitmap) != 125 {
			return errBrokenData
		}
		seg := segs[segId]
		if seg == nil {
			return errConsistencyLost
		}
		seg.groups = append(seg.groups, grpId)
		if len(forceHit) != 0 {
			grp.forceHit = strings.Split(forceHit, ",")
		}
		grps[grpId] = grp
	}
	return rows.Err()
}

func (s *mysqlSource) Fetch(ctx context.Context) (map[uint32]Application, error) {
	apps := make(map[uint32]*application)
	exps := make(map[uint32]*experiment)
	lyrs := make(map[uint32]*layer)
	segs := make(map[uint32]*segment)
	grps := make(map[uint32]*group)

	tx, err := s.client.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.getApplications(tx, apps); err != nil {
		return nil, fmt.Errorf("fail to run sql[getApp]: %v", err)
	}
	if err := s.getExperiment(tx, apps, exps); err != nil {
		return nil, fmt.Errorf("fail to run sql[getExp]: %v", err)
	}
	if err := s.getLayer(tx, exps, lyrs); err != nil {
		return nil, fmt.Errorf("fail to run sql[getLyr]: %v", err)
	}
	if err := s.getSegment(tx, lyrs, segs); err != nil {
		return nil, fmt.Errorf("fail to run sql[getSeg]: %v", err)
	}
	if err := s.getGroup(tx, segs, grps); err != nil {
		return nil, fmt.Errorf("fail to run sql[getGrp]: %v", err)
	}

	out := make(map[uint32]Application, len(apps))

	for appId, appX := range apps {
		app := Application{
			AccessToken: appX.token,
			Experiments: make([]core.Experiment, 0, len(appX.experiments)),
		}
		for _, expId := range appX.experiments {
			expX := exps[expId]
			if expX == nil {
				return nil, errConsistencyLost
			}
			exp := core.Experiment{
				Seed:   expX.seed,
				Layers: make([]core.Layer, 0, len(expX.layers)),
			}
			exp.Filter, err = core.ParseExpr(expX.filter)
			if err != nil {
				return nil, fmt.Errorf("fail to parse filter of experiment %d: %v", expId, err)
			}
			for _, lyrId := range expX.layers {
				lyrX := lyrs[lyrId]
				if lyrX == nil {
					return nil, errConsistencyLost
				}
				lyr := core.Layer{
					Name:     lyrX.name,
					Segments: make([]core.Segment, 0, len(lyrX.segments)),
				}
				lyr.ForceHit = make(map[string]core.HitIndex)
				for _, segId := range lyrX.segments {
					segX := segs[segId]
					if segX == nil {
						return nil, errConsistencyLost
					}
					seg := core.Segment{
						Seed:   segX.seed,
						Groups: make([]core.Group, 0, len(segX.groups)),
					}
					seg.Range.Begin = segX.begin
					seg.Range.End = segX.end
					for _, grpId := range segX.groups {
						grpX := grps[grpId]
						if grpX == nil {
							return nil, errConsistencyLost
						}
						for _, key := range grpX.forceHit {
							lyr.ForceHit[key] = core.HitIndex{
								Seg: uint32(len(lyr.Segments)),
								Grp: uint32(len(seg.Groups)),
							}
						}
						seg.Groups = append(seg.Groups, core.Group{
							Name:   grpX.name,
							Bitmap: grpX.bitmap,
							Config: grpX.config,
						})
					}
					lyr.Segments = append(lyr.Segments, seg)
				}
				exp.Layers = append(exp.Layers, lyr)
			}
			app.Experiments = append(app.Experiments, exp)
		}
		out[appId] = app
	}

	return out, nil
}
