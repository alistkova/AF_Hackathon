package identity

import (
	"context"
	"errors"
	"sync"
)

type MemStore struct {
	mu    sync.Mutex
	seq   int
	users map[string]User
	pwd   map[string]string
	sess  map[string]string
	acc   map[string]map[string]bool
}

func NewMemStore() *MemStore {
	return &MemStore{users: map[string]User{}, pwd: map[string]string{}, sess: map[string]string{}, acc: map[string]map[string]bool{}}
}

func (m *MemStore) Register(email, password string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	u := User{ID: "u" + itoa(m.seq), Email: email}
	m.users[u.ID] = u
	m.pwd[u.ID] = password
	m.acc[u.ID] = map[string]bool{}
	return u, nil
}

func (m *MemStore) Authenticate(email, password string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, u := range m.users {
		if u.Email == email && m.pwd[id] == password {
			token := "t" + id
			m.sess[token] = id
			return Session{Token: token, User: u}, nil
		}
	}
	return Session{}, errors.New("invalid")
}

func (m *MemStore) Logout(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sess, token)
	return nil
}

func (m *MemStore) Me(token string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.sess[token]
	if !ok {
		return User{}, errors.New("unauthorized")
	}
	u, ok := m.users[id]
	if !ok {
		return User{}, errors.New("not found")
	}
	return u, nil
}

func (m *MemStore) AccessibleCompanies(userID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for c := range m.acc[userID] {
		out = append(out, c)
	}
	return out, nil
}

func (m *MemStore) GrantAccess(userID, companyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.acc[userID] == nil {
		m.acc[userID] = map[string]bool{}
	}
	m.acc[userID][companyID] = true
	return nil
}

func (m *MemStore) RevokeAccess(userID, companyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.acc[userID], companyID)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

var _ = context.Background
