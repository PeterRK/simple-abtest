package main

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/engine/sign"
	"github.com/peterrk/simple-abtest/utils"
)

const (
	maxMetricNameLen             = 128
	maxBucketTypeLen             = 16
	maxBucketKeyLen              = 64
	maxResultWriteRowsPerRequest = 10000
	resultWriteBatchLevel        = 8
	bucketTypeHour               = "hour"
	bucketTypeDay                = "day"
	bucketTypeCustom             = "custom"
	accessTokenHeaderName        = "ACCESS_TOKEN"
)

var (
	validResultKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

	resultSql struct {
		upsert     []resultUpsertStmt
		getOptions *sql.Stmt
		getData    *sql.Stmt
	}
)

func prepareResultSql(db *sql.DB) (err error) {
	buildUpsertSql := func(rows int) string {
		const valueClause = "(?,?,?,?,?,?,?,?,?)"
		var builder strings.Builder
		builder.WriteString("INSERT INTO `exp_result`(")
		builder.WriteString("`app_id`,`exp_id`,`layer_name`,`group_name`,`metric_name`,")
		builder.WriteString("`bucket_type`,`bucket_key`,`bucket_stamp`,`metric_value`) VALUES ")
		for i := 0; i < rows; i++ {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(valueClause)
		}
		builder.WriteString(" ON DUPLICATE KEY UPDATE ")
		builder.WriteString("`bucket_stamp`=VALUES(`bucket_stamp`),")
		builder.WriteString("`metric_value`=VALUES(`metric_value`),")
		builder.WriteString("`update_time`=CURRENT_TIMESTAMP")
		return builder.String()
	}

	resultSql.upsert = make([]resultUpsertStmt, 0, resultWriteBatchLevel+1)
	for level := 0; level <= resultWriteBatchLevel; level++ {
		size := 1 << level
		stmt, err := db.Prepare(buildUpsertSql(size))
		if err != nil {
			return err
		}
		resultSql.upsert = append(resultSql.upsert, resultUpsertStmt{size: size, stmt: stmt})
	}
	resultSql.getOptions, err = db.Prepare(
		"SELECT DISTINCT `layer_name`,`bucket_type`,`metric_name` " +
			"FROM `exp_result` WHERE `app_id`=? AND `exp_id`=? " +
			"ORDER BY `layer_name` ASC,`bucket_type` ASC,`metric_name` ASC")
	if err != nil {
		return err
	}
	resultSql.getData, err = db.Prepare(
		"SELECT `bucket_key`,`bucket_stamp`,`group_name`,`metric_value` " +
			"FROM `exp_result` WHERE `app_id`=? AND `exp_id`=? " +
			"AND `layer_name`=? AND `bucket_type`=? AND `metric_name`=? " +
			"AND `bucket_stamp`>=? AND `bucket_stamp`<? " +
			"ORDER BY `bucket_stamp` ASC,`bucket_key` ASC,`group_name` ASC")
	if err != nil {
		return err
	}
	return nil
}

func bindResultOp(router *httprouter.Router) {
	router.Handle(http.MethodPost, "/api/app/:id/exp/:eid/result", resultWrite)
	router.Handle(http.MethodGet, "/api/app/:id/exp/:eid/result/options", resultGetOptions)
	router.Handle(http.MethodGet, "/api/app/:id/exp/:eid/result/data", resultGetData)
}

type resultWritePoint struct {
	GroupName    string    `json:"group_name"`
	BucketKey    string    `json:"bucket_key"`
	BucketStamp  int64     `json:"bucket_stamp"`
	MetricValues []float64 `json:"metric_value"`
}

type resultWriteRequest struct {
	LayerName  string             `json:"layer_name"`
	BucketType string             `json:"bucket_type"`
	MetricName []string           `json:"metric_name"`
	Points     []resultWritePoint `json:"points"`
}

type resultWriteRow struct {
	GroupName   string
	BucketKey   string
	BucketStamp int64
	MetricName  string
	MetricValue float64
}

type resultUpsertStmt struct {
	size int
	stmt *sql.Stmt
}

type resultWriteBuffer struct {
	ctx   *Context
	tx    *sql.Tx
	appId uint32
	expId uint32
	req   *resultWriteRequest
	rows  []resultWriteRow
}

type resultOptionBucket struct {
	Name    string   `json:"name"`
	Metrics []string `json:"metrics"`
}

type resultOptionLayer struct {
	Name        string               `json:"name"`
	BucketTypes []resultOptionBucket `json:"bucket_types"`
}

type resultDataPoint struct {
	BucketKey   string  `json:"bucket_key"`
	BucketStamp int64   `json:"bucket_stamp"`
	GroupName   string  `json:"group_name"`
	MetricValue float64 `json:"metric_value"`
}

