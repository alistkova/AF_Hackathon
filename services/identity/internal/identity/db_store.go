package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBStore struct {
	pool *pgxpool.Pool
}

func NewDBStore(pool *pgxpool.Pool) *DBStore {
	return &DBStore{pool: pool}
}

func (d *DBStore) Register(email, password string) (User, error) {
	id := fmt.Sprintf("u%d", time.Now().UnixNano())
	_, err := d.pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)`, id, email, password)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Email: email}, nil
}

func (d *DBStore) Authenticate(email, password string) (Session, error) {
	var id string
	err := d.pool.QueryRow(context.Background(), `SELECT id FROM users WHERE email=$1 AND password_hash=$2`, email, password).Scan(&id)
	if err != nil {
		return Session{}, errors.New("invalid")
	}
	token := "t" + id
	_, err = d.pool.Exec(context.Background(), `INSERT INTO sessions (token, user_id) VALUES ($1,$2) ON CONFLICT (token) DO NOTHING`, token, id)
	if err != nil {
		return Session{}, err
	}
	return Session{Token: token, User: User{ID: id, Email: email}}, nil
}

func (d *DBStore) Logout(token string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM sessions WHERE token=$1`, token)
	return err
}

func (d *DBStore) Me(token string) (User, error) {
	var id, email string
	err := d.pool.QueryRow(context.Background(), `SELECT u.id, u.email FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=$1`, token).Scan(&id, &email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, errors.New("unauthorized")
		}
		return User{}, err
	}
	return User{ID: id, Email: email}, nil
}

func (d *DBStore) AccessibleCompanies(userID string) ([]string, error) {
	rows, err := d.pool.Query(context.Background(), `SELECT company_id FROM company_access WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DBStore) GrantAccess(userID, companyID string) error {
	_, err := d.pool.Exec(context.Background(), `INSERT INTO company_access (user_id, company_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, companyID)
	return err
}

func (d *DBStore) RevokeAccess(userID, companyID string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM company_access WHERE user_id=$1 AND company_id=$2`, userID, companyID)
	return err
}
