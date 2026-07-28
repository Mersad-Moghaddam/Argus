package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"argus/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken           = errors.New("an account with this email already exists")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrInvalidEmail         = errors.New("invalid email address")
	ErrWeakPassword         = errors.New("password must be at least 8 characters")
	ErrInvalidToken         = errors.New("invalid or expired session token")
	ErrCurrentPassword      = errors.New("current password is incorrect")
	ErrInvalidRecoveryToken = errors.New("invalid or expired recovery token")
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

const tokenTTL = 30 * 24 * time.Hour
const lastUsedWriteInterval = 15 * time.Minute
const passwordRecoveryTTL = 30 * time.Minute

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

func (s *Service) ListSessions(ctx context.Context, userID int64, currentRawToken string) ([]models.AuthToken, error) {
	tokens, err := s.tokens.ListTokensByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	currentHash := hashToken(currentRawToken)
	now := time.Now().UTC()
	visible := make([]models.AuthToken, 0, len(tokens))
	for _, token := range tokens {
		if now.After(token.ExpiresAt) {
			continue
		}
		token.Current = subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(currentHash)) == 1
		token.TokenHash = ""
		visible = append(visible, token)
	}
	return visible, nil
}

func (s *Service) RevokeOtherSessions(ctx context.Context, userID int64, currentRawToken string) error {
	if strings.TrimSpace(currentRawToken) == "" {
		return ErrInvalidToken
	}
	return s.tokens.DeleteTokensByUserExcept(ctx, userID, hashToken(currentRawToken))
}

// ChangePassword verifies the active account credential before writing a new
// bcrypt hash. Other sessions are revoked so a leaked older session cannot
// continue after a password change; the current cookie session remains valid.
func (s *Service) ChangePassword(ctx context.Context, userID int64, currentRawToken, currentPassword, newPassword string) error {
	if strings.TrimSpace(currentRawToken) == "" {
		return ErrInvalidToken
	}
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	user, err := s.users.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return ErrCurrentPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.users.UpdateUserPassword(ctx, userID, string(hash)); err != nil {
		return err
	}
	return s.tokens.DeleteTokensByUserExcept(ctx, userID, hashToken(currentRawToken))
}

// RequestPasswordRecovery is deliberately non-enumerating: it returns nil for
// unknown addresses and for deployments where delivery is not configured. A
// configured delivery integration receives the raw one-time token directly;
// MySQL stores only its hash.
func (s *Service) RequestPasswordRecovery(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailPattern.MatchString(email) || s.passwordRecovery == nil || s.recoveryDelivery == nil {
		return nil
	}
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil || user == nil {
		return err
	}
	if err = s.passwordRecovery.DeletePasswordRecoveryTokensByUser(ctx, user.ID); err != nil {
		return err
	}
	raw, err := generateToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(passwordRecoveryTTL)
	if _, err = s.passwordRecovery.CreatePasswordRecoveryToken(ctx, models.PasswordRecoveryToken{UserID: user.ID, TokenHash: hashToken(raw), ExpiresAt: expiresAt}); err != nil {
		return err
	}
	if err = s.recoveryDelivery.DeliverPasswordRecovery(ctx, user.Email, raw, expiresAt); err != nil {
		// A failed delivery must not leave a usable credential behind.
		_ = s.passwordRecovery.DeletePasswordRecoveryTokensByUser(ctx, user.ID)
		return err
	}
	s.logger.Add("info", "auth", "password_recovery_requested", "Password recovery requested", nil, map[string]string{"email": email})
	return nil
}

// CompletePasswordRecovery consumes the one-time token before changing the
// credential, then revokes every browser/API session. Consumption wins races;
// if the password write fails the caller must request a fresh recovery token.
func (s *Service) CompletePasswordRecovery(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	if strings.TrimSpace(rawToken) == "" || s.passwordRecovery == nil {
		return ErrInvalidRecoveryToken
	}
	now := time.Now().UTC()
	token, err := s.passwordRecovery.ConsumePasswordRecoveryToken(ctx, hashToken(rawToken), now)
	if err != nil {
		return err
	}
	if token == nil {
		return ErrInvalidRecoveryToken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err = s.users.UpdateUserPassword(ctx, token.UserID, string(hash)); err != nil {
		return err
	}
	if err = s.tokens.DeleteTokensByUserExcept(ctx, token.UserID, ""); err != nil {
		return err
	}
	_ = s.passwordRecovery.DeletePasswordRecoveryTokensByUser(ctx, token.UserID)
	s.logger.Add("info", "auth", "password_recovery_completed", "Password recovery completed", nil, map[string]string{"userId": strconv.FormatInt(token.UserID, 10)})
	return nil
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
	now := time.Now().UTC()
	if token.LastUsedAt == nil || now.Sub(*token.LastUsedAt) >= lastUsedWriteInterval {
		_ = s.tokens.TouchToken(ctx, token.ID, now)
	}
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
