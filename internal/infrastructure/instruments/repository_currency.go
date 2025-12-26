package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateCurrency(ctx context.Context, currency *domain.Currency) error {
	if currency == nil {
		return errors.New("currency is nil")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := r.createInstrumentWith(ctx, tx, &currency.Instrument); err != nil {
			return err
		}
		const query = `INSERT INTO currencies (uid) VALUES ($1)`
		if _, err := tx.Exec(ctx, query, currency.UID); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) UpdateCurrency(ctx context.Context, currency *domain.Currency) error {
	if currency == nil {
		return errors.New("currency is nil")
	}
	if currency.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "currencies", currency.UID); err != nil {
			return err
		}
		return r.updateInstrumentWith(ctx, tx, &currency.Instrument)
	})
}

func (r *Repository) DeleteCurrency(ctx context.Context, uid uuid.UUID) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "currencies", uid); err != nil {
			return err
		}
		return r.deleteInstrumentWith(ctx, tx, uid)
	})
}

func (r *Repository) GetCurrency(ctx context.Context, uid uuid.UUID) (*domain.Currency, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at
		FROM instruments i
		INNER JOIN currencies c ON c.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	currency := &domain.Currency{}
	if err := scanInstrumentInto(row, &currency.Instrument); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	return currency, nil
}
