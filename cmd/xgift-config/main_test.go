package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectsCardBeforeCreatingVault(t *testing.T) {
	dir := t.TempDir()
	input, err := os.CreateTemp(dir,"input")
	if err != nil { t.Fatal(err) }
	defer input.Close()
	input.WriteString(`{"cookies":{},"api-auth":{},"proxy":{},"stripe-key":"pk_live_test","catalog":{},"card":{}}`)
	input.Seek(0,0)
	oldArgs, oldIn := os.Args, os.Stdin
	t.Cleanup(func(){ os.Args=oldArgs; os.Stdin=oldIn })
	db := filepath.Join(dir,"vault.db")
	os.Args=[]string{"xgift-config","--db",db,"init"}
	os.Stdin=input
	if err=run(); err==nil || !strings.Contains(err.Error(),"only") { t.Fatalf("unexpected result: %v",err) }
	if _,err=os.Stat(db); !os.IsNotExist(err) { t.Fatal("invalid input created a vault") }
}
