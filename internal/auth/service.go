package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/mailer"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrEmailTaken         = errors.New("email already taken")
	ErrInvalidCreds       = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrOrgNotFound        = errors.New("organization not found")
	ErrInvalidResetToken  = errors.New("invalid or expired reset token")
	ErrResetNotConfigured = errors.New("password reset is not configured")
)

type UserStore interface {
	Create(ctx context.Context, u *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id int64) (*models.User, error)
	FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (*models.User, error)
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
}

// ResetStore stores single-use password reset tokens.
type ResetStore interface {
	Create(ctx context.Context, reset *models.PasswordReset) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordReset, error)
	DeleteForUser(ctx context.Context, userID int64) error
	MarkUsed(ctx context.Context, id int64) error
}

// OrgStore is the slice of the organization store Signup needs to resolve an
// existing organization slug and record a pending join request.
type OrgStore interface {
	FindBySlug(ctx context.Context, slug string) (*models.Organization, error)
	AddJoinRequest(ctx context.Context, request *models.JoinRequest) error
}

const (
	defaultResetTTL = 24 * time.Hour
	defaultBaseURL  = "http://localhost:8080"
)

type Service struct {
	repo       UserStore
	orgRepo    OrgStore
	resets     ResetStore
	mailer     mailer.Mailer
	jwt        *JWTService
	bcryptCost int
	resetTTL   time.Duration
	baseURL    string
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithBcryptCost overrides the bcrypt work factor used when hashing passwords.
// The default is bcrypt.DefaultCost; production deployments should raise it
// via BCRYPT_COST.
func WithBcryptCost(cost int) ServiceOption {
	return func(s *Service) { s.bcryptCost = cost }
}

// WithOrgStore wires an organization store so signups can request to join an
// existing organization by slug.
func WithOrgStore(orgRepo OrgStore) ServiceOption {
	return func(s *Service) { s.orgRepo = orgRepo }
}

// WithResetStore wires the store backing password reset tokens. Password
// reset endpoints return ErrResetNotConfigured when no store is wired.
func WithResetStore(resets ResetStore) ServiceOption {
	return func(s *Service) { s.resets = resets }
}

// WithMailer overrides the mailer used to deliver password reset emails. The
// default logs the reset link instead of sending it.
func WithMailer(m mailer.Mailer) ServiceOption {
	return func(s *Service) { s.mailer = m }
}

// WithPasswordResetTTL sets how long reset tokens stay valid.
func WithPasswordResetTTL(ttl time.Duration) ServiceOption {
	return func(s *Service) { s.resetTTL = ttl }
}

// WithBaseURL sets the public URL used to build password reset links.
func WithBaseURL(baseURL string) ServiceOption {
	return func(s *Service) { s.baseURL = baseURL }
}

func NewService(repo UserStore, jwt *JWTService, opts ...ServiceOption) *Service {
	s := &Service{
		repo:       repo,
		jwt:        jwt,
		bcryptCost: bcrypt.DefaultCost,
		mailer:     mailer.New(mailer.Config{}),
		resetTTL:   defaultResetTTL,
		baseURL:    defaultBaseURL,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) Signup(ctx context.Context, req models.SignupRequest) (*models.AuthResponse, error) {
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	// Resolve the organization up front so a bad slug fails before the user
	// is created.
	var org *models.Organization
	if req.OrgSlug != "" {
		if s.orgRepo == nil {
			return nil, fmt.Errorf("joining an organization is not supported")
		}
		org, err = s.orgRepo.FindBySlug(ctx, req.OrgSlug)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return nil, ErrOrgNotFound
			}
			return nil, fmt.Errorf("find organization: %w", err)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	user := &models.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
		Role:         "user",
		APIKey:       HashAPIKey(apiKey),
		Organization: req.Organization,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	joinPending := false
	if org != nil {
		if err := s.orgRepo.AddJoinRequest(ctx, &models.JoinRequest{OrganizationID: org.ID, UserID: user.ID}); err != nil {
			return nil, fmt.Errorf("create join request: %w", err)
		}
		joinPending = true
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	// The raw key is returned exactly once, here at signup. Only its digest
	// exists anywhere else.
	return &models.AuthResponse{Token: token, APIKey: apiKey, User: *user, OrgJoinPending: joinPending}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrInvalidCreds
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCreds
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwt.ValidateToken(tokenString)
}

func (s *Service) Authenticate(ctx context.Context, tokenString string) (*Claims, error) {
	if strings.HasPrefix(tokenString, "iaas_") {
		user, err := s.repo.FindByAPIKeyHash(ctx, HashAPIKey(tokenString))
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return nil, ErrInvalidCreds
			}
			return nil, fmt.Errorf("find user by api key: %w", err)
		}
		return &Claims{
			UserID: user.ID,
			Email:  user.Email,
			Role:   user.Role,
		}, nil
	}
	return s.jwt.ValidateToken(tokenString)
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	hash := sha256.Sum256(bytes)
	return "iaas_" + hex.EncodeToString(hash[:16]), nil
}

// RequestPasswordReset mails a single-use reset link to the account's email
// address. It reports success even when the email has no account so callers
// cannot enumerate registered addresses. The link expires after resetTTL.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil // enumeration-safe: behave as if an email was sent
		}
		return fmt.Errorf("find user: %w", err)
	}
	if s.resets == nil {
		return ErrResetNotConfigured
	}

	// Invalidate any previously issued tokens for this user.
	if err := s.resets.DeleteForUser(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidate old reset tokens: %w", err)
	}

	token, err := generateResetToken()
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}

	reset := &models.PasswordReset{
		UserID:    user.ID,
		TokenHash: HashAPIKey(token),
		ExpiresAt: time.Now().UTC().Add(s.resetTTL),
	}
	if err := s.resets.Create(ctx, reset); err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}

	link := fmt.Sprintf("%s/?reset_token=%s", strings.TrimRight(s.baseURL, "/"), token)
	ttlHours := int(s.resetTTL.Hours())
	if err := s.mailer.SendEmail(user.Email, "Reset your IaaS Platform password", resetEmailHTML(link, ttlHours)); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	return nil
}

// ResetPassword sets a new password for the account bound to a valid, unused,
// unexpired reset token. The token is consumed so it cannot be replayed.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if s.resets == nil {
		return ErrResetNotConfigured
	}

	reset, err := s.resets.FindByTokenHash(ctx, HashAPIKey(token))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrInvalidResetToken
		}
		return fmt.Errorf("find reset token: %w", err)
	}
	if reset.UsedAt != nil || !reset.ExpiresAt.After(time.Now().UTC()) {
		return ErrInvalidResetToken
	}

	user, err := s.repo.FindByID(ctx, reset.UserID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.repo.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.resets.MarkUsed(ctx, reset.ID); err != nil {
		return fmt.Errorf("consume reset token: %w", err)
	}
	return nil
}

func generateResetToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return "pwr_" + hex.EncodeToString(bytes), nil
}

func resetEmailHTML(link string, ttlHours int) string {
	return fmt.Sprintf(`
<p>We received a request to reset your IaaS Platform password.</p>
<p><a href="%[1]s">Reset your password</a></p>
<p>If you didn't request this, you can safely ignore this email.</p>
<p>This link expires in %[2]d hours and can only be used once.</p>
`, link, ttlHours)
}

// HashAPIKey returns the SHA-256 hex digest of an API key. Only digests are
// stored in the database; the raw key is returned to the client exactly once,
// at signup.
func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}
