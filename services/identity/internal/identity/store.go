package identity

import (
	"context"
	"sync"
	"time"
)

type memSession struct {
	userID    string
	expiresAt time.Time
}

type MemStore struct {
	mu      sync.Mutex
	users   map[string]User
	pwd     map[string]string
	byEmail map[string]string
	sess    map[string]memSession
	acc     map[string]map[string]bool
}

func NewMemStore() *MemStore {
	return &MemStore{
		users:   map[string]User{},
		pwd:     map[string]string{},
		byEmail: map[string]string{},
		sess:    map[string]memSession{},
		acc:     map[string]map[string]bool{},
	}
}

func (m *MemStore) Register(_ context.Context, email, password string) (User, error) {
	email, err := validateCredentials(email, password)
	if err != nil {
		return User{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.byEmail[email]; ok {
		return User{}, ErrEmailExists
	}

	role := RoleAnalyst
	if len(m.users) == 0 {
		role = RoleAdmin
	}

	user := User{ID: newUserID(), Email: email, Role: role}
	m.users[user.ID] = user
	m.pwd[user.ID] = hash
	m.byEmail[email] = user.ID
	m.acc[user.ID] = map[string]bool{}
	return user, nil
}

func (m *MemStore) Authenticate(_ context.Context, email, password string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, ok := m.byEmail[normalizeEmail(email)]
	if !ok {
		return Session{}, ErrInvalidCredentials
	}
	if _, err := verifyPassword(m.pwd[id], password); err != nil {
		return Session{}, ErrInvalidCredentials
	}

	token := newSessionToken()
	expiresAt := time.Now().Add(SessionTTL())
	m.sess[hashToken(token)] = memSession{userID: id, expiresAt: expiresAt}

	return Session{Token: token, ExpiresAt: expiresAt, User: m.users[id]}, nil
}

func (m *MemStore) Logout(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sess, hashToken(token))
	return nil
}

func (m *MemStore) Me(_ context.Context, token string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := hashToken(token)
	session, ok := m.sess[key]
	if !ok {
		return User{}, ErrUnauthorized
	}
	if time.Now().After(session.expiresAt) {
		delete(m.sess, key)
		return User{}, ErrUnauthorized
	}

	user, ok := m.users[session.userID]
	if !ok {
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func (m *MemStore) AccessibleCompanies(_ context.Context, userID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]string, 0, len(m.acc[userID]))
	for companyID := range m.acc[userID] {
		items = append(items, companyID)
	}
	return items, nil
}

func (m *MemStore) GrantAccess(_ context.Context, userID, companyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[userID]; !ok {
		return ErrUserNotFound
	}
	if m.acc[userID] == nil {
		m.acc[userID] = map[string]bool{}
	}
	m.acc[userID][companyID] = true
	return nil
}

func (m *MemStore) RevokeAccess(_ context.Context, userID, companyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.acc[userID], companyID)
	return nil
}
