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

func TestProtocolSessionDerivationVectors(t *testing.T) {
	// Fixed vectors protect the MD5 derivations mandated by EcoFlow firmware.
	shared := make([]byte, 20)
	for i := range shared {
		shared[i] = byte(i)
	}
	key, iv := type7SessionSeed(shared)
	if hex.EncodeToString(key) != "000102030405060708090a0b0c0d0e0f" || hex.EncodeToString(iv) != "1549d1aae20214e065ab4b76aaac89a8" {
		t.Fatalf("type 7 protocol vector changed: key=%x, iv=%x", key, iv)
	}
	key, iv = type1Session("P231FAB4PJ7X3193")
	if hex.EncodeToString(key) != "3e819efdbcc2f698a23c1504182bd64b" || hex.EncodeToString(iv) != "0668bde719e434fce51e78dfe7d6963a" {
		t.Fatalf("type 1 protocol vector changed: key=%x, iv=%x", key, iv)
	}
}
