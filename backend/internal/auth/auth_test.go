package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenRoundTrip(t *testing.T) {
	tk := NewTokens([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	raw, err := tk.Issue("user-1")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := tk.Parse(raw)
	if err != nil || sub != "user-1" {
		t.Fatalf("got %q, %v", sub, err)
	}
}

func TestTokenRejectsTamperingAndExpiry(t *testing.T) {
	tk := NewTokens([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	other := NewTokens([]byte("ffffffffffffffffffffffffffffffff"), time.Hour)
	forged, _ := other.Issue("user-1")
	if _, err := tk.Parse(forged); err == nil {
		t.Fatal("accepted token signed with another key")
	}

	expired := NewTokens(tk.secret, -time.Minute)
	old, _ := expired.Issue("user-1")
	if _, err := tk.Parse(old); err == nil {
		t.Fatal("accepted expired token")
	}

	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{Subject: "x", Issuer: "livepoll"}).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := tk.Parse(none); err == nil {
		t.Fatal("accepted alg=none token")
	}
}

func TestPasswords(t *testing.T) {
	h, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "secret123") || CheckPassword(h, "wrong") || CheckPassword("", "secret123") {
		t.Fatal("password check misbehaved")
	}
}
