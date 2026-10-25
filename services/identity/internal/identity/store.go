package identity

import (
	"context"
	"sort"
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
	acc     map[string]map[string]string
}

func NewMemStore() *MemStore {
	return &MemStore{
		users:   map[string]User{},
		pwd:     map[string]string{},
		byEmail: map[string]string{},
		sess:    map[string]memSession{},
		acc:     map[string]map[string]string{},
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
	m.acc[user.ID] = map[string]string{}
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
	sort.Strings(items)
	return items, nil
}

func (m *MemStore) Memberships(_ context.Context, userID string) ([]Membership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]Membership, 0, len(m.acc[userID]))
	for companyID, role := range m.acc[userID] {
		items = append(items, Membership{CompanyID: companyID, Role: role})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CompanyID < items[j].CompanyID })
	return items, nil
}

func (m *MemStore) Members(_ context.Context, companyID string) ([]Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]Member, 0)
	for userID, companies := range m.acc {
		role, ok := companies[companyID]
		if !ok {
			continue
		}
		user, exists := m.users[userID]
		if !exists {
			continue
		}
		items = append(items, Member{UserID: userID, Email: user.Email, Role: role})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Role != items[j].Role {
			return items[i].Role == CompanyRoleOwner
		}
		return items[i].Email < items[j].Email
	})
	return items, nil
}

func (m *MemStore) MemberRole(_ context.Context, companyID, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	role, ok := m.acc[userID][companyID]
	if !ok {
		return "", ErrMemberNotFound
	}
	return role, nil
}

func (m *MemStore) AddMember(_ context.Context, companyID, email string) (Member, error) {
	email, err := validateEmail(email)
	if err != nil {
		return Member{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	userID, ok := m.byEmail[email]
	if !ok {
		return Member{}, ErrUserNotFound
	}
	if m.acc[userID] == nil {
		m.acc[userID] = map[string]string{}
	}
	if _, exists := m.acc[userID][companyID]; !exists {
		m.acc[userID][companyID] = CompanyRoleMember
	}
	return Member{UserID: userID, Email: email, Role: m.acc[userID][companyID]}, nil
}

func (m *MemStore) RemoveMember(_ context.Context, companyID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	role, ok := m.acc[userID][companyID]
	if !ok {
		return ErrMemberNotFound
	}
	if role == CompanyRoleOwner {
		owners := 0
		for _, companies := range m.acc {
			if companies[companyID] == CompanyRoleOwner {
				owners++
			}
		}
		if owners <= 1 {
			return ErrLastOwner
		}
	}

	delete(m.acc[userID], companyID)
	if user, exists := m.users[userID]; exists && user.PrimaryCompanyID == companyID {
		user.PrimaryCompanyID = ""
		m.users[userID] = user
	}
	return nil
}

func (m *MemStore) UpdateProfile(_ context.Context, userID string, input ProfileUpdate) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}

	if input.Email != nil {
		email := *input.Email
		if existing, exists := m.byEmail[email]; exists && existing != userID {
			return User{}, ErrEmailExists
		}
		delete(m.byEmail, user.Email)
		m.byEmail[email] = userID
		user.Email = email
	}
	if input.PrimaryCompanyID != nil {
		user.PrimaryCompanyID = *input.PrimaryCompanyID
	}
	m.users[userID] = user
	return user, nil
}

func (m *MemStore) ChangePassword(_ context.Context, userID, token, current, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash, ok := m.pwd[userID]
	if !ok {
		return ErrUserNotFound
	}
	if _, err := verifyPassword(hash, current); err != nil {
		return ErrInvalidCredentials
	}

	nextHash, err := hashPassword(next)
	if err != nil {
		return err
	}
	m.pwd[userID] = nextHash

	keep := hashToken(token)
	for key, session := range m.sess {
		if session.userID == userID && key != keep {
			delete(m.sess, key)
		}
	}
	return nil
}

func (m *MemStore) GrantAccess(_ context.Context, userID, companyID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[userID]; !ok {
		return ErrUserNotFound
	}
	if m.acc[userID] == nil {
		m.acc[userID] = map[string]string{}
	}
	m.acc[userID][companyID] = role
	return nil
}

func (m *MemStore) RevokeAccess(_ context.Context, userID, companyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.acc[userID], companyID)
	if user, exists := m.users[userID]; exists && user.PrimaryCompanyID == companyID {
		user.PrimaryCompanyID = ""
		m.users[userID] = user
	}
	return nil
}

func (m *MemStore) PruneSessions(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var removed int64
	for key, session := range m.sess {
		if now.After(session.expiresAt) {
			delete(m.sess, key)
			removed++
		}
	}
	return removed, nil
}
