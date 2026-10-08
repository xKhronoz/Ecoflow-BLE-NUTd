package ecoflow

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestEmbeddedKeyData(t *testing.T) {
	t.Parallel()
	data, err := loadKeyData()
	if err != nil {
		t.Fatalf("load embedded key data: %v", err)
	}
	if len(data) != 65280 {
		t.Fatalf("key data length = %d, want 65280", len(data))
	}
	sum := sha256.Sum256(data)
	const want = "3d1ba59b40c46bc86ba35fa231a3db196b448bbdfb1635c88ddd42f24abd4b2a"
	if hex.EncodeToString(sum[:]) != want {
		t.Fatalf("embedded key data checksum = %x", sum)
	}
}

func TestDeriveFinalSessionKey(t *testing.T) {
	t.Parallel()
	srand := make([]byte, 16)
	for i := range srand {
		srand[i] = byte(i)
	}
	key, err := deriveFinalSessionKey([]byte{0, 1}, srand)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(key); got != "cf19aab35c7605235e885521945354d8" {
		t.Fatalf("session key = %s", got)
	}
	for _, seed := range [][]byte{nil, {0}, {255, 0}} {
		if _, err := deriveFinalSessionKey(seed, srand); err == nil {
			t.Fatalf("expected error for seed %x", seed)
		}
	}
	if _, err := deriveFinalSessionKey([]byte{0, 1}, srand[:15]); err == nil {
		t.Fatal("expected error for short srand")
	}
}
