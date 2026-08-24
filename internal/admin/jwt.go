package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is kept separate from the customer/employee auth secrets so
// tokens for one role can never be interchanged. It is nil until
// InitJWTSecret is called from main(), after config.Load() — see the
// long-standing comment history on this file: a package-level var reading
// os.Getenv at package-init time runs BEFORE main(), and therefore before
// .env is loaded, which previously caused a hardcoded fallback secret to
// be used silently. Fails fast instead.
var JWTSecret []byte

func InitJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("ADMIN_JWT_SECRET is not set — refusing to start with no admin JWT signing secret")
	}
	JWTSecret = []byte(secret)
	return nil
}

// AccessTokenTTL is short deliberately: the access token itself carries no
// server-checkable revocation state (that's what AdminSession is for), so
// keeping its blast radius small if it leaks matters more than avoiding a
// refresh round-trip.
const AccessTokenTTL = 15 * time.Minute

// RefreshTokenTTL bounds how long a session can be silently renewed
// before requiring a fresh username/password login.
const RefreshTokenTTL = 7 * 24 * time.Hour

// Claims carries enough identity to authorize every admin request without
// a DB lookup for the role, while SessionID lets the auth middleware
// still verify — with one DB lookup — that the session behind this access
// token hasn't been revoked (logout, admin disabled, etc.). Username is
// kept for backward-compatible audit display; AdminID is authoritative.
type Claims struct {
	AdminID   uint   `json:"adminId"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	SessionID uint   `json:"sessionId"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(adminID uint, username, role string, sessionID uint) (string, error) {
	claims := Claims{
		AdminID:   adminID,
		Username:  username,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// GenerateRefreshToken returns a fresh opaque random token (to hand to the
// client) and its SHA-256 hash (to store in AdminSession.RefreshTokenHash)
// — the raw token is never persisted, mirroring how passwords are only
// ever stored hashed.
func GenerateRefreshToken() (rawToken string, tokenHash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(buf)
	tokenHash = HashRefreshToken(rawToken)
	return rawToken, tokenHash, nil
}

func HashRefreshToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
