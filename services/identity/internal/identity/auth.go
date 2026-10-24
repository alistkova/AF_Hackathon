package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin   = "admin"
	RoleAnalyst = "analyst"

	DefaultSessionTTL = 7 * 24 * time.Hour
	defaultCookieName = "alfa_session"

	minPasswordLength = 8
	maxPasswordBytes  = 72
	maxEmailLength    = 254
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already registered")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidInput       = errors.New("invalid input")
)

var passwordCost = bcrypt.DefaultCost

func SessionTTL() time.Duration {
	return config.Duration("IDENTITY_SESSION_TTL", DefaultSessionTTL)
}

func cookieName() string {
	return config.String("SESSION_COOKIE_NAME", defaultCookieName)
}

func cookieSecure() bool {
	return strings.EqualFold(config.String("SESSION_COOKIE_SECURE", "false"), "true")
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateCredentials(email, password string) (string, error) {
	email = normalizeEmail(email)
	if email == "" || len(email) > maxEmailLength {
		return "", ErrInvalidInput
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidInput
	}
	if utf8.RuneCountInString(password) < minPasswordLength || len(password) > maxPasswordBytes {
		return "", ErrInvalidInput
	}
	return email, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(encoded, password string) (legacy bool, err error) {
	if strings.HasPrefix(encoded, "$2a$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2y$") {
		if bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password)) != nil {
			return false, ErrInvalidCredentials
		}
		return false, nil
	}
	if subtle.ConstantTimeCompare([]byte(encoded), []byte(password)) == 1 {
		return true, nil
	}
	return false, ErrInvalidCredentials
}

func newUserID() string {
	return "usr_" + randomHex(16)
}

func newSessionToken() string {
	return randomBase64(32)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func randomBase64(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func setSessionCookie(c *gin.Context, session Session) {
	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName(),
		Value:    session.Token,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     cookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}
