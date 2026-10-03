package shop

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Auth mints and verifies stateless bearer tokens:
//
//	base64url("username|expiryUnix") + "." + base64url(HMAC-SHA256(secret, "username|expiryUnix"))
type Auth struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewAuth(secret string, ttl time.Duration) *Auth {
	return &Auth{secret: []byte(secret), ttl: ttl, now: time.Now}
}

const maxUsernameLen = 128

var errBadToken = errors.New("invalid or expired token")

func (a *Auth) sign(payload string) []byte {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// Mint returns a token for username and its expiry time.
func (a *Auth) Mint(username string) (string, time.Time) {
	exp := a.now().Add(a.ttl).Truncate(time.Second)
	payload := username + "|" + strconv.FormatInt(exp.Unix(), 10)
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(a.sign(payload)), exp
}

// Verify returns the username for a valid, unexpired token.
func (a *Auth) Verify(token string) (string, error) {
	dot := strings.IndexByte(token, '.')
	if dot <= 0 || dot == len(token)-1 {
		return "", errBadToken
	}
	enc := base64.RawURLEncoding
	payload, err := enc.DecodeString(token[:dot])
	if err != nil {
		return "", errBadToken
	}
	sig, err := enc.DecodeString(token[dot+1:])
	if err != nil || !hmac.Equal(sig, a.sign(string(payload))) {
		return "", errBadToken
	}
	p := string(payload)
	bar := strings.LastIndexByte(p, '|')
	if bar <= 0 {
		return "", errBadToken
	}
	exp, err := strconv.ParseInt(p[bar+1:], 10, 64)
	if err != nil || a.now().Unix() >= exp {
		return "", errBadToken
	}
	return p[:bar], nil
}

// bearer extracts the token from an Authorization header.
func bearer(h string) string {
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}
