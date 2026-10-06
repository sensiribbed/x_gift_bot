package site

import (
	"net/http/httptest"
	"strings"
	"testing"
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

func TestLiteEligibilityRoute(t *testing.T) {
	s := checkFixture(t)
	calls := 0
	fakeEligibility(t,&calls,"1234",nil)
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
