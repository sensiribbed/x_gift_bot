// Configuration only: this executable cannot submit a checkout or payment.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"xgift/internal/vault"
)

var allowed = map[string]bool{"cookies":true, "api-auth":true, "proxy":true, "stripe-key":true, "catalog":true}

func main() {
	if err := run(); err != nil { fmt.Fprintln(os.Stderr, "xgift-config:", err); os.Exit(1) }
}

func run() error {
	f := flag.NewFlagSet("xgift-config", flag.ContinueOnError)
	db := f.String("db", "/data/vault.db", "vault path")
	key := f.String("password-file", os.Getenv("XGIFT_PASSWORD_FILE"), "vault password file")
	name := f.String("name", "", "configuration record")
	if err := f.Parse(os.Args[1:]); err != nil { return err }
	if f.NArg() != 1 { return errors.New("usage: xgift-config [flags] init|put|status") }
	cmd := f.Arg(0)
	if cmd != "init" && cmd != "put" && cmd != "status" { return errors.New("unsupported command") }
	var records map[string]json.RawMessage
	var value []byte
	if cmd == "init" || cmd == "put" {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 1024*1024+1))
		if err != nil || len(b)>1024*1024 { return errors.New("invalid configuration input") }
		defer clear(b)
		if cmd == "init" {
			if json.Unmarshal(b, &records) != nil || len(records) != len(allowed) { return errors.New("expected cookies, api-auth, proxy, stripe-key and catalog only") }
			for n, raw := range records {
				if !allowed[n] || string(raw)=="null" { return errors.New("unsupported or empty configuration record") }
			}
		} else {
			if !allowed[*name] { return errors.New("unsupported configuration record") }
			if *name == "stripe-key" { value=[]byte(strings.TrimSpace(string(b))) } else {
				if !json.Valid(b) { return errors.New("configuration must be JSON") }; value=b
			}
		}
	}
	v, err := vault.Open(*db, *key, cmd=="init")
	if err != nil { return err }
	defer v.Close()
	if cmd == "init" {
		for n, raw := range records {
			b := []byte(raw)
			if n == "stripe-key" { var k string; if json.Unmarshal(raw,&k)!=nil { return errors.New("stripe-key must be a string") }; b=[]byte(k) }
			if err = v.Put(n,b); err != nil { return err }
		}
	} else if cmd == "put" {
		if err = v.Put(*name,value); err != nil { return err }
	}
	for n := range allowed {
		b, e := v.Get(n)
		clear(b)
		if e != nil { return fmt.Errorf("required configuration missing: %s",n) }
	}
	fmt.Println("Lite configuration ready (values hidden).")
	return nil
}
