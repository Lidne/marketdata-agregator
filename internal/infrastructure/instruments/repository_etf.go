package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateEtf(ctx context.Context, etf *domain.Etf) error {
	if etf == nil {
		return errors.New("etf is nil")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := r.createInstrumentWith(ctx, tx, &etf.Instrument); err != nil {
			return err
		}
		const query = `
			INSERT INTO etfs (uid, min_price_increment)
			VALUES ($1,$2)`
		if _, err := tx.Exec(ctx, query, etf.UID, etf.MinPriceIncrement); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) UpdateEtf(ctx context.Context, etf *domain.Etf) error {
	if etf == nil {
		return errors.New("etf is nil")
	}
	if etf.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "etfs", etf.UID); err != nil {
			return err
		}
		if err := r.updateInstrumentWith(ctx, tx, &etf.Instrument); err != nil {
			return err
		}
		const query = `
			UPDATE etfs
			SET min_price_increment=$2
			WHERE uid=$1`
		if cmdTag, err := tx.Exec(ctx, query, etf.UID, etf.MinPriceIncrement); err != nil {
			return err
		} else if cmdTag.RowsAffected() == 0 {
			return ErrInstrumentNotFound
		}
		return nil
	})
}

func (r *Repository) DeleteEtf(ctx context.Context, uid uuid.UUID) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "etfs", uid); err != nil {
			return err
		}
		return r.deleteInstrumentWith(ctx, tx, uid)
	})
}

func (r *Repository) GetEtf(ctx context.Context, uid uuid.UUID) (*domain.Etf, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at,
		       e.min_price_increment
		FROM instruments i
		INNER JOIN etfs e ON e.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	etf := &domain.Etf{}
	var minIncrement float64
	if err := scanInstrumentInto(row, &etf.Instrument, &minIncrement); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	etf.MinPriceIncrement = minIncrement
	return etf, nil
}
