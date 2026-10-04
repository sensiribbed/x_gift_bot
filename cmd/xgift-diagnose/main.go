// xgift-diagnose reads locally saved failures. It has no HTTP client and never
// imports checkout execution code. SQLite is opened read-only.
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/scrypt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(os.Args[1]) {
		return errors.New("usage: xgift-diagnose <32-character order ID from app log>")
	}
	password, err := os.ReadFile("/data/vault-password")
	if err != nil {
		return errors.New("cannot read vault password")
	}
	defer clear(password)
	db, err := sql.Open("sqlite3", "file:/data/vault.db?mode=ro&_busy_timeout=5000")
	if err != nil {
		return errors.New("cannot open vault read-only")
	}
	defer db.Close()
	var salt []byte
	if err = db.QueryRow("SELECT value FROM metadata WHERE key='salt'").Scan(&salt); err != nil || len(salt) != 32 {
		return errors.New("invalid vault metadata")
	}
	key, err := scrypt.Key(bytes.TrimSpace(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return errors.New("cannot derive vault key")
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return errors.New("cannot initialize vault cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return errors.New("cannot initialize vault cipher")
	}
	var check []byte
	if err = db.QueryRow("SELECT payload FROM secrets WHERE name='vault-check'").Scan(&check); err != nil {
		return errors.New("vault authentication record missing")
	}
	plain, err := decrypt(aead, "vault-check", check)
	if err != nil || string(plain) != "xgift-v1" {
		clear(plain)
		return errors.New("vault authentication failed")
	}
	clear(plain)
	rows, err := db.Query("SELECT name,payload FROM secrets WHERE name LIKE ? ORDER BY name DESC LIMIT 5", "redemption-failure:"+os.Args[1]+":%")
	if err != nil {
		return errors.New("cannot read saved failure records")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name string
		var payload []byte
		if err = rows.Scan(&name, &payload); err != nil {
			return errors.New("cannot read failure record")
		}
		plain, err := decrypt(aead, name, payload)
		if err != nil {
			return errors.New("failure record authentication failed")
		}
		var record struct {
			Error string `json:"error"`
			Stage string `json:"stage"`
			Months int `json:"months"`
			Observed int64 `json:"observed_at"`
		}
		err = json.Unmarshal(plain, &record)
		clear(plain)
		if err != nil {
			return errors.New("invalid saved failure JSON")
		}
		stage := "redacted"
		switch record.Stage {
		case "before_order", "creating", "created", "submitting", "unknown", "requires_action", "succeeded":
			stage = record.Stage
		}
		// Never print raw upstream error text, URLs, usernames, or credentials.
		fmt.Printf("observed_at=%d months=%d stage=%s reason=%s\n", record.Observed, record.Months, stage, classify(record.Error))
		count++
	}
	if rows.Err() != nil {
		return errors.New("failure record query failed")
	}
	if count == 0 {
		return errors.New("no saved failure found for this order ID")
	}
	return nil
}

func decrypt(aead cipher.AEAD, name string, payload []byte) ([]byte, error) {
	n := aead.NonceSize()
	if len(payload) < n+aead.Overhead() {
		return nil, errors.New("invalid encrypted payload")
	}
	return aead.Open(nil, payload[:n], payload[n:], []byte("xgift-v1:"+name))
}

func classify(message string) string {
	switch message {
	case "unexpected X product or price list":
		return "X_PRODUCT_OR_PRICE_LIST_MISMATCH"
	case "X price is not exactly the allowed one-time amount":
		return "X_PRICE_CURRENCY_OR_PAYMENT_TYPE_MISMATCH"
	case "card, cardholder name or email is incomplete":
		return "CARD_CONFIGURATION_INCOMPLETE"
	case "existing checkout differs from this recipient or plan; refusing another order":
		return "EXISTING_ORDER_PLAN_MISMATCH"
	case "invalid Stripe merchant publishable key":
		return "STRIPE_PUBLIC_KEY_INVALID"
	case "context deadline exceeded":
		return "TIMEOUT"
	}
	if strings.Contains(message, "X read failed") {
		return "X_READ_FAILED"
	}
	return "OTHER_ERROR_REDACTED"
}
