package account

import (
	"testing"
)

func TestExtractToken(t *testing.T) {
	// 1. Root accessToken (what Cloudflare Worker actually returns)
	json1 := []byte(`{"ok": true, "accessToken": "test_token_123", "expiresAtUtc": "2026-11-24T12:00:00Z"}`)
	if tok := extractToken(json1); tok != "test_token_123" {
		t.Fatalf("expected test_token_123, got %s", tok)
	}

	// 2. Root token
	json2 := []byte(`{"ok": true, "token": "test_token_456"}`)
	if tok := extractToken(json2); tok != "test_token_456" {
		t.Fatalf("expected test_token_456, got %s", tok)
	}

	// 3. Nested session token
	json3 := []byte(`{"ok": true, "session": {"token": "test_token_789"}}`)
	if tok := extractToken(json3); tok != "test_token_789" {
		t.Fatalf("expected test_token_789, got %s", tok)
	}

	// 4. Nested data accessToken
	json4 := []byte(`{"ok": true, "data": {"accessToken": "test_token_abc"}}`)
	if tok := extractToken(json4); tok != "test_token_abc" {
		t.Fatalf("expected test_token_abc, got %s", tok)
	}
}

func TestExtractErrorMessage(t *testing.T) {
	json1 := []byte(`{"ok": false, "error": "Account not found"}`)
	if msg := extractErrorMessage(json1); msg != "Аккаунт не найден" {
		t.Fatalf("expected 'Аккаунт не найден', got '%s'", msg)
	}

	json2 := []byte(`{"ok": false, "error": "Invalid credentials"}`)
	if msg := extractErrorMessage(json2); msg != "Неверный логин или пароль" {
		t.Fatalf("expected 'Неверный логин или пароль', got '%s'", msg)
	}
}
