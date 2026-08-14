package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func (f *fakeUserStore) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	if u, ok := f.usersByID[id]; ok {
		u.PasswordHash = passwordHash
		return nil
	}
	return database.ErrNotFound
}

func newTestService() (*Service, *fakeUserStore, *JWTService) {
	store := newFakeUserStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	return NewService(store, jwt), store, jwt
}

type fakeOrgStore struct {
	bySlug        map[string]*models.Organization
	requests      []*models.JoinRequest
	findSlugErr   error
	addRequestErr error
}

func newFakeOrgStore() *fakeOrgStore {
	return &fakeOrgStore{bySlug: map[string]*models.Organization{}}
}

func (f *fakeOrgStore) FindBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	if f.findSlugErr != nil {
		return nil, f.findSlugErr
	}
	if org, ok := f.bySlug[slug]; ok {
		return org, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeOrgStore) AddJoinRequest(ctx context.Context, request *models.JoinRequest) error {
	if f.addRequestErr != nil {
		return f.addRequestErr
	}
	f.requests = append(f.requests, request)
	return nil
}

type fakeResetStore struct {
	byHash map[string]*models.PasswordReset
	nextID int64
	err    error
}

func newFakeResetStore() *fakeResetStore {
	return &fakeResetStore{byHash: map[string]*models.PasswordReset{}}
}

func (f *fakeResetStore) Create(ctx context.Context, reset *models.PasswordReset) error {
	if f.err != nil {
		return f.err
	}
	f.nextID++
	reset.ID = f.nextID
	f.byHash[reset.TokenHash] = reset
	return nil
}

func (f *fakeResetStore) FindByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordReset, error) {
	if f.err != nil {
		return nil, f.err
	}
	if reset, ok := f.byHash[tokenHash]; ok {
		return reset, nil
	}
	return nil, database.ErrNotFound
}

func (f *fakeResetStore) DeleteForUser(ctx context.Context, userID int64) error {
	for hash, reset := range f.byHash {
		if reset.UserID == userID {
			delete(f.byHash, hash)
		}
	}
	return nil
}

func (f *fakeResetStore) MarkUsed(ctx context.Context, id int64) error {
	for _, reset := range f.byHash {
		if reset.ID == id {
			now := time.Now().UTC()
			reset.UsedAt = &now
		}
	}
	return nil
}

type fakeMailer struct {
	emails []emailMessage
	err    error
}

type emailMessage struct {
	to      string
	subject string
	body    string
}

func (m *fakeMailer) SendEmail(to, subject, htmlBody string) error {
	if m.err != nil {
		return m.err
	}
	m.emails = append(m.emails, emailMessage{to: to, subject: subject, body: htmlBody})
	return nil
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

func TestService_Signup_JoinRequestCreated(t *testing.T) {
	store := newFakeUserStore()
	orgs := newFakeOrgStore()
	orgs.bySlug["acme"] = &models.Organization{ID: 1, Name: "Acme", Slug: "acme"}
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt, WithOrgStore(orgs))

	resp, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "bob@example.com",
		Password: "hunter2",
		Name:     "Bob",
		OrgSlug:  "acme",
	})
	if err != nil {
		t.Fatalf("Signup: %v", err)
	}
	if !resp.OrgJoinPending {
		t.Fatal("expected OrgJoinPending to be true")
	}
	if len(orgs.requests) != 1 {
		t.Fatalf("expected 1 join request, got %d", len(orgs.requests))
	}
	if orgs.requests[0].OrganizationID != 1 || orgs.requests[0].UserID != resp.User.ID {
		t.Fatalf("unexpected join request: %+v", orgs.requests[0])
	}
}

func TestService_Signup_JoinRequestUnknownOrg(t *testing.T) {
	store := newFakeUserStore()
	orgs := newFakeOrgStore()
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt, WithOrgStore(orgs))

	_, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "bob@example.com",
		Password: "hunter2",
		Name:     "Bob",
		OrgSlug:  "nope",
	})
	if !errors.Is(err, ErrOrgNotFound) {
		t.Fatalf("expected ErrOrgNotFound, got %v", err)
	}
	if _, ok := store.usersByEmail["bob@example.com"]; ok {
		t.Fatal("user must not be created when the organization does not exist")
	}
}

