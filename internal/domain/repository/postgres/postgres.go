package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
)

type PoolManager struct {
	ctx context.Context
	log *zap.Logger
	cfg *config.ConfigModel
	db  *pgxpool.Pool
}

func NewPoolManager(l *zap.Logger, cfg *config.ConfigModel, ctx context.Context) (*PoolManager, error) {
	return &PoolManager{ctx: ctx, log: l.Named("repo.pg"), cfg: cfg}, nil
}

func (r *PoolManager) OnStart(_ context.Context) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		r.cfg.Postgres.Host,
		r.cfg.Postgres.Port,
		r.cfg.Postgres.User,
		r.cfg.Postgres.Password,
		r.cfg.Postgres.DBName,
		r.cfg.Postgres.SSLMode,
	)

	const tries = 5
	for i := 0; i < tries; i++ {
		if db, err := pgxpool.New(r.ctx, dsn); err == nil {
			r.db = db
			r.log.Info("PostgreSQL connected", zap.Int("try", i+1))
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("postgres: connection retries exceeded")
}

func (r *PoolManager) OnStop(_ context.Context) error {
	if r.db != nil {
		r.db.Close()
	}
	return nil
}

func (r *PoolManager) GetPool() *pgxpool.Pool {
	return r.db
}
