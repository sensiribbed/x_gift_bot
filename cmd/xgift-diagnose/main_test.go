package main

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestFailureOutputDoesNotEchoSecrets(t *testing.T) {
	if got := classify("server included private credential and checkout URL"); got != "OTHER_ERROR_REDACTED" {
		t.Fatal("unrecognized upstream errors must not be printed")
	}
	if got := classify("unexpected X product or price list"); got != "X_PRODUCT_OR_PRICE_LIST_MISMATCH" {
		t.Fatal(got)
	}
}

func TestDecryptAuthenticatesRecordName(t *testing.T) {
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	sealed := aead.Seal(append([]byte{}, nonce...), nonce, []byte("test"), []byte("xgift-v1:record"))
	plain, err := decrypt(aead, "record", sealed)
	if err != nil || string(plain) != "test" { t.Fatal("valid payload rejected") }
	if _, err = decrypt(aead, "other", sealed); err == nil { t.Fatal("record substitution accepted") }
	if _, err = decrypt(aead, "record", []byte{1}); err == nil { t.Fatal("truncated payload accepted") }
}
