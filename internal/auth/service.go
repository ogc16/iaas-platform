package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/user/iaas-platform/internal/database"
	"github.com/user/iaas-platform/internal/models"
)

var (
	ErrEmailTaken    = errors.New("email already taken")
	ErrInvalidCreds  = errors.New("invalid email or password")
	ErrUserNotFound  = errors.New("user not found")
)

type Service struct {
	repo  *database.UserRepository
	jwt   *JWTService
}

func NewService(repo *database.UserRepository, jwt *JWTService) *Service {
	return &Service{repo: repo, jwt: jwt}
}

func (s *Service) Signup(ctx context.Context, req models.SignupRequest) (*models.AuthResponse, error) {
	existing, _ := s.repo.FindByEmail(ctx, req.Email)
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
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
		APIKey:       apiKey,
		Organization: req.Organization,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := s.jwt.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCreds
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
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.jwt.ValidateToken(tokenString)
}

func (s *Service) Authenticate(ctx context.Context, tokenString string) (*Claims, error) {
	if len(tokenString) > 5 && tokenString[:5] == "iaas_" {
		user, err := s.repo.FindByAPIKey(ctx, tokenString)
		if err != nil {
			return nil, ErrInvalidCreds
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
