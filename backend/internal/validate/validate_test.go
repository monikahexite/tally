package validate

import (
	"strings"
	"testing"
)

func TestPollInput(t *testing.T) {
	tests := []struct {
		name      string
		question  string
		options   []string
		duration  int
		wantField string // "" = valid
	}{
		{"valid", "  Best   language? ", []string{"Go", " Rust "}, 60, ""},
		{"short question", "Hi", []string{"a", "b"}, 0, "question"},
		{"one option", "Pick one", []string{"only"}, 0, "options"},
		{"too many options", "Pick one", strings.Split("a,b,c,d,e,f,g,h,i,j,k", ","), 0, "options"},
		{"empty option", "Pick one", []string{"a", "   "}, 0, "options"},
		{"duplicate case-insensitive", "Pick one", []string{"Yes", "yes"}, 0, "options"},
		{"control chars", "Pick\x00one", []string{"a", "b"}, 0, "question"},
		{"negative duration", "Pick one", []string{"a", "b"}, -5, "durationMinutes"},
		{"too long duration", "Pick one", []string{"a", "b"}, MaxDurationMins + 1, "durationMinutes"},
		{"long option", "Pick one", []string{"a", strings.Repeat("x", 101)}, 0, "options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errs := PollInput(tt.question, "", tt.options, tt.duration)
			if tt.wantField == "" {
				if errs.Any() {
					t.Fatalf("expected valid, got %v", errs)
				}
				if out.Question != "Best language?" || out.Options[1] != "Rust" {
					t.Fatalf("input not normalised: %+v", out)
				}
				return
			}
			if _, ok := errs[tt.wantField]; !ok {
				t.Fatalf("expected error on %q, got %v", tt.wantField, errs)
			}
		})
	}
}

func TestSignupInput(t *testing.T) {
	if _, errs := SignupInput("Ann", " Ann@Example.com ", "secret123"); errs.Any() {
		t.Fatalf("expected valid, got %v", errs)
	}
	cases := map[string][3]string{
		"email":    {"Ann", "not-an-email", "secret123"},
		"password": {"Ann", "ann@example.com", "lettersonly"},
		"name":     {"A", "ann@example.com", "secret123"},
	}
	for field, in := range cases {
		if _, errs := SignupInput(in[0], in[1], in[2]); errs[field] == "" {
			t.Errorf("expected %s error, got %v", field, errs)
		}
	}
}

func TestIsSlug(t *testing.T) {
	for s, want := range map[string]bool{"abcd2345": true, "ab": false, "ABCD1234": false, "../etc": false, "abc$1234": false} {
		if IsSlug(s) != want {
			t.Errorf("IsSlug(%q) = %v", s, !want)
		}
	}
}