func TestService_Signup_JoinRequestWithoutOrgStore(t *testing.T) {
	svc, _, _ := newTestService()

	_, err := svc.Signup(context.Background(), models.SignupRequest{
		Email:    "bob@example.com",
		Password: "hunter2",
		Name:     "Bob",
		OrgSlug:  "acme",
	})
	if err == nil || err.Error() != "joining an organization is not supported" {
		t.Fatalf("expected unsupported error, got %v", err)
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

func newResetTestService() (*Service, *fakeUserStore, *fakeResetStore, *fakeMailer) {
	store := newFakeUserStore()
	resets := newFakeResetStore()
	mail := &fakeMailer{}
	jwt := NewJWTService("test-secret", "test-issuer", 3600)
	svc := NewService(store, jwt,
		WithResetStore(resets),
		WithMailer(mail),
		WithBaseURL("https://iaas.example.com"),
		WithPasswordResetTTL(2*time.Hour),
	)
	return svc, store, resets, mail
}

func TestService_RequestPasswordReset_SendsEmail(t *testing.T) {
	svc, store, resets, mail := newResetTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	if err := svc.RequestPasswordReset(context.Background(), "alice@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}

	if len(mail.emails) != 1 {
		t.Fatalf("expected 1 email, got %d", len(mail.emails))
	}
	if mail.emails[0].to != "alice@example.com" {
		t.Fatalf("unexpected recipient: %q", mail.emails[0].to)
	}
	if !strings.Contains(mail.emails[0].body, "/?reset_token=pwr_") {
		t.Fatalf("email must contain reset link, got %q", mail.emails[0].body)
	}
	if len(resets.byHash) != 1 {
		t.Fatalf("expected 1 stored reset token, got %d", len(resets.byHash))
	}
	for hash := range resets.byHash {
		if strings.Contains(hash, "pwr_") {
			t.Fatal("reset token must be stored as a hash, not plaintext")
		}
	}
}

func TestService_RequestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	svc, _, resets, mail := newResetTestService()

	if err := svc.RequestPasswordReset(context.Background(), "ghost@example.com"); err != nil {
		t.Fatalf("expected no error for unknown email, got %v", err)
	}
	if len(mail.emails) != 0 {
		t.Fatalf("expected no email for unknown account, got %d", len(mail.emails))
	}
	if len(resets.byHash) != 0 {
		t.Fatalf("expected no tokens for unknown account, got %d", len(resets.byHash))
	}
}

func TestService_RequestPasswordReset_InvalidatesOldTokens(t *testing.T) {
	svc, store, resets, mail := newResetTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	if err := svc.RequestPasswordReset(context.Background(), "alice@example.com"); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if err := svc.RequestPasswordReset(context.Background(), "alice@example.com"); err != nil {
		t.Fatalf("second request: %v", err)
	}

	if len(resets.byHash) != 1 {
		t.Fatalf("expected only the newest token to remain, got %d", len(resets.byHash))
	}
	if len(mail.emails) != 2 {
		t.Fatalf("expected 2 emails, got %d", len(mail.emails))
	}
}

func TestService_RequestPasswordReset_NotConfigured(t *testing.T) {
	svc, store, _ := newTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	err := svc.RequestPasswordReset(context.Background(), "alice@example.com")
	if !errors.Is(err, ErrResetNotConfigured) {
		t.Fatalf("expected ErrResetNotConfigured, got %v", err)
	}
}

func TestService_ResetPassword_Success(t *testing.T) {
	svc, store, resets, mail := newResetTestService()
	user := seedUser(t, store, "alice@example.com", "hunter2")
	if err := svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}

	token := resetTokenFromEmail(mail)
	if err := svc.ResetPassword(context.Background(), token, "brand-new-password"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(store.usersByID[user.ID].PasswordHash), []byte("brand-new-password")); err != nil {
		t.Fatalf("password was not updated: %v", err)
	}
	reset := resets.byHash[HashAPIKey(token)]
	if reset.UsedAt == nil {
		t.Fatal("expected the reset token to be consumed")
	}
}

func TestService_ResetPassword_WrongToken(t *testing.T) {
	svc, store, _, _ := newResetTestService()
	seedUser(t, store, "alice@example.com", "hunter2")

	err := svc.ResetPassword(context.Background(), "pwr_"+strings.Repeat("a", 64), "brand-new-password")
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected ErrInvalidResetToken, got %v", err)
	}
}

func TestService_ResetPassword_ExpiredToken(t *testing.T) {
	svc, store, resets, mail := newResetTestService()
	user := seedUser(t, store, "alice@example.com", "hunter2")
	if err := svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}

	token := resetTokenFromEmail(mail)
	resets.byHash[HashAPIKey(token)].ExpiresAt = time.Now().UTC().Add(-time.Hour)

	if err := svc.ResetPassword(context.Background(), token, "brand-new-password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected ErrInvalidResetToken for expired token, got %v", err)
	}
}

func TestService_ResetPassword_UsedToken(t *testing.T) {
	svc, store, _, mail := newResetTestService()
	user := seedUser(t, store, "alice@example.com", "hunter2")
	if err := svc.RequestPasswordReset(context.Background(), user.Email); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}

	token := resetTokenFromEmail(mail)
	if err := svc.ResetPassword(context.Background(), token, "brand-new-password"); err != nil {
		t.Fatalf("first reset: %v", err)
	}

	if err := svc.ResetPassword(context.Background(), token, "another-password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected replay to be rejected, got %v", err)
	}
}

func TestService_ResetPassword_NotConfigured(t *testing.T) {
	svc, _, _ := newTestService()

	err := svc.ResetPassword(context.Background(), "pwr_"+strings.Repeat("a", 64), "brand-new-password")
	if !errors.Is(err, ErrResetNotConfigured) {
		t.Fatalf("expected ErrResetNotConfigured, got %v", err)
	}
}

// resetTokenFromEmail extracts the raw reset token from the link embedded in a
// captured reset email.
func resetTokenFromEmail(mail *fakeMailer) string {
	if len(mail.emails) != 1 {
		panic("expected exactly one email")
	}
	body := mail.emails[0].body
	idx := strings.Index(body, "reset_token=")
	if idx < 0 {
		panic("no reset link in email body")
	}
	token := body[idx+len("reset_token="):]
	if i := strings.IndexAny(token, "\"\n\r<"); i >= 0 {
		token = token[:i]
	}
	return strings.TrimSpace(token)
}
