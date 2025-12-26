package instruments

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInstrumentNotFound = errors.New("instrument not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	if r == nil || r.pool == nil {
		return
	}
	r.pool.Close()
}

func (r *Repository) CreateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	return r.createInstrumentWith(ctx, r.pool, instrument)
}

func (r *Repository) GetInstrument(ctx context.Context, uid uuid.UUID) (*domain.Instrument, error) {
	const query = `
		SELECT uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at
		FROM instruments
		WHERE uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	instrument := &domain.Instrument{}
	if err := scanInstrumentInto(row, instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return instrument, nil
}

func (r *Repository) UpdateInstrument(ctx context.Context, instrument *domain.Instrument) error {
	return r.updateInstrumentWith(ctx, r.pool, instrument)
}

func (r *Repository) DeleteInstrument(ctx context.Context, uid uuid.UUID) error {
	return r.deleteInstrumentWith(ctx, r.pool, uid)
}

func scanInstrumentInto(row pgx.Row, instrument *domain.Instrument, extras ...interface{}) error {
	var deletedAt *time.Time
	args := []interface{}{
		&instrument.UID,
		&instrument.Figi,
		&instrument.Ticker,
		&instrument.Lot,
		&instrument.ClassCode,
		&instrument.LogoURL,
		&instrument.CreatedAt,
		&instrument.UpdatedAt,
		&deletedAt,
	}
	args = append(args, extras...)

	if err := row.Scan(args...); err != nil {
		return err
	}
	instrument.DeletedAt = deletedAt
	return nil
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type commandTagExecutor interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

func (r *Repository) withTx(ctx context.Context, fn func(pgx.Tx) error) (err error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) createInstrumentWith(ctx context.Context, runner queryRower, instrument *domain.Instrument) error {
	if instrument == nil {
		return errors.New("instrument is nil")
	}
	if instrument.UID == uuid.Nil {
		instrument.UID = uuid.New()
	}
	now := time.Now().UTC()
	if instrument.CreatedAt.IsZero() {
		instrument.CreatedAt = now
	}
	instrument.UpdatedAt = now

	const query = `
		INSERT INTO instruments (uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at`

	row := runner.QueryRow(ctx, query,
		instrument.UID,
		instrument.Figi,
		instrument.Ticker,
		instrument.Lot,
		instrument.ClassCode,
		instrument.LogoURL,
		instrument.CreatedAt,
		instrument.UpdatedAt,
		instrument.DeletedAt,
	)

	return scanInstrumentInto(row, instrument)
}

func (r *Repository) updateInstrumentWith(ctx context.Context, runner queryRower, instrument *domain.Instrument) error {
	if instrument == nil {
		return errors.New("instrument is nil")
	}
	if instrument.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	instrument.UpdatedAt = time.Now().UTC()

	const query = `
		UPDATE instruments
		SET figi=$2,
			ticker=$3,
			lot=$4,
			class_code=$5,
			logo_url=$6,
			updated_at=$7,
			deleted_at=$8
		WHERE uid=$1
		RETURNING uid, figi, ticker, lot, class_code, logo_url, created_at, updated_at, deleted_at`

	row := runner.QueryRow(ctx, query,
		instrument.UID,
		instrument.Figi,
		instrument.Ticker,
		instrument.Lot,
		instrument.ClassCode,
		instrument.LogoURL,
		instrument.UpdatedAt,
		instrument.DeletedAt,
	)

	if err := scanInstrumentInto(row, instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInstrumentNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) deleteInstrumentWith(ctx context.Context, execer commandTagExecutor, uid uuid.UUID) error {
	const query = `DELETE FROM instruments WHERE uid=$1`
	cmdTag, err := execer.Exec(ctx, query, uid)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrInstrumentNotFound
	}
	return nil
}

func ensureTypedRowExists(ctx context.Context, tx pgx.Tx, table string, uid uuid.UUID) error {
	query := fmt.Sprintf(`SELECT 1 FROM %s WHERE uid=$1`, table)
	var exists int
	if err := tx.QueryRow(ctx, query, uid).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInstrumentNotFound
		}
		return err
	}
	return nil
}
