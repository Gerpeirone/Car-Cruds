package business

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var (
	ErrAccountInput = errors.New("invalid account input")
	ErrAccountAuth  = errors.New("invalid account credentials")
)

type Account struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}

type AccountStore interface {
	InsertAccount(context.Context, Account, string) error
	AccountByEmail(context.Context, string) (Account, string, error)
	InsertSession(context.Context, string, []byte, time.Time) error
	AccountBySession(context.Context, []byte) (Account, error)
	DeleteSession(context.Context, []byte) error
}

type Accounts struct {
	Store AccountStore
	TTL   time.Duration
}

func NormalizeEmail(value string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(email)
	domain := email[strings.LastIndexByte(email, '@')+1:]
	return email, err == nil && len(email) <= 254 && address.Address == email && strings.Contains(domain, ".")
}

func validName(value string) bool {
	return value != "" && utf8.RuneCountInString(value) <= 80 && strings.IndexFunc(value, unicode.IsControl) == -1
}

func (s Accounts) Register(ctx context.Context, first, last, email, password, confirmation string) error {
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)
	email, valid := NormalizeEmail(email)
	if !valid || !validName(first) || !validName(last) || len(password) > 1024 ||
		utf8.RuneCountInString(password) < 12 || password != confirmation {
		return ErrAccountInput
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.Store.InsertAccount(ctx, Account{ID: uuid.NewString(), FirstName: first, LastName: last, Email: email, Role: "client"}, hash)
}

func (s Accounts) Login(ctx context.Context, email, password string) (Account, string, time.Time, error) {
	email, valid := NormalizeEmail(email)
	if !valid || password == "" || len(password) > 1024 {
		return Account{}, "", time.Time{}, ErrAccountAuth
	}
	account, hash, err := s.Store.AccountByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrAccountAuth) {
		return Account{}, "", time.Time{}, err
	}
	if errors.Is(err, ErrAccountAuth) {
		// Match the work of verifying an existing account.
		hash = dummyPasswordHash
	}
	if !verifyPassword(password, hash) || err != nil {
		return Account{}, "", time.Time{}, ErrAccountAuth
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return Account{}, "", time.Time{}, err
	}
	value := hex.EncodeToString(token)
	digest := sha256.Sum256(token)
	expires := time.Now().UTC().Add(s.TTL).Truncate(time.Second)
	err = s.Store.InsertSession(ctx, account.ID, digest[:], expires)
	return account, value, expires, err
}

func sessionDigest(value string) ([]byte, error) {
	if len(value) != 64 {
		return nil, ErrAccountAuth
	}
	token, err := hex.DecodeString(value)
	if err != nil {
		return nil, ErrAccountAuth
	}
	digest := sha256.Sum256(token)
	return digest[:], nil
}

func (s Accounts) Profile(ctx context.Context, token string) (Account, error) {
	digest, err := sessionDigest(token)
	if err != nil {
		return Account{}, err
	}
	return s.Store.AccountBySession(ctx, digest)
}

func (s Accounts) Logout(ctx context.Context, token string) error {
	digest, err := sessionDigest(token)
	if err != nil {
		return nil
	}
	return s.Store.DeleteSession(ctx, digest)
}

const passwordPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"
const dummyPasswordHash = passwordPrefix + "AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func derivePassword(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(derivePassword(password, salt)), nil
}

func verifyPassword(password, stored string) bool {
	body, ok := strings.CutPrefix(stored, passwordPrefix)
	if !ok {
		return false
	}
	saltText, hashText, ok := strings.Cut(body, "$")
	if !ok {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(saltText)
	hash, hashErr := base64.RawStdEncoding.DecodeString(hashText)
	return saltErr == nil && hashErr == nil && len(salt) == 16 && len(hash) == 32 &&
		subtle.ConstantTimeCompare(derivePassword(password, salt), hash) == 1
}
