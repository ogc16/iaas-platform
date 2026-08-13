package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
)

var (
	ErrEmailTaken   = errors.New("email already taken")
	ErrInvalidCreds = errors.New("invalid email or password")
	ErrUserNotFound = errors.New("user not found")
)

type UserStore interface {
	Create(ctx context.Context, u *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id int64) (*models.User, error)
	FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (*models.User, error)
}

// OrganizationCreator creates an organization and membership for a user.
// Used at signup so the org name collected there appears in the dashboard.
type OrganizationCreator interface {
	Create(ctx context.Context, userID int64, req models.CreateOrgRequest) (*models.Organization, error)
}

type Service struct {
	repo       UserStore
	jwt        *JWTService
	orgs       OrganizationCreator
	bcryptCost int
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithBcryptCost overrides the bcrypt work factor used when hashing passwords.
// The default is bcrypt.DefaultCost; production deployments should raise it
// via BCRYPT_COST.
func WithBcryptCost(cost int) ServiceOption {
	return func(s *Service) { s.bcryptCost = cost }
}

// WithOrganizationCreator wires the org bootstrap used when signup includes
// an organization name.
func WithOrganizationCreator(c OrganizationCreator) ServiceOption {
	return func(s *Service) { s.orgs = c }
}

func NewService(repo UserStore, jwt *JWTService, opts ...ServiceOption) *Service {
	s := &Service{repo: repo, jwt: jwt, bcryptCost: bcrypt.DefaultCost}
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

	// Persist a real organization + admin membership when the signup form
	// collected an org name. users.organization alone is display metadata;
	// the dashboard lists orgs via organization_members.
	if name := strings.TrimSpace(req.Organization); name != "" {
		if s.orgs == nil {
			return nil, fmt.Errorf("organization creator not configured")
		}
		slug := signupOrgSlug(name, user.ID)
		if _, err := s.orgs.Create(ctx, user.ID, models.CreateOrgRequest{Name: name, Slug: slug}); err != nil {
			return nil, fmt.Errorf("create organization: %w", err)
		}
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	// The raw key is returned exactly once, here at signup. Only its digest
	// exists anywhere else.
	return &models.AuthResponse{Token: token, APIKey: apiKey, User: *user}, nil
}

// signupOrgSlug builds a unique slug from the org name and user id so two
// signups with the same company name do not collide.
func signupOrgSlug(name string, userID int64) string {
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return -1
		}
	}, slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	suffix := fmt.Sprintf("-%d", userID)
	const maxSlug = 32
	if len(slug)+len(suffix) > maxSlug {
		slug = slug[:maxSlug-len(suffix)]
		slug = strings.TrimRight(slug, "-")
	}
	return slug + suffix
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

// HashAPIKey returns the SHA-256 hex digest of an API key. Only digests are
// stored in the database; the raw key is returned to the client exactly once,
// at signup.
func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}
