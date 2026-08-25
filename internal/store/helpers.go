package store

import (
	"database/sql"
	"time"
)

func timeText(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return timeText(*t)
}
func parseTime(v string) time.Time { t, _ := time.Parse(time.RFC3339Nano, v); return t }
func parseOptional(v sql.NullString) *time.Time {
	if !v.Valid {
		return nil
	}
	t := parseTime(v.String)
	return &t
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}
