package validate

import (
	"strings"
	"testing"
)

type sampleStruct struct {
	Email    string  `json:"email" validate:"required,email"`
	Password string  `json:"password" validate:"required,min=8"`
	Role     string  `json:"role" validate:"omitempty,oneof=admin member"`
	DiskGB   int     `json:"disk_gb" validate:"required,gte=1"`
	Port     int     `json:"port" validate:"lte=65535"`
	Qty      float64 `json:"quantity" validate:"gt=0"`
}

func TestStruct_Valid(t *testing.T) {
	s := sampleStruct{
		Email:    "dev@example.com",
		Password: "hunter2hunter2",
		Role:     "",
		DiskGB:   10,
		Port:     8080,
		Qty:      2.5,
	}
	if err := Struct(&s); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestStruct_Required(t *testing.T) {
	s := sampleStruct{Password: "hunter2hunter2", DiskGB: 10, Qty: 1}
	err := Struct(&s)
	if err == nil || !strings.Contains(err.Error(), `field "email"`) {
		t.Fatalf("expected email required error, got %v", err)
	}
}

func TestStruct_Email(t *testing.T) {
	s := sampleStruct{Email: "not-an-email", Password: "hunter2hunter2", DiskGB: 10, Qty: 1}
	err := Struct(&s)
	if err == nil || !strings.Contains(err.Error(), "valid email") {
		t.Fatalf("expected email format error, got %v", err)
	}
}

func TestStruct_MinLength(t *testing.T) {
	s := sampleStruct{Email: "dev@example.com", Password: "short", DiskGB: 10, Qty: 1}
	err := Struct(&s)
	if err == nil || !strings.Contains(err.Error(), "at least 8") {
		t.Fatalf("expected password length error, got %v", err)
	}
}

func TestStruct_Oneof(t *testing.T) {
	s := sampleStruct{Email: "dev@example.com", Password: "hunter2hunter2", Role: "owner", DiskGB: 10, Qty: 1}
	err := Struct(&s)
	if err == nil || !strings.Contains(err.Error(), "one of") {
		t.Fatalf("expected oneof error, got %v", err)
	}
}

func TestStruct_NumericBounds(t *testing.T) {
	cases := []sampleStruct{
		{Email: "dev@example.com", Password: "hunter2hunter2", DiskGB: 0, Qty: 1},
		{Email: "dev@example.com", Password: "hunter2hunter2", DiskGB: 10, Port: 70000},
		{Email: "dev@example.com", Password: "hunter2hunter2", DiskGB: 10, Qty: 0},
	}
	for _, s := range cases {
		if err := Struct(&s); err == nil {
			t.Fatalf("expected error for %+v", s)
		}
	}
}

func TestStruct_NestedUnsupportedTag(t *testing.T) {
	type bad struct {
		Name string `json:"name" validate:"no-such-rule"`
	}
	if err := Struct(bad{Name: "x"}); err == nil {
		t.Fatal("expected unsupported rule error")
	}
}
