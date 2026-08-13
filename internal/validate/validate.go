// Package validate implements a small, dependency-free struct validator driven
// by `validate` struct tags. It deliberately covers only the rules the
// platform's request models need.
//
// Supported rules (comma-separated in a single tag):
//
//	required   value must be non-zero (strings non-empty, numbers non-zero)
//	email      value must look like an email address
//	min=N      strings: minimum rune length; numbers: minimum value
//	max=N      strings: maximum rune length; numbers: maximum value
//	gte=N      numbers: value >= N
//	gt=N       numbers: value > N
//	lte=N      numbers: value <= N
//	oneof=a b c  value must be one of the space-separated tokens
//	omitempty  skip the remaining rules when the value is empty or zero
//
// The first violated rule is returned as an error naming the JSON field.
package validate

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Struct validates the exported fields of v against their `validate` tags.
// Pointers to structs are dereferenced; non-struct values pass without error.
func Struct(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		tag, ok := f.Tag.Lookup("validate")
		if !ok || tag == "" {
			continue
		}
		if err := validateField(f, rv.Field(i), tag); err != nil {
			return err
		}
	}
	return nil
}

func validateField(f reflect.StructField, v reflect.Value, tag string) error {
	name := f.Name
	if jn := f.Tag.Get("json"); jn != "" && jn != "-" {
		name = jn
	}

	for _, rule := range strings.Split(tag, ",") {
		if rule == "" {
			continue
		}
		if rule == "omitempty" {
			if isEmpty(v) {
				return nil
			}
			continue
		}

		switch {
		case rule == "required":
			if isEmpty(v) {
				return fmt.Errorf("field %q is required", name)
			}
		case rule == "email":
			if !isEmail(v) {
				return fmt.Errorf("field %q must be a valid email address", name)
			}
		case strings.HasPrefix(rule, "min="):
			n, err := parseRuleInt(rule, name)
			if err != nil {
				return err
			}
			if err := checkMin(v, name, n); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "max="):
			n, err := parseRuleInt(rule, name)
			if err != nil {
				return err
			}
			if err := checkMax(v, name, n); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "gte="):
			n, err := parseRuleInt(rule, name)
			if err != nil {
				return err
			}
			if err := checkNumericBound(v, name, "gte", n, true); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "gt="):
			n, err := parseRuleInt(rule, name)
			if err != nil {
				return err
			}
			if err := checkNumericBound(v, name, "gt", n, false); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "lte="):
			n, err := parseRuleInt(rule, name)
			if err != nil {
				return err
			}
			if err := checkNumericBound(v, name, "lte", n, true); err != nil {
				return err
			}
		case strings.HasPrefix(rule, "oneof="):
			tokens := strings.Fields(strings.TrimPrefix(rule, "oneof="))
			if !contains(tokens, v.String()) {
				return fmt.Errorf("field %q must be one of: %s", name, strings.Join(tokens, ", "))
			}
		default:
			return fmt.Errorf("unsupported validate rule %q on field %q", rule, name)
		}
	}
	return nil
}

func parseRuleInt(rule, field string) (int, error) {
	parts := strings.SplitN(rule, "=", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid validate rule %q on field %q", rule, field)
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid validate rule %q on field %q", rule, field)
	}
	return n, nil
}

func checkMin(v reflect.Value, name string, n int) error {
	switch v.Kind() {
	case reflect.String:
		if utf8.RuneCountInString(v.String()) < n {
			return fmt.Errorf("field %q must be at least %d characters", name, n)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() < int64(n) {
			return fmt.Errorf("field %q must be at least %d", name, n)
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() < float64(n) {
			return fmt.Errorf("field %q must be at least %d", name, n)
		}
	}
	return nil
}

func checkMax(v reflect.Value, name string, n int) error {
	switch v.Kind() {
	case reflect.String:
		if utf8.RuneCountInString(v.String()) > n {
			return fmt.Errorf("field %q must be at most %d characters", name, n)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Int() > int64(n) {
			return fmt.Errorf("field %q must be at most %d", name, n)
		}
	case reflect.Float32, reflect.Float64:
		if v.Float() > float64(n) {
			return fmt.Errorf("field %q must be at most %d", name, n)
		}
	}
	return nil
}

func checkNumericBound(v reflect.Value, name, op string, n int, inclusive bool) error {
	var val float64
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val = float64(v.Int())
	case reflect.Float32, reflect.Float64:
		val = v.Float()
	default:
		return nil
	}

	target := float64(n)
	ok := val >= target
	verb := "greater than or equal to"
	if op == "lte" {
		ok = val <= target
		verb = "less than or equal to"
	} else if !inclusive {
		ok = val > target
		verb = "greater than"
	}
	if !ok {
		return fmt.Errorf("field %q must be %s %d", name, verb, n)
	}
	return nil
}

func isEmail(v reflect.Value) bool {
	if v.Kind() != reflect.String {
		return false
	}
	return emailRe.MatchString(v.String())
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Array:
		return v.IsNil()
	default:
		return false
	}
}

func contains(tokens []string, s string) bool {
	for _, t := range tokens {
		if t == s {
			return true
		}
	}
	return false
}
