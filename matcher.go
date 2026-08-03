package botfilter

import (
	"net/http"
	"strings"
)

type matchResult struct {
	points   int
	forceBan bool
	reason   string
}

func inspectRequest(r *http.Request, cfg *compiledConfig) matchResult {
	paths := requestPaths(r)
	userAgent := strings.TrimSpace(r.UserAgent())

	if cfg.RequireUserAgent && userAgent == "" {
		return matchResult{points: cfg.EmptyUserAgentScore, forceBan: true, reason: "missing user-agent"}
	}
	if cfg.RequireAccept && !hasHeader(r, "Accept") {
		return matchResult{points: cfg.MissingAcceptScore, forceBan: true, reason: "missing accept"}
	}
	if cfg.RequireHost && strings.TrimSpace(r.Host) == "" {
		return matchResult{forceBan: true, reason: "missing host"}
	}
	if containsToken(strings.ToLower(userAgent), cfg.blockedAgents) {
		return matchResult{points: cfg.BlockedUserAgentScore, forceBan: true, reason: "blocked user-agent"}
	}
	if matchesBlockedPath(paths, cfg.blockedPaths) {
		return matchResult{points: cfg.BadPathScore, forceBan: true, reason: "blocked path"}
	}
	if matchesBlockedExtension(paths, cfg.blockedExts) {
		return matchResult{points: cfg.BadPathScore, forceBan: true, reason: "blocked extension"}
	}

	result := matchResult{}
	if userAgent == "" {
		result.points += cfg.EmptyUserAgentScore
		result.reason = "empty user-agent"
	}
	if !hasHeader(r, "Accept") {
		result.points += cfg.MissingAcceptScore
		result.reason = appendReason(result.reason, "missing accept")
	}
	if cfg.BrowserValidation && implausibleBrowser(r, userAgent) {
		result.points += cfg.FakeBrowserScore
		result.reason = appendReason(result.reason, "implausible browser headers")
	}
	return result
}

func containsToken(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func matchesBlockedPath(paths, blockedPaths []string) bool {
	for _, requestPath := range paths {
		if pathMatchesAny(requestPath, blockedPaths) {
			return true
		}
	}
	return false
}

func matchesBlockedExtension(paths, extensions []string) bool {
	for _, requestPath := range paths {
		for _, extension := range extensions {
			if strings.HasSuffix(requestPath, extension) {
				return true
			}
		}
	}
	return false
}

func pathMatchesAny(requestPath string, rules []string) bool {
	for _, rule := range rules {
		if pathMatches(requestPath, rule) {
			return true
		}
	}
	return false
}

func hasStaticAssetExtension(requestPath string) bool {
	for _, extension := range []string{".css", ".js", ".mjs", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico", ".woff", ".woff2"} {
		if strings.HasSuffix(requestPath, extension) {
			return true
		}
	}
	return false
}

func implausibleBrowser(r *http.Request, userAgent string) bool {
	ua := strings.ToLower(userAgent)
	if !strings.Contains(ua, "mozilla/") {
		return false
	}

	// Do not require optional browser client-hint or fetch-metadata headers:
	// older browsers and privacy tools legitimately omit them. Instead reject
	// internally inconsistent browser family declarations.
	isChromium := strings.Contains(ua, "chrome/") || strings.Contains(ua, "crios/") || strings.Contains(ua, "edg/") || strings.Contains(ua, "opr/")
	isFirefox := strings.Contains(ua, "firefox/")
	isSafari := strings.Contains(ua, "safari/") && !isChromium

	if !isChromium && !isFirefox && !isSafari {
		return true
	}
	if isChromium && !strings.Contains(ua, "applewebkit/") {
		return true
	}
	if isFirefox && !strings.Contains(ua, "gecko/") {
		return true
	}
	if isSafari && (!strings.Contains(ua, "applewebkit/") || !strings.Contains(ua, "version/")) {
		return true
	}

	if value := r.Header.Get("Sec-Fetch-Site"); value != "" {
		switch value {
		case "same-origin", "same-site", "cross-site", "none":
		default:
			return true
		}
	}
	return false
}

func appendReason(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}
