package sign

import (
	"testing"
	"time"
)

func TestVerifyPublicTokenV1(t *testing.T) {
	secret := "secret"
	appID := uint32(1001)
	expireAt := uint32(time.Now().Add(time.Hour).Unix())

	raw := BuildPublicToken(secret, appID, expireAt)
	if got, want := len(raw), 32; got != want {
		t.Fatalf("unexpected V1 token length: got %d want %d", got, want)
	}

	capability, ok := VerifyPublicTokenV2(secret, appID, raw)
	if !ok {
		t.Fatal("V1 token should verify")
	}
	if !VerifyPublicToken(secret, appID, raw) {
		t.Fatal("V1 token should verify through the compatibility wrapper")
	}
	if capability != 0 {
		t.Fatalf("unexpected V1 capability: got %d want 0", capability)
	}
}

func TestVerifyPublicTokenV2(t *testing.T) {
	secret := "secret"
	appID := uint32(1001)
	expireAt := uint32(time.Now().Add(time.Hour).Unix())
	capability := uint32(0x010203)

	raw, ok := BuildPublicTokenV2(secret, appID, expireAt, capability)
	if !ok {
		t.Fatal("V2 token should build")
	}
	if got, want := len(raw), 36; got != want {
		t.Fatalf("unexpected V2 token length: got %d want %d", got, want)
	}

	gotCapability, ok := VerifyPublicTokenV2(secret, appID, raw)
	if !ok {
		t.Fatal("V2 token should verify")
	}
	if !VerifyPublicToken(secret, appID, raw) {
		t.Fatal("V2 token should verify through the compatibility wrapper")
	}
	if gotCapability != capability {
		t.Fatalf("unexpected V2 capability: got %d want %d", gotCapability, capability)
	}
}

func TestPublicTokenRejectsInvalidInputs(t *testing.T) {
	secret := "secret"
	appID := uint32(1001)
	expireAt := uint32(time.Now().Add(time.Hour).Unix())

	raw, ok := BuildPublicTokenV2(secret, appID, expireAt, 1)
	if !ok {
		t.Fatal("V2 token should build")
	}
	if _, ok := VerifyPublicTokenV2(secret, appID+1, raw); ok {
		t.Fatal("token should not verify for another app")
	}
	if _, ok := VerifyPublicTokenV2("another-secret", appID, raw); ok {
		t.Fatal("token should not verify with another secret")
	}
	if _, ok := VerifyPublicTokenV2(secret, appID, mutateToken(raw)); ok {
		t.Fatal("token should not verify after tampering")
	}

	expired := BuildPublicToken(secret, appID, uint32(time.Now().Add(-time.Hour).Unix()))
	if _, ok := VerifyPublicTokenV2(secret, appID, expired); ok {
		t.Fatal("expired token should not verify")
	}
	if _, ok := BuildPublicTokenV2(secret, appID, expireAt, 1<<24); ok {
		t.Fatal("capability outside 24 bits should not build")
	}
}

func mutateToken(raw string) string {
	if raw[len(raw)-1] == 'A' {
		return raw[:len(raw)-1] + "B"
	}
	return raw[:len(raw)-1] + "A"
}
