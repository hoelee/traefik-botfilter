package botfilter

import "log"

type filterLogger struct {
	name    string
	enabled bool
}

func (l filterLogger) blocked(ip, reason string, score int) {
	if !l.enabled {
		return
	}
	// This is intentionally opt-in. Logging every rejected request during a
	// flood can become a second source of CPU and disk pressure.
	log.Printf("botfilter[%s]: blocked client=%s reason=%q score=%d", l.name, ip, reason, score)
}
