package infra

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scthy/live-stream-danmaku/internal/logger"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var migrations embed.FS

func InitDB() *pgxpool.Pool {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://danmaku:danmaku@127.0.0.1:5432/danmaku_db?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Log.Fatal("Invalid PostgreSQL configuration", zap.Error(err))
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		logger.Log.Fatal("PostgreSQL migration failed", zap.Error(err))
	}
	return pool
}

// Version files are applied in lexical order under the same lock as the old AutoMigrate.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1684106861, 1)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS danmaku_schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
			return err
		}
		files, err := migrations.ReadDir("migrations")
		if err != nil {
			return err
		}
		for _, file := range files {
			var applied bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM danmaku_schema_migrations WHERE version=$1)", file.Name()).Scan(&applied); err != nil {
				return err
			}
			if applied {
				continue
			}
			script, err := migrations.ReadFile("migrations/" + file.Name())
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, string(script)); err != nil {
				return fmt.Errorf("%s: %w", file.Name(), err)
			}
			if _, err := tx.Exec(ctx, "INSERT INTO danmaku_schema_migrations(version) VALUES ($1)", file.Name()); err != nil {
				return err
			}
		}
		return nil
	})
}
