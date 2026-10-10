package repository

import (
	"context"
	"database/sql"
	"time"
)

type PushDevice struct {
	ID    int64
	Token string
}
type DeviceRepository struct{ db *sql.DB }

func NewDeviceRepository(db *sql.DB) *DeviceRepository { return &DeviceRepository{db: db} }

func (r *DeviceRepository) Register(ctx context.Context, sub, token string, expiry time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO push_devices (user_sub, token, expires_at) VALUES ($1,$2,$3)
 ON CONFLICT (token) DO UPDATE SET user_sub=EXCLUDED.user_sub, expires_at=EXCLUDED.expires_at`, sub, token, expiry)
	return err
}
func (r *DeviceRepository) Unregister(ctx context.Context, sub, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_devices WHERE user_sub=$1 AND token=$2`, sub, token)
	return err
}
func (r *DeviceRepository) ActiveAfter(ctx context.Context, after int64) ([]PushDevice, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,token FROM push_devices WHERE expires_at > CURRENT_TIMESTAMP AND id > $1 ORDER BY id LIMIT 500`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PushDevice{}
	for rows.Next() {
		var d PushDevice
		if err := rows.Scan(&d.ID, &d.Token); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (r *DeviceRepository) DeleteInvalid(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_devices WHERE token=$1`, token)
	return err
}
