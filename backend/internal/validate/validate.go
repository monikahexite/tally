// Package validate normalises and checks every piece of client input before it
// reaches the database. Handlers never use raw request structs directly.
package validate

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Errors maps a field name to a human readable problem.
type Errors map[string]string

func (e Errors) add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

func (e Errors) Any() bool { return len(e) > 0 }

const (
	MinOptions       = 2
	MaxOptions       = 10
	MaxDurationMins  = 30 * 24 * 60 // 30 days
	maxQuestionLen   = 200
	maxDescLen       = 500
	maxOptionLen     = 100
	minPasswordBytes = 8
	maxPasswordBytes = 72 // bcrypt ignores anything past 72 bytes
)

// ---------- auth ----------

type Signup struct {
	Name     string
	Email    string
	Password string
}

func SignupInput(name, email, password string) (Signup, Errors) {
	errs := Errors{}
	out := Signup{
		Name:     collapseSpaces(name),
		Email:    normaliseEmail(email),
		Password: password,
	}
	checkText(errs, "name", out.Name, 2, 50)
	checkEmail(errs, out.Email)
	switch {
	case len(password) < minPasswordBytes:
		errs.add("password", "Password must be at least 8 characters")
	case len(password) > maxPasswordBytes:
		errs.add("password", "Password must be at most 72 characters")
	case !hasLetterAndDigit(password):
		errs.add("password", "Password needs at least one letter and one number")
	}
	return out, errs
}

func LoginInput(email, password string) (string, Errors) {
	errs := Errors{}
	e := normaliseEmail(email)
	checkEmail(errs, e)
	if password == "" || len(password) > maxPasswordBytes {
		errs.add("password", "Enter your password")
	}
	return e, errs
}

// ---------- polls ----------

type Poll struct {
	Question        string
	Description     string
	Options         []string
	DurationMinutes int // 0 = no automatic close
}

func PollInput(question, description string, options []string, durationMinutes int) (Poll, Errors) {
	errs := Errors{}
	out := Poll{
		Question:        collapseSpaces(question),
		Description:     strings.TrimSpace(description),
		DurationMinutes: durationMinutes,
	}
	checkText(errs, "question", out.Question, 3, maxQuestionLen)

	if utf8.RuneCountInString(out.Description) > maxDescLen {
		errs.add("description", "Description must be at most 500 characters")
	} else if hasControl(out.Description, true) {
		errs.add("description", "Description contains invalid characters")
	}

	if len(options) < MinOptions || len(options) > MaxOptions {
		errs.add("options", "A poll needs between 2 and 10 options")
	} else {
		seen := make(map[string]bool, len(options))
		for _, raw := range options {
			o := collapseSpaces(raw)
			key := strings.ToLower(o)
			switch {
			case o == "":
				errs.add("options", "Options cannot be empty")
			case utf8.RuneCountInString(o) > maxOptionLen:
				errs.add("options", "Each option must be at most 100 characters")
			case hasControl(o, false):
				errs.add("options", "Options contain invalid characters")
			case seen[key]:
				errs.add("options", "Options must be unique")
			}
			seen[key] = true
			out.Options = append(out.Options, o)
		}
	}

	if durationMinutes < 0 || durationMinutes > MaxDurationMins {
		errs.add("durationMinutes", "Duration must be between 0 and 30 days")
	}
	return out, errs
}

func Status(s string) (string, bool) {
	switch s {
	case "open", "closed":
		return s, true
	}
	return "", false
}

// IsSlug guards the URL parameter before it is used in a query.
func IsSlug(s string) bool {
	if len(s) < 4 || len(s) > 16 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func IsOptionID(s string) bool {
	if s == "" || len(s) > 16 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// ---------- helpers ----------

func checkText(errs Errors, field, v string, min, max int) {
	n := utf8.RuneCountInString(v)
	label := strings.ToUpper(field[:1]) + field[1:]
	switch {
	case n < min:
		errs.add(field, label+" is too short")
	case n > max:
		errs.add(field, label+" is too long")
	case hasControl(v, false):
		errs.add(field, label+" contains invalid characters")
	}
}

func checkEmail(errs Errors, email string) {
	if len(email) == 0 || len(email) > 254 {
		errs.add("email", "Enter a valid email address")
		return
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		errs.add("email", "Enter a valid email address")
	}
}

func normaliseEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// collapseSpaces trims and folds runs of whitespace into a single space.
func collapseSpaces(s string) string { return strings.Join(strings.Fields(s), " ") }

func hasControl(s string, allowNewlines bool) bool {
	if !utf8.ValidString(s) {
		return true
	}
	for _, r := range s {
		if allowNewlines && (r == '\n' || r == '\r') {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func hasLetterAndDigit(s string) bool {
	var letter, digit bool
	for _, r := range s {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	return letter && digit
}
