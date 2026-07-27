package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"argus/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("an account with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidToken       = errors.New("invalid or expired session token")
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const tokenTTL = 30 * 24 * time.Hour

// AuthResult bundles a user with the freshly-issued bearer token for it.
type AuthResult struct {
	User  models.User
	Token string
}

func (s *Service) Register(ctx context.Context, email, password, name string) (AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailPattern.MatchString(email) {
		return AuthResult{}, ErrInvalidEmail
	}
	if len(password) < 8 {
		return AuthResult{}, ErrWeakPassword
	}
	existing, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, err
	}
	if existing != nil {
		return AuthResult{}, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = email
	}
	id, err := s.users.CreateUser(ctx, models.User{Email: email, Name: name, PasswordHash: string(hash)})
	if err != nil {
		return AuthResult{}, err
	}
	user := models.User{ID: id, Email: email, Name: name}
	token, err := s.issueToken(ctx, id, "register")
	if err != nil {
		return AuthResult{}, err
	}
	s.logger.Add("info", "auth", "user_registered", "New user registered", nil, map[string]string{"email": email})
	return AuthResult{User: user, Token: token}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, err
	}
	if user == nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	token, err := s.issueToken(ctx, user.ID, "login")
	if err != nil {
		return AuthResult{}, err
	}
	user.PasswordHash = ""
	return AuthResult{User: *user, Token: token}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	return s.tokens.DeleteToken(ctx, hashToken(rawToken))
}

// Authenticate resolves a bearer token to its owning user, rejecting
// expired or unknown tokens using a constant-time comparison for the hash
// lookup key to reduce timing side-channels.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (*models.User, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidToken
	}
	hash := hashToken(rawToken)
	token, err := s.tokens.GetTokenByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if token == nil || subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(hash)) != 1 {
		return nil, ErrInvalidToken
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	user, err := s.users.GetUserByID(ctx, token.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidToken
	}
	_ = s.tokens.TouchToken(ctx, token.ID, time.Now().UTC())
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) issueToken(ctx context.Context, userID int64, name string) (string, error) {
	raw, err := generateToken()
	if err != nil {
		return "", err
	}
	_, err = s.tokens.CreateToken(ctx, models.AuthToken{UserID: userID, TokenHash: hashToken(raw), Name: name, ExpiresAt: time.Now().UTC().Add(tokenTTL)})
	if err != nil {
		return "", err
	}
	return raw, nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
