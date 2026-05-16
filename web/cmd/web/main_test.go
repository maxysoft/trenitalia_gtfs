package main

import (
	"os"
	"testing"
)

func TestBuildWhere_Empty(t *testing.T) {
	sql, args := buildWhere("", "")
	if sql != "" {
		t.Errorf("expected empty SQL clause, got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}

func TestBuildWhere_LineOnly(t *testing.T) {
	sql, args := buildWhere("", "FL3")
	if sql != "WHERE line_code = ?" {
		t.Errorf("unexpected SQL: %q", sql)
	}
	if len(args) != 1 || args[0] != "FL3" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestBuildWhere_QueryOnly(t *testing.T) {
	sql, args := buildWhere("roma", "")
	if sql == "" {
		t.Error("expected non-empty SQL for text query")
	}
	// 4 LIKE placeholders for train_number, station_name, service_date, status
	if len(args) != 4 {
		t.Errorf("expected 4 args for query-only, got %d: %v", len(args), args)
	}
	for i, a := range args {
		s, ok := a.(string)
		if !ok {
			t.Errorf("arg[%d] is not a string: %T", i, a)
			continue
		}
		if s != "%roma%" {
			t.Errorf("arg[%d] = %q, want %%roma%%", i, s)
		}
	}
}

func TestBuildWhere_Both(t *testing.T) {
	sql, args := buildWhere("test", "FL3")
	if sql == "" {
		t.Error("expected non-empty SQL with both filters")
	}
	// 1 for line + 4 for LIKE search
	if len(args) != 5 {
		t.Errorf("expected 5 args, got %d: %v", len(args), args)
	}
	if args[0] != "FL3" {
		t.Errorf("expected first arg=FL3, got %v", args[0])
	}
}

func TestEscapeLike(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"50%", `50\%`},
		{"_test", `\_test`},
		{`back\slash`, `back\\slash`},
		{"FL3_Roma%", `FL3\_Roma\%`},
		{"", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := escapeLike(tc.input)
			if got != tc.expected {
				t.Errorf("escapeLike(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestGetenv_DefaultWhenMissing(t *testing.T) {
	const key = "TEST_GETENV_MISSING_KEY"
	os.Unsetenv(key)
	if got := getenv(key, "default"); got != "default" {
		t.Errorf("expected default, got %q", got)
	}
}

func TestGetenv_ReturnsValue(t *testing.T) {
	const key = "TEST_GETENV_SET_KEY"
	t.Setenv(key, "custom")
	if got := getenv(key, "default"); got != "custom" {
		t.Errorf("expected custom, got %q", got)
	}
}

func TestGetenv_TrimsSpaces(t *testing.T) {
	const key = "TEST_GETENV_SPACES_KEY"
	t.Setenv(key, "  trimmed  ")
	if got := getenv(key, "default"); got != "trimmed" {
		t.Errorf("expected trimmed, got %q", got)
	}
}

func TestGetenv_EmptyFallsBack(t *testing.T) {
	const key = "TEST_GETENV_EMPTY_KEY"
	t.Setenv(key, "   ")
	if got := getenv(key, "fallback"); got != "fallback" {
		t.Errorf("expected fallback for whitespace-only value, got %q", got)
	}
}

func TestAtoiDefault(t *testing.T) {
	cases := []struct {
		input    string
		fallback int
		expected int
	}{
		{"42", 0, 42},
		{"0", 99, 0},
		{"-1", 0, -1},
		{"", 99, 99},
		{"invalid", 5, 5},
		{"  10  ", 0, 10},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			got := atoiDefault(tc.input, tc.fallback)
			if got != tc.expected {
				t.Errorf("atoiDefault(%q, %d) = %d, want %d", tc.input, tc.fallback, got, tc.expected)
			}
		})
	}
}
