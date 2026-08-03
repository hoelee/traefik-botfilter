package botfilter

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	pathpkg "path"
	"strings"
)

func clientIP(r *http.Request, cfg *compiledConfig) netip.Addr {
	remote := parseRemoteAddress(r.RemoteAddr)
	if cfg.clientIPHeader == "" || !addressInPrefixes(remote, cfg.trustedProxies) {
		return remote
	}

	// A trusted reverse proxy must overwrite (not append to an untrusted
	// client-provided value) this header. The left-most valid value is the
	// original client in the conventional X-Forwarded-For representation.
	for _, value := range strings.Split(r.Header.Get(cfg.clientIPHeader), ",") {
		if addr, err := netip.ParseAddr(strings.TrimSpace(value)); err == nil {
			return addr.Unmap()
		}
	}
	return remote
}

func parseRemoteAddress(value string) netip.Addr {
	host, _, err := net.SplitHostPort(strings.TrimSpace(value))
	if err == nil {
		value = host
	}
	addr, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(value), "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func addressInPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	if !addr.IsValid() {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func normaliseTokens(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if token := strings.ToLower(strings.TrimSpace(value)); token != "" {
			result = append(result, token)
		}
	}
	return result
}

func normalisePaths(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if path := normalisePath(value); path != "" {
			result = append(result, path)
		}
	}
	return result
}

func normaliseExtensions(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if !strings.HasPrefix(value, ".") {
			value = "." + value
		}
		result = append(result, value)
	}
	return result
}

func normalisePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return strings.ToLower(pathpkg.Clean(value))
}

func requestPaths(r *http.Request) []string {
	values := []string{r.URL.Path, r.URL.EscapedPath()}
	result := make([]string, 0, len(values)*3)
	seen := make(map[string]struct{})
	for _, value := range values {
		for i := 0; i < 3 && value != ""; i++ {
			normalised := normalisePath(value)
			if _, ok := seen[normalised]; !ok {
				seen[normalised] = struct{}{}
				result = append(result, normalised)
			}
			decoded, err := url.PathUnescape(value)
			if err != nil || decoded == value {
				break
			}
			value = decoded
		}
	}
	return result
}

func pathMatches(path, rule string) bool {
	return path == rule || strings.HasPrefix(path, rule+"/")
}

func hasHeader(r *http.Request, name string) bool {
	return strings.TrimSpace(r.Header.Get(name)) != ""
}
