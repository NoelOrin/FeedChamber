package metrics

import (
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Serve binds addr (e.g. ":9091") and exposes the registry there.
//
// The endpoint is deliberately unauthenticated: it MUST stay on an internal
// network/interface and MUST NOT be proxied by nginx, since it reveals pool and
// traffic internals. Binding happens synchronously so a port conflict is
// reported to the caller instead of silently losing the scrape target.
func Serve(addr string) (*http.Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Handler:           promhttp.HandlerFor(Registry, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Warn("metrics listener stopped serving")
		}
	}()

	return srv, nil
}
