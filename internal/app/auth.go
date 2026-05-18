package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const sessionCookie = "nixhostforge_session"

func (a *App) hasAdmin(ctx context.Context) (bool, error) {
	_, ok, err := a.store.GetSetting(ctx, "admin_password_hash")
	return ok, err
}

func (a *App) setAdminPassword(ctx context.Context, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return a.store.SetSetting(ctx, "admin_password_hash", hash)
}

func (a *App) verifyAdminPassword(ctx context.Context, password string) (bool, error) {
	hash, ok, err := a.store.GetSetting(ctx, "admin_password_hash")
	if err != nil || !ok {
		return false, err
	}
	return verifyPassword(password, hash)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	timeCost := uint32(3)
	memory := uint32(64 * 1024)
	threads := uint8(2)
	keyLen := uint32(32)
	hash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, timeCost, threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid password hash")
	}
	var memory uint32
	var timeCost uint32
	var threads uint8
	for _, part := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "m":
			v, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return false, err
			}
			memory = uint32(v)
		case "t":
			v, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return false, err
			}
			timeCost = uint32(v)
		case "p":
			v, err := strconv.ParseUint(kv[1], 10, 8)
			if err != nil {
				return false, err
			}
			threads = uint8(v)
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}

func (a *App) createSession(ctx context.Context, w http.ResponseWriter) error {
	token, err := randomToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	expires := now.Add(30 * 24 * time.Hour)
	if _, err := a.store.db.ExecContext(ctx, `insert into sessions(token_hash, created_at, expires_at) values(?, ?, ?)`, tokenHash(token), now.Format(time.RFC3339Nano), expires.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: expires})
	return nil
}

func (a *App) destroySession(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_, _ = a.store.db.ExecContext(ctx, `delete from sessions where token_hash = ?`, tokenHash(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (a *App) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	var expires string
	err = a.store.db.QueryRowContext(r.Context(), `select expires_at from sessions where token_hash = ?`, tokenHash(cookie.Value)).Scan(&expires)
	if err != nil {
		return false
	}
	parsed, err := time.Parse(time.RFC3339Nano, expires)
	return err == nil && time.Now().UTC().Before(parsed)
}
