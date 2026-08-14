//go:build integration

package database

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ogc16/iaas-platform/internal/models"
)

// These tests exercise the real repositories and migrations against a live
// PostgreSQL. They are excluded from the default `go test ./...` run via the
// `integration` build tag. Run them with:
//
//	TEST_DATABASE_URL='postgres://iaas:iaas@127.0.0.1:5432/iaas?sslmode=disable' \
//		go test -tags integration -count=1 ./internal/database/
//
// CI runs them against a postgres:16-alpine service container.

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://iaas:iaas@127.0.0.1:5432/iaas?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := Connect(ctx, dsn, 4, 1)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func uniqueTag(prefix string) string {
	return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func TestIntegration_Migrate_IsIdempotentAndComplete(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)

	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if n, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	} else if n != 0 {
		t.Fatalf("expected 0 migrations on second run, got %d", n)
	}

	want, err := migrationFiles()
	if err != nil {
		t.Fatalf("migrationFiles: %v", err)
	}
	var got int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&got); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if got != len(want) {
		t.Fatalf("expected %d applied migrations, got %d", len(want), got)
	}
}

func TestIntegration_UserRepository_CreateFindUpdate(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewUserRepository(pool)
	email := uniqueTag("user") + "@example.com"
	u := &models.User{
		Email:        email,
		PasswordHash: "hash-v1",
		Name:         "Test User",
		Role:         "user",
		APIKey:       uniqueTag("apikey"),
		Organization: "acme",
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("expected a generated user id")
	}

	byEmail, err := repo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.ID != u.ID || byEmail.PasswordHash != u.PasswordHash {
		t.Fatalf("FindByEmail mismatch: %+v", byEmail)
	}

	byKey, err := repo.FindByAPIKeyHash(ctx, u.APIKey)
	if err != nil {
		t.Fatalf("FindByAPIKeyHash: %v", err)
	}
	if byKey.ID != u.ID {
		t.Fatalf("FindByAPIKeyHash mismatch: %+v", byKey)
	}

	if err := repo.UpdatePassword(ctx, u.ID, "hash-v2"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	after, err := repo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail after update: %v", err)
	}
	if after.PasswordHash != "hash-v2" {
		t.Fatalf("password not updated: %q", after.PasswordHash)
	}
}

func TestIntegration_OrgRepository_CreateAndMembers(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	orgRepo := NewOrgRepository(pool)
	userRepo := NewUserRepository(pool)

	org := &models.Organization{Name: uniqueTag("Org"), Slug: uniqueTag("slug")}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	admin := &models.User{Email: uniqueTag("admin") + "@example.com", PasswordHash: "h", Name: "Admin", Role: "user", APIKey: uniqueTag("ak-admin")}
	member := &models.User{Email: uniqueTag("member") + "@example.com", PasswordHash: "h", Name: "Member", Role: "user", APIKey: uniqueTag("ak-member")}
	if err := userRepo.Create(ctx, admin); err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	if err := userRepo.Create(ctx, member); err != nil {
		t.Fatalf("create member user: %v", err)
	}

	if err := orgRepo.AddMember(ctx, &models.OrgMember{OrganizationID: org.ID, UserID: admin.ID, Role: "admin"}); err != nil {
		t.Fatalf("add admin: %v", err)
	}
	if err := orgRepo.AddMember(ctx, &models.OrgMember{OrganizationID: org.ID, UserID: member.ID, Role: "member"}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	m, err := orgRepo.FindMember(ctx, org.ID, admin.ID)
	if err != nil {
		t.Fatalf("FindMember: %v", err)
	}
	if m.Role != "admin" {
		t.Fatalf("expected admin role, got %q", m.Role)
	}

	all, err := orgRepo.FindMembers(ctx, org.ID, 50, 0)
	if err != nil {
		t.Fatalf("FindMembers: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 members, got %d", len(all))
	}

	total, err := orgRepo.CountMembers(ctx, org.ID)
	if err != nil {
		t.Fatalf("CountMembers: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected count 2, got %d", total)
	}
}

func TestIntegration_PasswordResetRepository_Lifecycle(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userRepo := NewUserRepository(pool)
	resetRepo := NewPasswordResetRepository(pool)

	u := &models.User{Email: uniqueTag("reset") + "@example.com", PasswordHash: "h", Name: "Reset", Role: "user", APIKey: uniqueTag("ak-reset")}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenHash := uniqueTag("tok")
	reset := &models.PasswordReset{UserID: u.ID, TokenHash: tokenHash, ExpiresAt: time.Now().Add(time.Hour)}
	if err := resetRepo.Create(ctx, reset); err != nil {
		t.Fatalf("create reset: %v", err)
	}

	found, err := resetRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if found.ID != reset.ID {
		t.Fatalf("reset id mismatch")
	}
	if found.UsedAt != nil {
		t.Fatal("expected unused token")
	}

	if err := resetRepo.MarkUsed(ctx, reset.ID); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
	used, err := resetRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("FindByTokenHash after use: %v", err)
	}
	if used.UsedAt == nil {
		t.Fatal("expected used_at to be set")
	}

	if err := resetRepo.DeleteForUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteForUser: %v", err)
	}
	if _, err := resetRepo.FindByTokenHash(ctx, tokenHash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
