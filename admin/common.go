package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
)

type Context struct {
	context.Context
	*utils.ContextLogger
}

func NewContext(ctx context.Context, tag string) *Context {
	return &Context{
		Context:       ctx,
		ContextLogger: utils.NewContextLogger(tag),
	}
}

func parseUintParam(w http.ResponseWriter, p httprouter.Params, key string) (uint32, bool) {
	id, err := strconv.ParseUint(p.ByName(key), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	return uint32(id), true
}

func getJsonArgs(ctx *Context, w http.ResponseWriter, r *http.Request, req any) bool {
	if err := utils.HttpGetJsonArgs(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func getJsonArgsWithLog(ctx *Context, w http.ResponseWriter, r *http.Request, req any) bool {
	if err := utils.HttpGetJsonArgsWithLog(ctx.ContextLogger, r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func withTx(
	ctx *Context, w http.ResponseWriter,
	opts *sql.TxOptions, fn func(*Context, *sql.Tx) int,
) bool {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		ctx.Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	defer tx.Rollback()

	code := fn(ctx, tx)
	if code != http.StatusOK {
		w.WriteHeader(code)
		return false
	}

	if err = tx.Commit(); err != nil {
		ctx.Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	return true
}

func touch(ctx *Context, stmt *sql.Stmt, id, version uint32, hint string) int {
	n, err := utils.SqlModify(ctx, stmt, version+1, id, version)
	if err != nil {
		ctx.Errorf("fail to run sql[%s.touch]: %v", hint, err)
		return http.StatusInternalServerError
	}
	if n == 0 {
		ctx.Warnf("operation conflict: %d", id)
		return http.StatusConflict
	}
	return http.StatusOK
}

func queryRows(
	ctx *Context, hint string,
	query func() (*sql.Rows, error), scan func(*sql.Rows) error,
) int {
	rows, err := query()
	if err != nil {
		ctx.Errorf("fail to run sql[%s]: %v", hint, err)
		return http.StatusInternalServerError
	}
	defer rows.Close()

	for rows.Next() {
		if err = scan(rows); err != nil {
			ctx.Errorf("fail to run sql[%s]: %v", hint, err)
			return http.StatusInternalServerError
		}
	}
	if err = rows.Err(); err != nil {
		ctx.Errorf("fail to iterate sql[%s]: %v", hint, err)
		return http.StatusInternalServerError
	}
	return http.StatusOK
}
