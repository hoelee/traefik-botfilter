// Package botfilter is a Traefik middleware plugin that blocks known scans
// and applies a bounded, in-memory per-IP suspicion score.
package botfilter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"
)

type botFilter struct {
	next   http.Handler
	config *compiledConfig
	cache  *clientCache
	logger filterLogger
}

// New constructs a Traefik middleware. Its signature is the interface used
// by Traefik's Go plugin runtime.
func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("botfilter: next handler is nil")
	}
	compiled, err := compileConfig(config)
	if err != nil {
		return nil, err
	}
	return &botFilter{
		next:   next,
		config: compiled,
		cache:  newClientCache(compiled),
		logger: filterLogger{name: name, enabled: compiled.LogBlockedRequests},
	}, nil
}

func (b *botFilter) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	now := time.Now()
	addr := clientIP(r, b.config)
	ip := addressString(addr)
	if addressInPrefixes(addr, b.config.whitelist) {
		b.next.ServeHTTP(rw, r)
		return
	}
	if existing := b.cache.isBanned(ip, now); existing.banned {
		b.reject(rw, ip, "temporary ban", existing)
		return
	}

	match := inspectRequest(r, b.config)
	paths := requestPaths(r)
	decision := b.cache.observeRequest(ip, requestObservation{
		at:       now,
		paths:    paths,
		points:   match.points,
		forceBan: match.forceBan,
	})
	if decision.banned {
		reason := match.reason
		if reason == "" {
			reason = "suspicion score threshold"
		}
		b.reject(rw, ip, reason, decision)
		return
	}

	recorder := &statusRecorder{ResponseWriter: rw}
	b.next.ServeHTTP(recorder, r)
	if result := b.cache.observeResponse(ip, recorder.statusCode(), b.config.NotFoundScore, time.Now()); result.banned {
		b.logger.blocked(ip, "404 score threshold", result.score)
	}
}

func (b *botFilter) reject(rw http.ResponseWriter, ip, reason string, decision cacheResult) {
	b.logger.blocked(ip, reason, decision.score)
	rw.Header().Set("Cache-Control", "no-store")
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if decision.banUntil.After(time.Now()) {
		seconds := int(time.Until(decision.banUntil).Seconds())
		if seconds < 1 {
			seconds = 1
		}
		rw.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	rw.WriteHeader(b.config.StatusCode)
	_, _ = io.WriteString(rw, "request denied\n")
}

func addressString(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

// statusRecorder keeps downstream response capabilities available while
// recording the final status used for 404 scoring.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func (r *statusRecorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, options *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (r *statusRecorder) ReadFrom(source io.Reader) (int64, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if readerFrom, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		return readerFrom.ReadFrom(source)
	}
	return io.Copy(struct{ io.Writer }{Writer: r.ResponseWriter}, source)
}
