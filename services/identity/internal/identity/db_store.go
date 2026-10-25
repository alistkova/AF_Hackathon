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

const registrationLockKey int64 = 0x6964656e74697479

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
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, registrationLockKey); err != nil {
		return User{}, err
	}

	var users int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		return User{}, err
	}
	user.Role = RoleAnalyst
	if users == 0 {
		user.Role = RoleAdmin
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, user.ID, user.Email, hash, user.Role); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (d *DBStore) Authenticate(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)

	var (
		id               string
		hash             string
		role             string
		primaryCompanyID string
	)
	err := d.pool.QueryRow(ctx, `
		SELECT id, password_hash, role, primary_company_id FROM users WHERE email = $1
	`, email).Scan(&id, &hash, &role, &primaryCompanyID)
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
		User:      User{ID: id, Email: email, Role: role, PrimaryCompanyID: primaryCompanyID},
	}, nil
}

func (d *DBStore) Logout(ctx context.Context, token string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, hashToken(token))
	return err
}

func (d *DBStore) Me(ctx context.Context, token string) (User, error) {
	var user User
	err := d.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.role, u.primary_company_id
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()
	`, hashToken(token)).Scan(&user.ID, &user.Email, &user.Role, &user.PrimaryCompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (d *DBStore) UpdateProfile(ctx context.Context, userID string, input ProfileUpdate) (User, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if input.Email != nil {
		if _, err := tx.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1`, userID, *input.Email); err != nil {
			if isUniqueViolation(err) {
				return User{}, ErrEmailExists
			}
			return User{}, err
		}
	}
	if input.PrimaryCompanyID != nil {
		if _, err := tx.Exec(ctx, `UPDATE users SET primary_company_id = $2 WHERE id = $1`, userID, *input.PrimaryCompanyID); err != nil {
			return User{}, err
		}
	}

	var user User
	err = tx.QueryRow(ctx, `
		SELECT id, email, role, primary_company_id FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Role, &user.PrimaryCompanyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (d *DBStore) ChangePassword(ctx context.Context, userID, token, current, next string) error {
	var hash string
	err := d.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if _, err := verifyPassword(hash, current); err != nil {
		return ErrInvalidCredentials
	}

	nextHash, err := hashPassword(next)
	if err != nil {
		return err
	}

	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, nextHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM sessions WHERE user_id = $1 AND token <> $2
	`, userID, hashToken(token)); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (d *DBStore) Memberships(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT company_id, role FROM company_access WHERE user_id = $1 ORDER BY company_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Membership, 0)
	for rows.Next() {
		var item Membership
		if err := rows.Scan(&item.CompanyID, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DBStore) Members(ctx context.Context, companyID string) ([]Member, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT u.id, u.email, ca.role
		FROM company_access ca
		JOIN users u ON u.id = ca.user_id
		WHERE ca.company_id = $1
		ORDER BY CASE ca.role WHEN 'owner' THEN 0 ELSE 1 END, u.email
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Member, 0)
	for rows.Next() {
		var item Member
		if err := rows.Scan(&item.UserID, &item.Email, &item.Role); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *DBStore) MemberRole(ctx context.Context, companyID, userID string) (string, error) {
	var role string
	err := d.pool.QueryRow(ctx, `
		SELECT role FROM company_access WHERE company_id = $1 AND user_id = $2
	`, companyID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMemberNotFound
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

func (d *DBStore) AddMember(ctx context.Context, companyID, email string) (Member, error) {
	email, err := validateEmail(email)
	if err != nil {
		return Member{}, err
	}

	var userID string
	err = d.pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrUserNotFound
	}
	if err != nil {
		return Member{}, err
	}

	if _, err := d.pool.Exec(ctx, `
		INSERT INTO company_access (user_id, company_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, company_id) DO NOTHING
	`, userID, companyID, CompanyRoleMember); err != nil {
		return Member{}, err
	}

	var member Member
	err = d.pool.QueryRow(ctx, `
		SELECT u.id, u.email, ca.role
		FROM company_access ca
		JOIN users u ON u.id = ca.user_id
		WHERE ca.company_id = $1 AND ca.user_id = $2
	`, companyID, userID).Scan(&member.UserID, &member.Email, &member.Role)
	if err != nil {
		return Member{}, err
	}
	return member, nil
}

func (d *DBStore) RemoveMember(ctx context.Context, companyID, userID string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var role string
	err = tx.QueryRow(ctx, `
		SELECT role FROM company_access WHERE company_id = $1 AND user_id = $2
	`, companyID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}

	if role == CompanyRoleOwner {
		var owners int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM company_access WHERE company_id = $1 AND role = $2
		`, companyID, CompanyRoleOwner).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM company_access WHERE company_id = $1 AND user_id = $2
	`, companyID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET primary_company_id = '' WHERE id = $1 AND primary_company_id = $2
	`, userID, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DBStore) GrantAccess(ctx context.Context, userID, companyID, role string) error {
	if err := d.requireUser(ctx, userID); err != nil {
		return err
	}
	_, err := d.pool.Exec(ctx, `
		INSERT INTO company_access (user_id, company_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, company_id) DO UPDATE SET role = EXCLUDED.role
	`, userID, companyID, role)
	return err
}

func (d *DBStore) RevokeAccess(ctx context.Context, userID, companyID string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		DELETE FROM company_access WHERE user_id = $1 AND company_id = $2
	`, userID, companyID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET primary_company_id = '' WHERE id = $1 AND primary_company_id = $2
	`, userID, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DBStore) PruneSessions(ctx context.Context) (int64, error) {
	tag, err := d.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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
