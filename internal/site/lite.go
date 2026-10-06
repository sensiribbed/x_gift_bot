package site

// The lite entrypoint deliberately has its own route and worker allowlist.
// Upstream additions to Run do not become publicly reachable here.
import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"xgift/internal/proxy"
	"xgift/internal/vault"
)

func (s *server) liteHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.asset("lite.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /favicon.svg", s.asset("favicon.svg", "image/svg+xml"))
	mux.HandleFunc("GET /appearance.js", s.asset("appearance.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("GET /lite.js", s.asset("lite.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		b, err := s.vault.Get("vault-check")
		defer clear(b)
		if err != nil || string(b) != "xgift-v1" {
			reply(w, 503, map[string]any{"ok": false})
			return
		}
		reply(w, 200, map[string]any{"ok": true, "mode": "lite", "payments_enabled": false})
	})
	mux.HandleFunc("GET /api/security", s.securityConfig)
	mux.HandleFunc("POST /api/check", s.human("check", s.check))
	mux.HandleFunc("GET /api/manual-link/plans", s.publicLinkPlans)
	mux.HandleFunc("POST /api/manual-link", s.human("manual_link", s.publicLink))
	mux.HandleFunc("GET /api/manual-link/queue/{ticket}", s.publicLinkQueueStatus)
	return s.middleware(mux)
}

func RunLite(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	origin := os.Getenv("XGIFT_ORIGIN")
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return errors.New("XGIFT_ORIGIN must be an HTTPS origin")
	}
	dir := os.Getenv("XGIFT_DATA_DIR")
	if dir == "" { return errors.New("XGIFT_DATA_DIR is required") }
	if err = os.MkdirAll(dir, 0700); err != nil { return err }
	instance, err := os.OpenFile(filepath.Join(dir, "site.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil { return err }
	defer instance.Close()
	if err = syscall.Flock(int(instance.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another site instance is using this data directory")
	}
	v, err := vault.Open(filepath.Join(dir, "vault.db"), os.Getenv("XGIFT_PASSWORD_FILE"), false)
	if err != nil { return err }
	defer v.Close()
	s := &server{vault: v, origin: origin, lockPath: filepath.Join(dir, "checkout.lock"), work: make(chan struct{}, 1), checks: make(chan struct{}, 4), ctx: ctx, limits: map[string]limit{}}
	if err = s.configureTurnstile(); err != nil { return err }
	raw, err := v.Get("proxy")
	if err != nil { return err }
	defer clear(raw)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return err }
	s.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	box, err := proxy.Start(ctx, raw, s.port)
	if err != nil { return err }
	defer box.Close()
	addr := os.Getenv("XGIFT_LISTEN")
	if addr == "" { addr = "127.0.0.1:8787" }
	host, _, err := net.SplitHostPort(addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("listen address must use a loopback IP")
	}
	h := &http.Server{Addr: addr, Handler: s.liteHandler(), ReadHeaderTimeout: 5*time.Second, ReadTimeout: 10*time.Second, WriteTimeout: 130*time.Second, IdleTimeout: 60*time.Second, MaxHeaderBytes: 16 << 10}
	done := make(chan error, 1)
	go func() { done <- h.ListenAndServe() }()
	log.Printf("xgift-lite listening on %s; eligibility and manual links only", addr)
	// No redemption database, card processing, or automatic recovery worker.
	s.jobs.Add(1)
	go func() { defer s.jobs.Done(); s.publicLinkQueueLoop() }()
	select {
	case err = <-done:
	case <-ctx.Done():
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 55*time.Second)
	defer stop()
	if h.Shutdown(shutdown) != nil { h.Close() }
	s.jobs.Wait()
	if errors.Is(err, http.ErrServerClosed) { return nil }
	return err
}
