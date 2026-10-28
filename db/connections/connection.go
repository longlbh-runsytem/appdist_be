package connections

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/utils"
)

// NewPool mở connection pool tới Postgres và ping để chắc chắn kết nối được.
//
// Trả về *pgxpool.Pool chứ không phải *sqlc.Queries: db.NewStore cần pool để
// mở transaction, mà sqlc.Queries thì không cho lấy lại pool.
func NewPool(cfg utils.Config) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig(cfg.DBSource)
	if err != nil {
		return nil, fmt.Errorf("cannot parse DB config: %w", err)
	}

	conf.MaxConns = 50
	conf.MinConns = 5
	conf.MaxConnLifetime = 30 * time.Minute
	conf.MaxConnIdleTime = 5 * time.Minute
	conf.HealthCheckPeriod = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("cannot create DB pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot ping DB: %w", err)
	}

	log.Info().Msg("DB connected successfully")
	return pool, nil
}
