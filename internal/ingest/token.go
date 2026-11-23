package ingest

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const TokenEnv = "AGENT_TIMELINE_INGEST_TOKEN"

type Auth struct{ digest [sha256.Size]byte }

func NewAuth(token string) Auth { return Auth{digest: sha256.Sum256([]byte(token))} }

func (a Auth) Valid(header http.Header) bool {
	values := header.Values("Authorization")
	if len(values) != 1 {
		return false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	supplied := sha256.Sum256([]byte(parts[1]))
	return subtle.ConstantTimeCompare(a.digest[:], supplied[:]) == 1
}

func LoadAuth(path string) (Auth, error) {
	if token := os.Getenv(TokenEnv); token != "" {
		return authFromToken(token)
	}
	auth, err := readToken(path)
	if !errors.Is(err, os.ErrNotExist) {
		return auth, err
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return Auth{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(secret[:])
	file, err := os.CreateTemp(filepath.Dir(path), ".ingest-token-*")
	if err != nil {
		return Auth{}, err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return Auth{}, err
	}
	_, writeErr := file.WriteString(token + "\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return Auth{}, err
	}
	if err := os.Link(file.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return readToken(path)
		}
		return Auth{}, err
	}
	return NewAuth(token), nil
}

func readToken(path string) (Auth, error) {
	file, err := os.Open(path)
	if err != nil {
		return Auth{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Auth{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return Auth{}, errors.New("invalid ingest token file")
	}
	if info.Mode().Perm() != 0600 {
		if err := file.Chmod(0600); err != nil {
			return Auth{}, err
		}
	}
	body := make([]byte, info.Size())
	if _, err := file.ReadAt(body, 0); err != nil && len(body) > 0 {
		return Auth{}, err
	}
	return authFromToken(strings.TrimSpace(string(body)))
}

func authFromToken(token string) (Auth, error) {
	if token == "" || strings.Join(strings.Fields(token), "") != token {
		return Auth{}, errors.New("ingest token must be a nonempty bearer token without whitespace")
	}
	return NewAuth(token), nil
}
