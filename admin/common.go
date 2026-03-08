package main

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/peterrk/simple-abtest/utils"
)

func parseUintParam(w http.ResponseWriter, p httprouter.Params, key string) (uint32, bool) {
	id, err := strconv.ParseUint(p.ByName(key), 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return 0, false
	}
	return uint32(id), true
}

func getJsonArgs(logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request, req any) bool {
	if err := utils.HttpGetJsonArgsWithLog(logger, r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return false
	}
	return true
}

func withTx(
	logger *utils.ContextLogger, w http.ResponseWriter, opts *sql.TxOptions,
	fn func(tx *sql.Tx) int,
) bool {
	tx, err := db.BeginTx(context.Background(), opts)
	if err != nil {
		logger.Errorf("fail to start transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	defer tx.Rollback()

	code := fn(tx)
	if code != http.StatusOK {
		w.WriteHeader(code)
		return false
	}

	if err = tx.Commit(); err != nil {
		logger.Errorf("fail to commit transaction: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	return true
}

func touch(logger *utils.ContextLogger, stmt *sql.Stmt, id, version uint32, hint string) int {
	n, err := utils.SqlModify(stmt, version+1, id, version)
	if err != nil {
		logger.Errorf("fail to run sql[%s.touch]: %v", hint, err)
		return http.StatusInternalServerError
	}
	if n == 0 {
		logger.Warnf("operation conflict: %d", id)
		return http.StatusConflict
	}
	return http.StatusOK
}
