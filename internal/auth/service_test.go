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
	usersByHash  map[string]*models.User
	nextID       int64
	findErr      error
	createErr    error
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		usersByEmail: map[string]*models.User{},
		usersByID:    map[int64]*models.User{},
		usersByHash:  map[string]*models.User{},
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
	f.usersByHash[u.APIKey] = u
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

func (f *fakeUserStore) FindByAPIKeyHash(ctx context.Context, apiKeyHash string) (*models.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if u, ok := f.usersByHash[apiKeyHash]; ok {
		return u, nil
	}
	return nil, database.ErrNotFound
}

type fakeOrgCreator struct {
	created []models.CreateOrgRequest
	userIDs []int64
	err     error
}

func (f *fakeOrgCreator) Create(ctx context.Context, userID int64, req models.CreateOrgRequest) (*models.Organization, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = append(f.created, req)
	f.userIDs = append(f.userIDs, userID)
	return &models.Organization{ID: int64(len(f.created)), Name: req.Name, Slug: req.Slug}, nil
}

func newTestService() (*Service, *fakeUserStore, *JWTService) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	return NewService(store, jwt, WithOrganizationCreator(&fakeOrgCreator{})), store, jwt
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
		APIKey:       HashAPIKey("iaas_seeded"),
	}
	store.nextID++
	u.ID = store.nextID
	store.usersByID[u.ID] = u
	store.usersByEmail[u.Email] = u
	store.usersByHash[u.APIKey] = u
	return u
}

func TestService_Signup_Success(t *testing.T) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	orgs := &fakeOrgCreator{}
	svc := NewService(store, jwt, WithOrganizationCreator(orgs))

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
	if !strings.HasPrefix(resp.APIKey, "iaas_") {
		t.Fatalf("expected raw API key in response, got %q", resp.APIKey)
	}
	stored := store.usersByEmail["alice@example.com"]
	if stored.APIKey == resp.APIKey {
		t.Fatal("API key must not be stored in plaintext")
	}
	if stored.APIKey != HashAPIKey(resp.APIKey) {
		t.Fatalf("expected stored API key to be the SHA-256 digest, got %q", stored.APIKey)
	}
	if _, ok := store.usersByEmail["alice@example.com"]; !ok {
		t.Fatal("expected user to be persisted")
	}
	if len(orgs.created) != 1 {
		t.Fatalf("expected one organization to be created, got %d", len(orgs.created))
	}
	if orgs.created[0].Name != "acme" {
		t.Fatalf("unexpected org name: %q", orgs.created[0].Name)
	}
	if orgs.userIDs[0] != resp.User.ID {
		t.Fatalf("org membership must belong to signup user, got userID %d", orgs.userIDs[0])
	}
	wantSlug := signupOrgSlug("acme", resp.User.ID)
	if orgs.created[0].Slug != wantSlug {
		t.Fatalf("unexpected org slug: got %q want %q", orgs.created[0].Slug, wantSlug)
	}
}

func TestService_Signup_SkipsOrgWhenEmpty(t *testing.T) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	orgs := &fakeOrgCreator{}
	svc := NewService(store, jwt, WithOrganizationCreator(orgs))

	if _, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
		Name:     "Alice",
	}); err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if len(orgs.created) != 0 {
		t.Fatalf("expected no organization when signup omits one, got %d", len(orgs.created))
	}
}

func TestService_Signup_RequiresOrgCreator(t *testing.T) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt) // no OrganizationCreator

	_, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:        "alice@example.com",
		Password:     "hunter2",
		Name:         "Alice",
		Organization: "acme",
	})
	if err == nil {
		t.Fatal("expected signup with org name to fail without OrganizationCreator")
	}
}

func TestSignupOrgSlug(t *testing.T) {
	got := signupOrgSlug("Acme Corp!", 42)
	if got != "acme-corp-42" {
		t.Fatalf("got %q", got)
	}
	got = signupOrgSlug("!!!", 7)
	if got != "org-7" {
		t.Fatalf("got %q", got)
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

func TestService_Signup_UsesConfiguredBcryptCost(t *testing.T) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt, WithBcryptCost(13))

	if _, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
		Name:     "Alice",
	}); err != nil {
		t.Fatalf("Signup: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(store.usersByEmail["alice@example.com"].PasswordHash))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != 13 {
		t.Fatalf("expected password hashed with cost 13, got %d", cost)
	}
}

func TestService_Login_DoesNotExposeAPIKey(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	resp, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    "alice@example.com",
		Password: "hunter2",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.APIKey != "" {
		t.Fatalf("raw API key must only be returned at signup, got %q", resp.APIKey)
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