func resultWrite(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "resultWrite")
	appId, expId, ok := parseResultPath(w, p)
	if !ok {
		return
	}
	if !verifyResultWriteToken(ctx, w, r, appId) {
		return
	}

	req := &resultWriteRequest{}
	if !getJsonArgs(ctx, w, r, req) {
		return
	}
	if !validResultWriteRequest(req) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !withTx(ctx, w, &sql.TxOptions{}, func(ctx *Context, tx *sql.Tx) int {
		buf := newResultWriteBuffer(ctx, tx, appId, expId, req)
		for i := range req.Points {
			point := &req.Points[i]
			for j, metricName := range req.MetricName {
				if code := buf.append(resultWriteRow{
					GroupName:   point.GroupName,
					BucketKey:   point.BucketKey,
					BucketStamp: point.BucketStamp,
					MetricName:  metricName,
					MetricValue: point.MetricValues[j],
				}); code != http.StatusOK {
					return code
				}
			}
		}
		return buf.flush()
	}) {
		return
	}

	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &struct {
		PointCount  int `json:"point_count"`
		MetricCount int `json:"metric_count"`
		RowCount    int `json:"row_count"`
	}{
		PointCount:  len(req.Points),
		MetricCount: len(req.MetricName),
		RowCount:    len(req.Points) * len(req.MetricName),
	})
}

func resultGetOptions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "resultGetOptions")
	appId, expId, ok := parseResultPath(w, p)
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, appId, privilegeReadOnly); !ok {
		return
	}

	type bucketMark struct {
		idx int
	}
	type layerMark struct {
		idx     int
		buckets map[string]bucketMark
	}
	resp := &struct {
		Layers []resultOptionLayer `json:"layers"`
	}{
		Layers: make([]resultOptionLayer, 0),
	}
	layers := make(map[string]layerMark)

	code := queryRows(ctx, "result.getOptions",
		func() (*sql.Rows, error) { return resultSql.getOptions.QueryContext(ctx, appId, expId) },
		func(rows *sql.Rows) error {
			var layerName, bucketType, metricName string
			if err := rows.Scan(&layerName, &bucketType, &metricName); err != nil {
				return err
			}
			lm, ok := layers[layerName]
			if !ok {
				lm = layerMark{idx: len(resp.Layers), buckets: make(map[string]bucketMark)}
				layers[layerName] = lm
				resp.Layers = append(resp.Layers, resultOptionLayer{Name: layerName})
			}
			bm, ok := lm.buckets[bucketType]
			if !ok {
				bm = bucketMark{idx: len(resp.Layers[lm.idx].BucketTypes)}
				lm.buckets[bucketType] = bm
				resp.Layers[lm.idx].BucketTypes = append(resp.Layers[lm.idx].BucketTypes,
					resultOptionBucket{Name: bucketType})
			}
			resp.Layers[lm.idx].BucketTypes[bm.idx].Metrics = append(
				resp.Layers[lm.idx].BucketTypes[bm.idx].Metrics, metricName)
			return nil
		})
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, resp)
}

func resultGetData(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	ctx := NewContext(r.Context(), "resultGetData")
	appId, expId, ok := parseResultPath(w, p)
	if !ok {
		return
	}
	if _, ok := requireAppPrivilege(ctx, w, r, appId, privilegeReadOnly); !ok {
		return
	}

	query := r.URL.Query()
	layerName := query.Get("layer_name")
	bucketType := query.Get("bucket_type")
	metricName := query.Get("metric_name")
	begin, ok := parseInt64Query(w, query.Get("begin_stamp"))
	if !ok {
		return
	}
	end, ok := parseInt64Query(w, query.Get("end_stamp"))
	if !ok {
		return
	}
	if begin >= end ||
		!validName(layerName, maxLayerNameLen) ||
		!validBucketType(bucketType) ||
		!validResultKey(metricName, maxMetricNameLen) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp := make([]resultDataPoint, 0)
	code := queryRows(ctx, "result.getData",
		func() (*sql.Rows, error) {
			return resultSql.getData.QueryContext(ctx,
				appId, expId, layerName, bucketType, metricName, begin, end)
		},
		func(rows *sql.Rows) error {
			var point resultDataPoint
			if err := rows.Scan(&point.BucketKey, &point.BucketStamp,
				&point.GroupName, &point.MetricValue); err != nil {
				return err
			}
			resp = append(resp, point)
			return nil
		})
	if code != http.StatusOK {
		w.WriteHeader(code)
		return
	}
	utils.HttpReplyJsonWithLog(ctx.ContextLogger, w, http.StatusOK, &resp)
}

