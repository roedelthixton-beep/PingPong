package main

import (
	"testing"

	"pingpong_server/rpc/auth/dal"
)

func TestIsOTPMatched(t *testing.T) {
	email := "ops@test.com"
	ip := "10.0.0.1"
	challenge := &dal.AuthEmailChallenge{
		CodeHash: sha256Hex("654321"),
		Email:    &email,
		ClientIP: &ip,
	}

	t.Run("whitelist_match_bypasses_hash", func(t *testing.T) {
		svc := &AuthServiceImpl{
			mockUniversalOTP:   "abc123",
			mockOTPEmailSuffix: []string{"@test.com"},
			mockOTPIPWhitelist: []string{"10.0.0.1"},
		}
		if !svc.isOTPMatched("abc123", challenge) {
			t.Fatal("expected mock OTP to pass when email suffix + IP match")
		}
	})

	t.Run("whitelist_wrong_ip_rejects", func(t *testing.T) {
		svc := &AuthServiceImpl{
			mockUniversalOTP:   "abc123",
			mockOTPEmailSuffix: []string{"@test.com"},
			mockOTPIPWhitelist: []string{"192.168.1.1"},
		}
		// Email suffix matches but IP doesn't → should reject (not fall through to hash check)
		if svc.isOTPMatched("abc123", challenge) {
			t.Fatal("expected mock OTP to fail when IP does not match whitelist")
		}
		// Even the real OTP hash should fail because email suffix matched → entered mock path
		if svc.isOTPMatched("654321", challenge) {
			t.Fatal("expected real OTP to also fail when email suffix matched but IP rejected")
		}
	})

	t.Run("whitelist_wrong_email_suffix_uses_hash", func(t *testing.T) {
		svc := &AuthServiceImpl{
			mockUniversalOTP:   "abc123",
			mockOTPEmailSuffix: []string{"@other.com"},
			mockOTPIPWhitelist: []string{"10.0.0.1"},
		}
		// Email suffix doesn't match → normal hash path
		if svc.isOTPMatched("abc123", challenge) {
			t.Fatal("expected mock OTP to fail when email suffix does not match")
		}
		if !svc.isOTPMatched("654321", challenge) {
			t.Fatal("expected real OTP hash to work when email suffix does not match")
		}
	})

	t.Run("no_whitelist_config_uses_hash", func(t *testing.T) {
		svc := &AuthServiceImpl{
			mockUniversalOTP: "abc123",
		}
		if svc.isOTPMatched("abc123", challenge) {
			t.Fatal("expected mock OTP to fail when no whitelist configured")
		}
		if !svc.isOTPMatched("654321", challenge) {
			t.Fatal("expected real OTP hash to work when no whitelist configured")
		}
	})

	t.Run("normal_otp_hash_still_works", func(t *testing.T) {
		svc := &AuthServiceImpl{}
		if !svc.isOTPMatched("654321", challenge) {
			t.Fatal("expected challenge OTP hash check to remain valid")
		}
	})

	t.Run("mock_otp_allows_alphanumeric", func(t *testing.T) {
		svc := &AuthServiceImpl{
			mockUniversalOTP:   "Pass99",
			mockOTPEmailSuffix: []string{"@test.com"},
			mockOTPIPWhitelist: []string{"10.0.0.1"},
		}
		if !svc.isOTPMatched("Pass99", challenge) {
			t.Fatal("expected alphanumeric mock OTP to pass")
		}
	})

	t.Run("nil_client_ip_rejects_when_email_matches", func(t *testing.T) {
		noIPChallenge := &dal.AuthEmailChallenge{
			CodeHash: sha256Hex("654321"),
			Email:    &email,
			ClientIP: nil,
		}
		svc := &AuthServiceImpl{
			mockUniversalOTP:   "abc123",
			mockOTPEmailSuffix: []string{"@test.com"},
			mockOTPIPWhitelist: []string{"10.0.0.1"},
		}
		if svc.isOTPMatched("abc123", noIPChallenge) {
			t.Fatal("expected mock OTP to fail when client IP is nil")
		}
	})
}

func TestIsMockOTPBypass(t *testing.T) {
	svc := &AuthServiceImpl{
		mockUniversalOTP:   "abc123",
		mockOTPEmailSuffix: []string{"@test.com"},
		mockOTPIPWhitelist: []string{"10.0.0.1", "127.0.0.1"},
	}

	t.Run("matching_email_and_ip_bypass_ip_limit", func(t *testing.T) {
		if !svc.isMockOTPBypass("ops@test.com", "127.0.0.1") {
			t.Fatal("expected mock OTP bypass when both email suffix and IP match")
		}
	})

	t.Run("matching_email_without_ip_does_not_bypass", func(t *testing.T) {
		if svc.isMockOTPBypass("ops@test.com", "192.168.1.1") {
			t.Fatal("expected no bypass when IP is not allowlisted")
		}
	})

	t.Run("matching_ip_without_email_does_not_bypass", func(t *testing.T) {
		if svc.isMockOTPBypass("ops@example.com", "127.0.0.1") {
			t.Fatal("expected no bypass when email suffix is not allowlisted")
		}
	})
}
