package botfilter

import (
	"sync"
	"time"
)

type scoreEvent struct {
	at     time.Time
	points int
}

type clientState struct {
	banUntil  time.Time
	lastSeen  time.Time
	requests  int
	sawHome    bool
	sawStatic  bool
	sawFavicon bool
	events    []scoreEvent
}

type requestObservation struct {
	at       time.Time
	paths    []string
	points   int
	forceBan bool
}

type cacheResult struct {
	banned   bool
	banUntil time.Time
	score    int
}

// clientCache deliberately has no background goroutine. A middleware
// instance can be discarded on a Traefik reload without needing goroutine
// cleanup, and opportunistic cleanup keeps its memory use bounded.
type clientCache struct {
	mu          sync.Mutex
	entries     map[string]*clientState
	maxEntries  int
	maxEvents   int
	scoreWindow time.Duration
	banDuration time.Duration
	threshold   int
	randomPaths []string
	randomScore int
	operations  uint64
}

func newClientCache(cfg *compiledConfig) *clientCache {
	return &clientCache{
		entries:     make(map[string]*clientState),
		maxEntries:  cfg.MaxTrackedIPs,
		maxEvents:   cfg.MaxScoreEventsPerIP,
		scoreWindow: cfg.scoreWindow,
		banDuration: cfg.banDuration,
		threshold:   cfg.ScoreThreshold,
		randomPaths: append([]string(nil), cfg.randomPaths...),
		randomScore: cfg.RandomArticleScore,
	}
}

func (c *clientCache) isBanned(ip string, now time.Time) cacheResult {
	if ip == "" {
		return cacheResult{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.entries[ip]
	if state == nil || !state.banUntil.After(now) {
		return cacheResult{}
	}
	return cacheResult{banned: true, banUntil: state.banUntil}
}

func (c *clientCache) observeRequest(ip string, observation requestObservation) cacheResult {
	if ip == "" {
		// An invalid peer address must never share a cache entry with another
		// malformed request. The request can still be rejected by its headers.
		return cacheResult{banned: observation.forceBan}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.operations++
	c.cleanupLocked(observation.at)

	state := c.entries[ip]
	if state != nil && state.banUntil.After(observation.at) {
		return cacheResult{banned: true, banUntil: state.banUntil}
	}
	if state == nil {
		c.ensureCapacityLocked(observation.at)
		state = &clientState{}
		c.entries[ip] = state
	}

	c.pruneEventsLocked(state, observation.at)
	points := observation.points + c.behaviorScoreLocked(state, observation.paths)
	state.requests++
	state.lastSeen = observation.at
	if points > 0 {
		state.events = append(state.events, scoreEvent{at: observation.at, points: points})
		if len(state.events) > c.maxEvents {
			state.events = append([]scoreEvent(nil), state.events[len(state.events)-c.maxEvents:]...)
		}
	}
	score := sumScore(state.events)
	if observation.forceBan || score >= c.threshold {
		state.banUntil = observation.at.Add(c.banDuration)
		return cacheResult{banned: true, banUntil: state.banUntil, score: score}
	}
	return cacheResult{score: score}
}

func (c *clientCache) observeResponse(ip string, status int, points int, now time.Time) cacheResult {
	if ip == "" || status != 404 || points <= 0 {
		return cacheResult{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	state := c.entries[ip]
	if state == nil || state.banUntil.After(now) {
		return cacheResult{}
	}
	c.pruneEventsLocked(state, now)
	state.lastSeen = now
	state.events = append(state.events, scoreEvent{at: now, points: points})
	if len(state.events) > c.maxEvents {
		state.events = append([]scoreEvent(nil), state.events[len(state.events)-c.maxEvents:]...)
	}
	score := sumScore(state.events)
	if score >= c.threshold {
		state.banUntil = now.Add(c.banDuration)
		return cacheResult{banned: true, banUntil: state.banUntil, score: score}
	}
	return cacheResult{score: score}
}

func (c *clientCache) behaviorScoreLocked(state *clientState, paths []string) int {
	// The heuristic is intentionally weak: visiting a content page first is
	// normal for a shared link, so it adds only the configured 15 points.
	// It becomes useful in combination with malformed headers or 404 scans.
	firstRequest := state.requests == 0
	isContent := false
	for _, requestPath := range paths {
		if requestPath == "/" || requestPath == "/index.html" {
			state.sawHome = true
		}
		if requestPath == "/favicon.ico" {
			state.sawFavicon = true
		}
		if hasStaticAssetExtension(requestPath) {
			state.sawStatic = true
		}
		if pathMatchesAny(requestPath, c.randomPaths) {
			isContent = true
		}
	}
	if firstRequest && isContent && !state.sawHome && !state.sawStatic && !state.sawFavicon {
		return c.randomScore
	}
	return 0
}

func (c *clientCache) pruneEventsLocked(state *clientState, now time.Time) {
	cutoff := now.Add(-c.scoreWindow)
	first := 0
	for first < len(state.events) && !state.events[first].at.After(cutoff) {
		first++
	}
	if first > 0 {
		state.events = append([]scoreEvent(nil), state.events[first:]...)
	}
}

func (c *clientCache) cleanupLocked(now time.Time) {
	// Full cleanup every 256 requests amortises the map scan while retaining
	// recently scored clients for the entire score window.
	if c.operations%256 != 0 {
		return
	}
	cutoff := now.Add(-c.scoreWindow)
	for ip, state := range c.entries {
		if !state.banUntil.After(now) && !state.lastSeen.After(cutoff) {
			delete(c.entries, ip)
		}
	}
}

func (c *clientCache) ensureCapacityLocked(now time.Time) {
	if len(c.entries) < c.maxEntries {
		return
	}

	// Evict the oldest non-banned entry first. If all entries are banned, the
	// oldest ban is evicted; the cap remains a hard upper bound either way.
	var oldestIP string
	var oldestTime time.Time
	for ip, state := range c.entries {
		candidate := state.lastSeen
		if state.banUntil.After(now) {
			candidate = state.banUntil
		}
		if oldestIP == "" || candidate.Before(oldestTime) {
			oldestIP, oldestTime = ip, candidate
		}
	}
	if oldestIP != "" {
		delete(c.entries, oldestIP)
	}
}

func sumScore(events []scoreEvent) int {
	total := 0
	for _, event := range events {
		total += event.points
	}
	return total
}
