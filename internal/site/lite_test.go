package site

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"xgift/internal/vault"
)

func TestLiteExcludesFullApplication(t *testing.T) {
	t.Setenv("XGIFT_PAYMENTS_ENABLED", "true")
	s := checkFixture(t)
	h := s.liteHandler()
	for _, path := range []string{"/admin", "/admin.js", "/app.js", "/api/admin/codes", "/api/admin/recovery/start", "/api/redeem", "/api/status", "/api/admin/manual-link"} {
		for _, method := range []string{"GET", "POST"} {
			r := httptest.NewRequest(method, path, strings.NewReader("{}"))
			r.Header.Set("Origin", s.origin)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w,r)
			if w.Code != 404 { t.Errorf("%s %s returned %d",method,path,w.Code) }
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w,httptest.NewRequest("GET","/healthz",nil))
	if w.Code!=200 || !strings.Contains(w.Body.String(), `"payments_enabled":false`) { t.Fatal(w.Body.String()) }
}

func TestLiteQueueRoutesSurviveRestart(t *testing.T) {
	s := checkFixture(t)
	if err := s.vault.Put("catalog", []byte(`{"merchant":"acct_Test","currency":"bdt","plans":[{"months":3,"amount":30000,"product":"prod_Test"}]}`)); err != nil { t.Fatal(err) }
	owner := strings.Repeat("a",64)
	call := func(s *server, method, path, body, cookie string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method,path,strings.NewReader(body))
		r.Header.Set("Origin",s.origin)
		r.Header.Set("Content-Type","application/json")
		r.AddCookie(&http.Cookie{Name:"__Host-xgift-link",Value:cookie})
		w := httptest.NewRecorder()
		s.liteHandler().ServeHTTP(w,r)
		return w
	}
	w := call(s,"POST","/api/manual-link",`{"username":"someone","months":3,"request_id":"`+strings.Repeat("b",64)+`"}`,owner)
	if w.Code!=202 { t.Fatalf("enqueue: %d %s",w.Code,w.Body.String()) }
	var response struct { Ticket string `json:"ticket"` }
	if json.Unmarshal(w.Body.Bytes(),&response)!=nil || response.Ticket=="" { t.Fatal("missing ticket") }
	restarted := &server{vault:s.vault,origin:s.origin,work:make(chan struct{},1),checks:make(chan struct{},4),limits:map[string]limit{}}
	if err:=restarted.restorePublicLinkQueue(); err!=nil { t.Fatal(err) }
	for _, path := range []string{"/api/manual-link/queue/current","/api/manual-link/queue/"+response.Ticket} {
		w=call(restarted,"GET",path,"",owner)
		if w.Code!=200 && w.Code!=202 { t.Fatalf("recover %s: %d %s",path,w.Code,w.Body.String()) }
	}
	w=call(restarted,"GET","/api/manual-link/queue/current","",strings.Repeat("c",64))
	if w.Code!=404 { t.Fatal("another browser recovered the ticket") }
	w=call(restarted,"GET","/api/manual-link/queue","",owner)
	if w.Code!=200 { t.Fatal("queue summary unavailable") }
	w=call(restarted,"POST","/api/manual-link/queue/"+response.Ticket+"/leave","{}",owner)
	if w.Code!=200 { t.Fatal("leave route unavailable") }
	w=call(restarted,"POST","/api/manual-link/queue/"+response.Ticket+"/cancel","{}",owner)
	if w.Code!=200 { t.Fatal("cancel route unavailable") }
	w=call(restarted,"GET","/api/manual-link/queue/current","",owner)
	if w.Code!=404 { t.Fatal("cancelled ticket still recoverable") }
}

func TestLiteEligibilityRoute(t *testing.T) {
	s := checkFixture(t)
	calls := 0
	original := eligibilityCheck
	eligibilityCheck = func(context.Context, *vault.Vault, string, int) (string, error) { calls++; return "1234", nil }
	t.Cleanup(func() { eligibilityCheck = original })
	r := httptest.NewRequest("POST","/api/check",strings.NewReader(`{"username":"someone"}`))
	r.Header.Set("Origin",s.origin)
	r.Header.Set("Content-Type","application/json")
	w := httptest.NewRecorder()
	s.liteHandler().ServeHTTP(w,r)
	if w.Code!=200 || calls!=1 || !strings.Contains(w.Body.String(),`"eligible":true`) { t.Fatalf("%d %s",w.Code,w.Body.String()) }
}

func TestLiteRejectsCrossOrigin(t *testing.T) {
	s := checkFixture(t)
	r := httptest.NewRequest("POST","/api/manual-link",strings.NewReader("{}"))
	r.Header.Set("Origin","https://other.test")
	w := httptest.NewRecorder()
	s.liteHandler().ServeHTTP(w,r)
	if w.Code!=403 { t.Fatal(w.Code) }
}
