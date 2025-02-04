package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(db *sql.DB) *Database {
	return &Database{db: db}
}

func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, opts)
}

func (d *Database) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

func (d *Database) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

func (d *Database) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

func (d *Database) DeleteOldAuditLogs(ctx context.Context, cutoffDate time.Time) error {
	query := `DELETE FROM audit_log WHERE changed_at < $1`
	_, err := d.db.ExecContext(ctx, query, cutoffDate)
	return err
}

func (d *Database) VacuumAnalyze(ctx context.Context, table string) error {
	query := fmt.Sprintf("VACUUM ANALYZE %s", table)
	_, err := d.db.ExecContext(ctx, query)
	return err
}

func (db *Database) GetDB() *sql.DB {
	return db.db
}
