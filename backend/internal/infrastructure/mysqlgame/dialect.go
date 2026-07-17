package mysqlgame

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type SQLDialect string

const (
	DialectMySQL  SQLDialect = "mysql"
	DialectSQLite SQLDialect = "sqlite"
)

func normalizeDialect(dialect SQLDialect) SQLDialect {
	if dialect == DialectSQLite {
		return DialectSQLite
	}
	return DialectMySQL
}

func detectSQLDialect(db *sql.DB) SQLDialect {
	if db != nil && strings.Contains(strings.ToLower(fmt.Sprintf("%T", db.Driver())), "sqlite") {
		return DialectSQLite
	}
	return DialectMySQL
}

func detectSQLDialectFromQueryer(queryer Queryer) SQLDialect {
	return detectSQLDialectFromRunner(queryer)
}

func detectSQLDialectFromRunner(runner any) SQLDialect {
	switch value := runner.(type) {
	case SQLQueryer:
		return detectSQLDialect(value.DB)
	case *SQLQueryer:
		return detectSQLDialect(value.DB)
	case sqlTransactionRunner:
		return value.dialect
	case *sqlTransactionRunner:
		return value.dialect
	}
	return DialectMySQL
}

func rewriteSQLiteLimitedMutation(query string) string {
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)
	limitAt := strings.LastIndex(upper, " LIMIT ")
	if limitAt < 0 {
		return query
	}
	limit := strings.TrimSpace(trimmed[limitAt+len(" LIMIT "):])
	if _, err := strconv.Atoi(limit); err != nil {
		return query
	}
	body := trimmed[:limitAt]
	upperBody := upper[:limitAt]
	if strings.HasPrefix(upperBody, "UPDATE ") {
		setAt := strings.Index(upperBody, " SET ")
		if setAt < 0 {
			return query
		}
		whereRelative := strings.Index(upperBody[setAt+len(" SET "):], " WHERE ")
		if whereRelative < 0 {
			return query
		}
		whereAt := setAt + len(" SET ") + whereRelative
		table := strings.TrimSpace(body[len("UPDATE "):setAt])
		if table == "" || strings.ContainsAny(table, " \t\r\n") {
			return query
		}
		setClause := strings.TrimSpace(body[setAt+len(" SET ") : whereAt])
		whereClause := strings.TrimSpace(body[whereAt+len(" WHERE "):])
		return "UPDATE " + table + " SET " + setClause + " WHERE rowid IN (SELECT rowid FROM " + table + " WHERE " + whereClause + " LIMIT " + limit + ")"
	}
	if strings.HasPrefix(upperBody, "DELETE FROM ") {
		tail := body[len("DELETE FROM "):]
		upperTail := upperBody[len("DELETE FROM "):]
		clauseAt := len(tail)
		for _, marker := range []string{" WHERE ", " ORDER BY "} {
			if index := strings.Index(upperTail, marker); index >= 0 && index < clauseAt {
				clauseAt = index
			}
		}
		table := strings.TrimSpace(tail[:clauseAt])
		if table == "" || strings.ContainsAny(table, " \t\r\n") {
			return query
		}
		clauses := strings.TrimSpace(tail[clauseAt:])
		if clauses != "" {
			clauses = " " + clauses
		}
		return "DELETE FROM " + table + " WHERE rowid IN (SELECT rowid FROM " + table + clauses + " LIMIT " + limit + ")"
	}
	return query
}
