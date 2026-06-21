package ecoflow

import (
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"

	"github.com/fd/secp160r1"
)

//go:embed assets/keydata.b64
var keyDataBase64 string

var (
	keyDataOnce sync.Once
	keyData     []byte
	keyDataErr  error
)

func loadKeyData() ([]byte, error) {
	keyDataOnce.Do(func() {
		keyData, keyDataErr = base64.StdEncoding.DecodeString(keyDataBase64)
	})
	return keyData, keyDataErr
}

func generateSECP160KeyPair() (privateKey []byte, publicKey []byte, err error) {
	curve := secp160r1.P160()
	privateKey, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	publicKey = make([]byte, 0, 40)
	publicKey = append(publicKey, leftPad(x.Bytes(), 20)...)
	publicKey = append(publicKey, leftPad(y.Bytes(), 20)...)
	return privateKey, publicKey, nil
}

func deriveSharedSecret(privateKey []byte, devicePubKey []byte) ([]byte, error) {
	if len(devicePubKey) != 40 {
		return nil, fmt.Errorf("unsupported device public key size %d", len(devicePubKey))
	}
	curve := secp160r1.P160()
	x := new(big.Int).SetBytes(devicePubKey[:20])
	y := new(big.Int).SetBytes(devicePubKey[20:])
	if !curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("device public key is not on secp160r1")
	}
	sx, _ := curve.ScalarMult(x, y, privateKey)
	return leftPad(sx.Bytes(), 20), nil
}

func type7SessionSeed(sharedSecret []byte) (sessionKey []byte, iv []byte) {
	sum := md5.Sum(sharedSecret)
	return append([]byte(nil), sharedSecret[:16]...), sum[:]
}

func deriveFinalSessionKey(seed []byte, srand []byte) ([]byte, error) {
	keyData, err := loadKeyData()
	if err != nil {
		return nil, err
	}
	if len(seed) < 2 || len(srand) < 16 {
		return nil, fmt.Errorf("invalid key info payload")
	}
	pos := int(seed[0])*0x10 + int((seed[1]-1)&0xFF)*0x100
	if pos+16 > len(keyData) {
		return nil, fmt.Errorf("key data offset out of range")
	}
	data := make([]byte, 32)
	copy(data[0:8], keyData[pos:pos+8])
	copy(data[8:16], keyData[pos+8:pos+16])
	copy(data[16:24], srand[:8])
	copy(data[24:32], srand[8:16])
	sum := md5.Sum(data)
	return sum[:], nil
}

func type1Session(serial string) (sessionKey []byte, iv []byte) {
	keySum := md5.Sum([]byte(serial))
	reversed := []byte(serial)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	ivSum := md5.Sum(reversed)
	return keySum[:], ivSum[:]
}

func getECDHTypeSize(curveNum byte) int {
	switch curveNum {
	case 1:
		return 52
	case 2:
		return 56
	case 3, 4:
		return 64
	default:
		return 40
	}
}

func leftPad(in []byte, size int) []byte {
	if len(in) >= size {
		return append([]byte(nil), in[len(in)-size:]...)
	}
	out := make([]byte, size)
	copy(out[size-len(in):], in)
	return out
}

func parseUint64LE(data []byte) uint64 {
	return binary.LittleEndian.Uint64(data)
}
