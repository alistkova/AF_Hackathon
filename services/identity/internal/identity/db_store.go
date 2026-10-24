package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBStore struct {
	pool *pgxpool.Pool
}

func NewDBStore(pool *pgxpool.Pool) *DBStore {
	return &DBStore{pool: pool}
}

func (d *DBStore) Register(ctx context.Context, email, password string) (User, error) {
	email, err := validateCredentials(email, password)
	if err != nil {
		return User{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}

	user := User{ID: newUserID(), Email: email}
	err = d.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		SELECT $1, $2, $3, CASE WHEN EXISTS (SELECT 1 FROM users) THEN $4 ELSE $5 END
		RETURNING role
	`, user.ID, user.Email, hash, RoleAnalyst, RoleAdmin).Scan(&user.Role)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}

	return user, nil
}

func (d *DBStore) Authenticate(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)

	var (
		id   string
		hash string
		role string
	)
	err := d.pool.QueryRow(ctx, `
		SELECT id, password_hash, role FROM users WHERE email = $1
	`, email).Scan(&id, &hash, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}

	legacy, err := verifyPassword(hash, password)
	if err != nil {
		return Session{}, err
	}
	if legacy {
		if upgraded, hashErr := hashPassword(password); hashErr == nil {
			_, _ = d.pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, upgraded)
		}
	}

	token := newSessionToken()
	expiresAt := time.Now().Add(SessionTTL())
	_, err = d.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)
	`, hashToken(token), id, expiresAt)
	if err != nil {
		return Session{}, err
	}

	_, _ = d.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)

	return Session{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      User{ID: id, Email: email, Role: role},
	}, nil
}

func (d *DBStore) Logout(ctx context.Context, token string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, hashToken(token))
	return err
}

func (d *DBStore) Me(ctx context.Context, token string) (User, error) {
	var user User
	err := d.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.role
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()
	`, hashToken(token)).Scan(&user.ID, &user.Email, &user.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (d *DBStore) AccessibleCompanies(ctx context.Context, userID string) ([]string, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT company_id FROM company_access WHERE user_id = $1 ORDER BY company_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var companyID string
		if err := rows.Scan(&companyID); err != nil {
			return nil, err
		}
		items = append(items, companyID)
	}
	return items, rows.Err()
}

func (d *DBStore) GrantAccess(ctx context.Context, userID, companyID string) error {
	if err := d.requireUser(ctx, userID); err != nil {
		return err
	}
	_, err := d.pool.Exec(ctx, `
		INSERT INTO company_access (user_id, company_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, companyID)
	return err
}

func (d *DBStore) RevokeAccess(ctx context.Context, userID, companyID string) error {
	_, err := d.pool.Exec(ctx, `
		DELETE FROM company_access WHERE user_id = $1 AND company_id = $2
	`, userID, companyID)
	return err
}

func (d *DBStore) requireUser(ctx context.Context, userID string) error {
	var exists bool
	if err := d.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrUserNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
