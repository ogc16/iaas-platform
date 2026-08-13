package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ogc16/iaas-platform/internal/database"
	"github.com/ogc16/iaas-platform/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	usersByEmail map[string]*models.User
	usersByID    map[int64]*models.User
	usersByKey   map[string]*models.User
	nextID       int64
	findErr      error
	createErr    error
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		usersByEmail: map[string]*models.User{},
		usersByID:    map[int64]*models.User{},
		usersByKey:   map[string]*models.User{},
	}
}

func (f *fakeUserStore) Create(ctx context.Context, u *models.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	u.ID = f.nextID
	f.usersByID[u.ID] = u
	f.usersByEmail[u.Email] = u
	f.usersByKey[u.APIKey] = u
	return nil
}

func (f *fakeUserStore) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if u, ok := f.usersByEmail[email]; ok {
		return u, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeUserStore) FindByID(ctx context.Context, id int64) (*models.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if u, ok := f.usersByID[id]; ok {
		return u, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeUserStore) FindByAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if u, ok := f.usersByKey[apiKey]; ok {
		return u, nil
	}
	return nil, database.ErrNotFound
}

func newTestService() (*Service, *fakeUserStore, *JWTService) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	return NewService(store, jwt), store, jwt
}

func seedUser(t *testing.T, store *fakeUserStore, email, password string) *models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u := &models.User{
		Email:        email,
		PasswordHash: string(hash),
		Name:         "Test User",
		Role:         "user",
		APIKey:       "iaas_seeded",
	}
	store.nextID++
	u.ID = store.nextID
	store.usersByID[u.ID] = u
	store.usersByEmail[u.Email] = u
	store.usersByKey[u.APIKey] = u
	return u
}

func TestService_Signup_Success(t *testing.T) {
	svc, store, _ := newTestService()

	resp, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:        "alice@example.com",
		Password:     "hunter2",
		Name:         "Alice",
		Organization: "acme",
	})
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}

	if resp.Token == "" {
		t.Fatal("expected a token to be returned")
	}
	if resp.User.ID == 0 {
		t.Fatal("expected user ID to be assigned")
	}
	if resp.User.PasswordHash == "hunter2" || resp.User.PasswordHash == "" {
		t.Fatal("password must be stored hashed")
	}
	if !strings.HasPrefix(resp.User.APIKey, "iaas_") {
		t.Fatalf("expected API key, got %q", resp.User.APIKey)
	}
	if _, ok := store.usersByEmail["alice@example.com"]; !ok {
		t.Fatal("expected user to be persisted")
	}
}

func TestService_Signup_DuplicateEmail(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	_, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
		Name:     "Alice",
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestService_Signup_PropagatesDBError(t *testing.T) {
	svc, store, _ := newTestService()
	dbErr := errors.New("connection refused")
	store.findErr = dbErr

	_, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
		Name:     "Alice",
	})
	if err == nil || errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Login_Success(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	resp, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected a token to be returned")
	}
	if resp.User.Email != "alice@example.com" {
		t.Fatalf("unexpected user email: %q", resp.User.Email)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "alice@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestService_Login_UnknownUser(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "ghost@example.com",
		Password: "whatever",
	})
	if !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestService_Login_PropagatesDBError(t *testing.T) {
	svc, store, _ := newTestService()
	dbErr := errors.New("connection refused")
	store.findErr = dbErr

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
	})
	if err == nil || errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_GetUserByID_NotFound(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.GetUserByID(context.Background(), 999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestService_GetUserByID_PropagatesDBError(t *testing.T) {
	svc, store, _ := newTestService()
	dbErr := errors.New("connection refused")
	store.findErr = dbErr

	_, err := svc.GetUserByID(context.Background(), 1)
	if err == nil || errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Authenticate_APIKey(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	claims, err := svc.Authenticate(context.Background(), "iaas_seeded")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if claims.Email != "alice@example.com" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestService_Authenticate_UnknownAPIKey(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Authenticate(context.Background(), "iaas_nonexistent")
	if !errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestService_Authenticate_APIKeyDBError(t *testing.T) {
	svc, store, _ := newTestService()
	dbErr := errors.New("connection refused")
	store.findErr = dbErr

	_, err := svc.Authenticate(context.Background(), "iaas_seeded")
	if err == nil || errors.Is(err, ErrInvalidCreds) {
		t.Fatalf("expected a real error to propagate, got %v", err)
	}
}

func TestService_Authenticate_JWT(t *testing.T) {
	svc, _, jwt := newTestService()

	token, err := jwt.GenerateToken(7, "bob@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if claims.UserID != 7 || claims.Email != "bob@example.com" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestService_Authenticate_InvalidJWT(t *testing.T) {
	svc, _, _ := newTestService()

	if _, err := svc.Authenticate(context.Background(), "garbage-token"); err == nil {
		t.Fatal("expected invalid JWT to be rejected")
	}
}
