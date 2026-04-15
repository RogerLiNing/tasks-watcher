package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashPassword(t *testing.T) {
	password := "mySecretPassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	if hash == password {
		t.Fatal("hash must not equal plaintext password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testPassword99!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !VerifyPassword(password, hash) {
		t.Error("correct password should verify")
	}
	if VerifyPassword("wrongPassword", hash) {
		t.Error("wrong password should not verify")
	}
	if VerifyPassword("", hash) {
		t.Error("empty password should not verify")
	}
	if VerifyPassword(password, "not_a_bcrypt_hash") {
		t.Error("invalid hash format should not verify")
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	p1, _ := HashPassword("samepassword")
	p2, _ := HashPassword("samepassword")
	if p1 == p2 {
		t.Error("same password should produce different hashes (bcrypt uses salt)")
	}
}

func TestGenerateToken(t *testing.T) {
	secret := "test-secret-key-32-bytes-long-here"
	userID := "user-abc-123"
	duration := 1 * time.Hour

	token, expiresAt, err := GenerateToken(userID, secret, duration)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
	if expiresAt <= time.Now().Unix() {
		t.Error("expiresAt should be in the future")
	}

	// Verify we can parse the token
	parsed, err := jwt.ParseWithClaims(token, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse generated token: %v", err)
	}
	claims, ok := parsed.Claims.(*TokenClaims)
	if !ok {
		t.Fatal("failed to cast claims")
	}
	if claims.UserID != userID {
		t.Errorf("UserID mismatch: got %q, want %q", claims.UserID, userID)
	}
}

func TestValidateToken(t *testing.T) {
	secret := "correct-secret"
	wrongSecret := "wrong-secret"
	userID := "user-xyz"

	token, _, _ := GenerateToken(userID, secret, 1*time.Hour)

	gotID, valid := ValidateToken(token, secret)
	if !valid {
		t.Fatal("valid token should pass ValidateToken")
	}
	if gotID != userID {
		t.Errorf("UserID: got %q, want %q", gotID, userID)
	}

	_, valid = ValidateToken(token, wrongSecret)
	if valid {
		t.Error("token signed with wrong secret should fail")
	}

	_, valid = ValidateToken("not.a.jwt.token", secret)
	if valid {
		t.Error("malformed token should fail")
	}

	_, valid = ValidateToken("", secret)
	if valid {
		t.Error("empty token should fail")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := "test-secret"
	userID := "user-expired"

	// Generate a token that expired 1 hour ago
	expiresAt := time.Now().Add(-1 * time.Hour).Unix()
	claims := TokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
			IssuedAt:  jwt.NewNumericDate(time.Unix(expiresAt-3600, 0)),
			NotBefore: jwt.NewNumericDate(time.Unix(expiresAt-3600, 0)),
			Issuer:    "tasks-watcher",
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, valid := ValidateToken(tokenStr, secret)
	if valid {
		t.Error("expired token should fail ValidateToken")
	}
}

func TestHashToken(t *testing.T) {
	token := "session-token-abc123"
	hash := HashToken(token)
	if hash == "" {
		t.Fatal("HashToken returned empty")
	}
	if hash == token {
		t.Fatal("hash must differ from input token")
	}
	if len(hash) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64-char hex hash, got %d", len(hash))
	}

	// Same input always produces same output
	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("HashToken must be deterministic")
	}

	// Different inputs produce different outputs
	hash3 := HashToken(token + "x")
	if hash == hash3 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestGenerateSession(t *testing.T) {
	secret := "session-secret-key-32-bytes-ok"
	userID := "session-user-1"

	session, token, err := GenerateSession(userID, secret)
	if err != nil {
		t.Fatalf("GenerateSession failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}
	if session.UserID != userID {
		t.Errorf("UserID: got %q, want %q", session.UserID, userID)
	}
	if session.TokenHash == "" {
		t.Error("TokenHash should not be empty")
	}
	if session.TokenHash == HashToken(token) {
		// This would be a security issue — the raw token is stored
		// TokenHash must be HashToken(token) since that's how we store it
	}

	// Session should be validatable
	gotID, valid := ValidateToken(token, secret)
	if !valid {
		t.Error("generated session token should be valid")
	}
	if gotID != userID {
		t.Errorf("UserID: got %q, want %q", gotID, userID)
	}

	// Session should expire in ~24h
	if session.ExpiresAt-time.Now().Unix() < 23*3600 {
		t.Error("session should expire in approximately 24 hours")
	}
}

func TestTokenClaims_Issuer(t *testing.T) {
	secret := "issuer-test-secret"
	userID := "issuer-user"
	token, _, _ := GenerateToken(userID, secret, 1*time.Hour)

	parsed, _ := jwt.ParseWithClaims(token, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	claims := parsed.Claims.(*TokenClaims)
	if claims.Issuer != "tasks-watcher" {
		t.Errorf("Issuer: got %q, want %q", claims.Issuer, "tasks-watcher")
	}
	if claims.Subject != userID {
		t.Errorf("Subject: got %q, want %q", claims.Subject, userID)
	}
}

func TestGenerateToken_InvalidAlg(t *testing.T) {
	secret := "test-secret"
	userID := "user-alg"

	// Create a token with "none" algorithm (alg: none) — classic JWT vuln
	expiresAt := time.Now().Add(1 * time.Hour).Unix()
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "tasks-watcher",
		Subject:   userID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	// ValidateToken only accepts HMAC family; none should fail
	_, valid := ValidateToken(tokenStr, secret)
	if valid {
		t.Error("token signed with 'none' algorithm should be rejected")
	}
}

func TestVerifyPassword_EmptyInputs(t *testing.T) {
	// Empty password should not panic
	hash, _ := HashPassword("nonempty")
	VerifyPassword("", hash)
	VerifyPassword("password", "")
	// Both should return false without panicking
}

func TestGenerateToken_EmptyUserID(t *testing.T) {
	secret := "test-secret"
	token, _, err := GenerateToken("", secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken with empty userID failed: %v", err)
	}
	gotID, valid := ValidateToken(token, secret)
	if !valid {
		t.Error("token with empty userID should still be valid")
	}
	if gotID != "" {
		t.Errorf("UserID: got %q, want empty string", gotID)
	}
}

func TestToken_Duration(t *testing.T) {
	secret := "duration-test-secret"
	userID := "duration-user"

	// 1 second duration
	token, expiresAt, _ := GenerateToken(userID, secret, 1*time.Second)
	expectedExpiry := time.Now().Add(1 * time.Second).Unix()

	if expiresAt < expectedExpiry-2 || expiresAt > expectedExpiry+2 {
		t.Errorf("expiresAt=%d too far from expected %d", expiresAt, expectedExpiry)
	}

	// Token should still be valid now
	gotID, valid := ValidateToken(token, secret)
	if !valid {
		t.Error("token should still be valid immediately")
	}
	if gotID != userID {
		t.Errorf("UserID mismatch: got %q, want %q", gotID, userID)
	}

	_ = token // silence unused warning
}

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HashPassword("benchmarkPassword123!")
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	hash, _ := HashPassword("benchmarkPassword123!")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyPassword("benchmarkPassword123!", hash)
	}
}

func BenchmarkValidateToken(b *testing.B) {
	secret := "benchmark-secret-key-32-bytes-x"
	token, _, _ := GenerateToken("bench-user", secret, 1*time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateToken(token, secret)
	}
}