func verifyResultWriteToken(ctx *Context, w http.ResponseWriter, r *http.Request, appId uint32) bool {
	raw := r.Header.Get(accessTokenHeaderName)
	if len(raw) == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	var signingSecret string
	err := appSql.getToken.QueryRowContext(ctx, appId).Scan(&signingSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
		} else {
			ctx.Errorf("fail to run sql[app.getToken]: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return false
	}
	capability, ok := sign.VerifyPublicTokenV2(signingSecret, appId, raw)
	if !ok || capability&^appTokenCapabilityKnownMask != 0 ||
		capability&appTokenCapabilityResultWrite == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

func parseResultPath(w http.ResponseWriter, p httprouter.Params) (uint32, uint32, bool) {
	appId, ok := parseUintParam(w, p, "id")
	if !ok {
		return 0, 0, false
	}
	expId, ok := parseUintParam(w, p, "eid")
	if !ok {
		return 0, 0, false
	}
	if appId == 0 || expId == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return 0, 0, false
	}
	return appId, expId, true
}

func parseInt64Query(w http.ResponseWriter, raw string) (int64, bool) {
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	return val, true
}

func resultWriteBatchStmt(remaining int) (int, *sql.Stmt) {
	for i := len(resultSql.upsert) - 1; i >= 0; i-- {
		upsert := &resultSql.upsert[i]
		if remaining >= upsert.size {
			return upsert.size, upsert.stmt
		}
	}
	return 0, nil
}

func newResultWriteBuffer(
	ctx *Context, tx *sql.Tx, appId, expId uint32, req *resultWriteRequest,
) *resultWriteBuffer {
	return &resultWriteBuffer{
		ctx:   ctx,
		tx:    tx,
		appId: appId,
		expId: expId,
		req:   req,
		rows:  make([]resultWriteRow, 0, 1<<resultWriteBatchLevel),
	}
}

func (buf *resultWriteBuffer) append(row resultWriteRow) int {
	buf.rows = append(buf.rows, row)
	if len(buf.rows) < cap(buf.rows) {
		return http.StatusOK
	}
	return buf.flush()
}

func (buf *resultWriteBuffer) flush() int {
	for len(buf.rows) > 0 {
		size, rawStmt := resultWriteBatchStmt(len(buf.rows))
		if rawStmt == nil {
			return http.StatusInternalServerError
		}
		stmt := buf.tx.Stmt(rawStmt)
		args := make([]any, 0, size*9)
		for i := 0; i < size; i++ {
			row := &buf.rows[i]
			args = append(args,
				buf.appId,
				buf.expId,
				buf.req.LayerName,
				row.GroupName,
				row.MetricName,
				buf.req.BucketType,
				row.BucketKey,
				row.BucketStamp,
				row.MetricValue,
			)
		}
		if _, err := stmt.ExecContext(buf.ctx, args...); err != nil {
			buf.ctx.Errorf("fail to run sql[result.upsert.%d]: %v", size, err)
			return http.StatusInternalServerError
		}
		copy(buf.rows, buf.rows[size:])
		buf.rows = buf.rows[:len(buf.rows)-size]
	}
	return http.StatusOK
}

func validResultWriteRequest(req *resultWriteRequest) bool {
	if !validName(req.LayerName, maxLayerNameLen) ||
		!validBucketType(req.BucketType) ||
		!validResultMetricNames(req.MetricName) ||
		len(req.Points) == 0 ||
		len(req.Points)*len(req.MetricName) > maxResultWriteRowsPerRequest {
		return false
	}
	for i := range req.Points {
		point := &req.Points[i]
		if !validName(point.GroupName, maxGroupNameLen) ||
			!validResultKey(point.BucketKey, maxBucketKeyLen) ||
			len(point.MetricValues) != len(req.MetricName) {
			return false
		}
	}
	return true
}

func validResultMetricNames(metricNames []string) bool {
	if len(metricNames) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(metricNames))
	for _, metricName := range metricNames {
		if !validResultKey(metricName, maxMetricNameLen) {
			return false
		}
		if _, ok := seen[metricName]; ok {
			return false
		}
		seen[metricName] = struct{}{}
	}
	return true
}

func validBucketType(bucketType string) bool {
	if len(bucketType) == 0 || len(bucketType) > maxBucketTypeLen {
		return false
	}
	switch bucketType {
	case bucketTypeHour, bucketTypeDay, bucketTypeCustom:
		return true
	default:
		return false
	}
}

func validResultKey(name string, maxLen int) bool {
	return len(name) != 0 && len(name) <= maxLen && validResultKeyPattern.MatchString(name)
}
