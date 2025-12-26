package instruments

import (
	"context"
	"errors"

	domain "main/internal/domain/entity/instruments"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateBond(ctx context.Context, bond *domain.Bond) error {
	if bond == nil {
		return errors.New("bond is nil")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := r.createInstrumentWith(ctx, tx, &bond.Instrument); err != nil {
			return err
		}
		const query = `
			INSERT INTO bonds (uid, nominal, aci_value)
			VALUES ($1,$2,$3)`
		if _, err := tx.Exec(ctx, query, bond.UID, bond.Nominal, bond.AciValue); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) UpdateBond(ctx context.Context, bond *domain.Bond) error {
	if bond == nil {
		return errors.New("bond is nil")
	}
	if bond.UID == uuid.Nil {
		return errors.New("instrument UID is required")
	}
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "bonds", bond.UID); err != nil {
			return err
		}
		if err := r.updateInstrumentWith(ctx, tx, &bond.Instrument); err != nil {
			return err
		}
		const query = `
			UPDATE bonds
			SET nominal=$2,
				aci_value=$3
			WHERE uid=$1`
		if cmdTag, err := tx.Exec(ctx, query, bond.UID, bond.Nominal, bond.AciValue); err != nil {
			return err
		} else if cmdTag.RowsAffected() == 0 {
			return ErrInstrumentNotFound
		}
		return nil
	})
}

func (r *Repository) DeleteBond(ctx context.Context, uid uuid.UUID) error {
	return r.withTx(ctx, func(tx pgx.Tx) error {
		if err := ensureTypedRowExists(ctx, tx, "bonds", uid); err != nil {
			return err
		}
		return r.deleteInstrumentWith(ctx, tx, uid)
	})
}

func (r *Repository) GetBond(ctx context.Context, uid uuid.UUID) (*domain.Bond, error) {
	const query = `
		SELECT i.uid, i.figi, i.ticker, i.lot, i.class_code, i.logo_url, i.created_at, i.updated_at, i.deleted_at,
		       b.nominal, b.aci_value
		FROM instruments i
		INNER JOIN bonds b ON b.uid = i.uid
		WHERE i.uid = $1`

	row := r.pool.QueryRow(ctx, query, uid)
	bond := &domain.Bond{}
	var nominal, aciValue float64
	if err := scanInstrumentInto(row, &bond.Instrument, &nominal, &aciValue); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInstrumentNotFound
		}
		return nil, err
	}
	bond.Nominal = nominal
	bond.AciValue = aciValue
	return bond, nil
}
