package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/poshagator/content-service/config"
)

func NewPool(lc fx.Lifecycle, l *zap.Logger, cfg *config.ConfigModel) (*pgxpool.Pool, error) {
	log := l.Named("repo.pg")
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			const tries = 5
			var pingErr error
			for i := 0; i < tries; i++ {
				pingErr = pool.Ping(ctx)
				if pingErr == nil {
					log.Info("PostgreSQL connected", zap.Int("try", i+1))
					return nil
				}
				log.Warn("PostgreSQL connection failed, retrying...", zap.Error(pingErr), zap.Int("try", i+1))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}
			return fmt.Errorf("postgres: connection retries exceeded: %w", pingErr)
		},
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
