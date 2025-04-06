package spmetrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"stream.place/streamplace/pkg/log"
)

const SESSION_EXPIRE_TIME = 30 * time.Second

var viewers = map[string]int{}
var viewersLock sync.RWMutex

var sessions = map[string]map[string]time.Time{}
var sessionsLock sync.RWMutex
var Viewers = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_viewers",
	Help: "number of current viewers per user",
}, []string{"user"})

var ViewersTotal = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "streamplace_viewers_total",
	Help: "total number of viewers",
})

var TranscodeAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_attempts_total",
	Help: "total number of transcode attempts",
})

var TranscodeSuccessesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_successes_total",
	Help: "total number of transcode successes",
})

var TranscodeErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_errors_total",
	Help: "total number of transcode errors",
})

var Version = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_version",
	Help: "version of streamplace",
}, []string{"version"})

func ViewerInc(user string) {
	go func() {
		viewersLock.Lock()
		defer viewersLock.Unlock()
		viewers[user]++
		Viewers.WithLabelValues(user).Set(float64(viewers[user]))
		ViewersTotal.Inc()
	}()
}

func ViewerDec(user string) {
	go func() {
		viewersLock.Lock()
		defer viewersLock.Unlock()
		viewers[user]--
		if viewers[user] == 0 {
			Viewers.DeleteLabelValues(user)
		} else {
			Viewers.WithLabelValues(user).Set(float64(viewers[user]))
		}
		ViewersTotal.Dec()
	}()
}

func GetViewCount(user string) int {
	viewersLock.RLock()
	defer viewersLock.RUnlock()
	return viewers[user]
}

func SessionSeen(user string, session string) {
	now := time.Now()
	go func() {
		sessionsLock.Lock()
		defer sessionsLock.Unlock()
		if _, ok := sessions[user]; !ok {
			sessions[user] = map[string]time.Time{}
		}
		if _, ok := sessions[user][session]; !ok {
			log.Warn(context.TODO(), "ViewerInc", "user", user, "session", session)
			ViewerInc(user)
		}
		sessions[user][session] = now
	}()
}

func ExpireSessions(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			sessionsLock.Lock()
			for user, sessions := range sessions {
				for session, seen := range sessions {
					if time.Since(seen) > SESSION_EXPIRE_TIME {
						delete(sessions, session)
						ViewerDec(user)
					}
				}
			}
			sessionsLock.Unlock()
		}
	}
}
