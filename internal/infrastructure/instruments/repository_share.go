package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateShare(ctx context.Context, share *domain.Share) error {
	if share == nil {
		return errors.New("share is nil")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := r.createInstrumentWith(ctx, tx, &share.Instrument); err != nil {
			return err
		}
		const query = `INSERT INTO shares (uid) VALUES ($1)`
		if _, err := tx.Exec(ctx, query, share.UID); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) UpdateShare(ctx context.Context, share *domain.Share) error {
	if share == nil {
		return errors.New("share is nil")
	}
	if share.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "shares", share.UID); err != nil {
			return err
		}
		return r.updateInstrumentWith(ctx, tx, &share.Instrument)
	})
}

func (r *Repository) DeleteShare(ctx context.Context, uid uuid.UUID) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "shares", uid); err != nil {
			return err
		}
		return r.deleteInstrumentWith(ctx, tx, uid)
	})
}

func (r *Repository) GetShare(ctx context.Context, uid uuid.UUID) (*domain.Share, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at
		FROM instruments i
		INNER JOIN shares s ON s.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	share := &domain.Share{}
	if err := scanInstrumentInto(row, &share.Instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return share, nil
}
