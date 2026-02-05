package utils

import (
	"context"
	"database/sql"
	"strings"

	"github.com/go-sql-driver/mysql"
)

func IsMysqlDuplicateError(err error) bool {
	if obj, ok := err.(*mysql.MySQLError); ok {
		return obj.Number == 1062
	}
	return false
}

func SqlCreate(stmt *sql.Stmt, args ...any) (int64, error) {
	result, err := stmt.Exec(args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func SqlModify(stmt *sql.Stmt, args ...any) (int64, error) {
	result, err := stmt.Exec(args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func SqlModifyContext(ctx context.Context, stmt *sql.Stmt, args ...any) (int64, error) {
	result, err := stmt.ExecContext(ctx, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func OverwriteMysqlParams(config string, patch map[string]string) string {
	if len(patch) == 0 {
		return config
	}

	head := config
	params := make(map[string]string)
	idx := strings.IndexByte(config, '?')
	if idx >= 0 {
		head = config[:idx]
		tail := config[idx+1:]
		if len(tail) != 0 {
			for _, param := range strings.Split(tail, "&") {
				idx = strings.IndexByte(param, '=')
				if idx <= 0 {
					continue
				}
				params[param[:idx]] = param[idx+1:]
			}
		}
	}

	for k, v := range patch {
		params[k] = v
	}

	var sb strings.Builder
	sb.WriteString(head)
	sep := byte('?')
	for k, v := range params {
		sb.WriteByte(sep)
		sep = byte('&')
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(v)
	}
	return sb.String()
}
