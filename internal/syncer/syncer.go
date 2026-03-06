package syncer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vivek/time-tracker/internal/config"
	"github.com/vivek/time-tracker/internal/monitor"
	"github.com/vivek/time-tracker/internal/storage"
)

type sessionPayload struct {
	ID        int64   `json:"id"`
	MachineID string  `json:"machine_id"`
	Date      string  `json:"date"`
	StartTime string  `json:"start_time"`
	EndTime   string  `json:"end_time"`
	State     string  `json:"state"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type syncRequest struct {
	SyncType string           `json:"sync_type"`
	Sessions []sessionPayload `json:"sessions"`
	SyncedAt string           `json:"synced_at"`
}

// sessionFlusher is implemented by Monitor to close any open session before sync.
type sessionFlusher interface {
	FlushCurrentSession(at time.Time)
}

// Syncer handles morning and evening scheduled syncs.
type Syncer struct {
	cfg                *config.Config
	db                 *storage.DB
	mon                sessionFlusher
	client             *http.Client
	morningSyncedToday string
	eveningSyncedToday string
}

// New creates a Syncer ready to run.
func New(cfg *config.Config, db *storage.DB, mon sessionFlusher) *Syncer {
	return &Syncer{
		cfg:    cfg,
		db:     db,
		mon:    mon,
		client: &http.Client{Timeout: cfg.SyncTimeout()},
	}
}

// Run starts the syncer event loop.
func (s *Syncer) Run(stopCh <-chan struct{}, wakeEvents <-chan monitor.WakeEvent) {
	log.Println("syncer: starting")
	// Check morning sync at startup — handles the case where the machine
	// never slept (so no wake event fired) but it's already past MorningSyncHour.
	s.handleWake(time.Now())
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			log.Println("syncer: shutting down")
			return
		case we := <-wakeEvents:
			log.Printf("syncer: wake event woke=%s", we.WokeAt.Format(time.RFC3339))
			s.handleWake(we.WokeAt)
		case now := <-ticker.C:
			s.handleWake(now)
			s.checkEveningSync(now)
			if now.Minute() == 0 {
				s.prune()
			}
		}
	}
}

func (s *Syncer) handleWake(now time.Time) {
	today := now.Local().Format("2006-01-02")
	if s.morningSyncedToday == today {
		return
	}
	if now.Local().Hour() < s.cfg.MorningSyncHour {
		return
	}
	yesterday := now.Local().AddDate(0, 0, -1).Format("2006-01-02")
	log.Printf("syncer: morning sync pushing sessions up to %s", yesterday)
	sessions, err := s.db.UnsyncedUpToDate(yesterday)
	if err != nil {
		log.Printf("syncer: morning query: %v", err)
		return
	}
	if len(sessions) == 0 {
		log.Println("syncer: morning: nothing to push")
		s.morningSyncedToday = today
		return
	}
	if err := s.push("morning", sessions); err != nil {
		log.Printf("syncer: morning push failed: %v", err)
		return
	}
	s.morningSyncedToday = today
	s.cleanup(yesterday)
}

func (s *Syncer) checkEveningSync(now time.Time) {
	today := now.Local().Format("2006-01-02")
	if s.eveningSyncedToday == today {
		return
	}
	l := now.Local()
	if l.Hour() < s.cfg.EveningSyncHour {
		return
	}
	if l.Hour() == s.cfg.EveningSyncHour && l.Minute() < s.cfg.EveningSyncMinute {
		return
	}
	log.Printf("syncer: evening sync pushing today %s", today)
	// Close any in-progress session so it is included in this sync.
	s.mon.FlushCurrentSession(now)
	sessions, err := s.db.UnsyncedForDate(today)
	if err != nil {
		log.Printf("syncer: evening query: %v", err)
		return
	}
	if len(sessions) == 0 {
		log.Println("syncer: evening: nothing to push")
		s.eveningSyncedToday = today
		return
	}
	if err := s.push("evening", sessions); err != nil {
		log.Printf("syncer: evening push failed: %v", err)
		return
	}
	s.eveningSyncedToday = today
	yesterday := now.Local().AddDate(0, 0, -1).Format("2006-01-02")
	s.cleanup(yesterday)
}

// push sends sessions to the API with up to 3 retries.
func (s *Syncer) push(syncType string, sessions []storage.Session) error {
	payload := syncRequest{
		SyncType: syncType,
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
		Sessions: toPayload(sessions),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	var lastErr error
	backoff := 5 * time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		log.Printf("syncer: push attempt %d/3 (%d sessions)", attempt, len(sessions))
		req, err := http.NewRequest(http.MethodPost, s.cfg.SyncAPIURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if s.cfg.SyncAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.cfg.SyncAPIKey)
		}
		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("syncer: attempt %d network error: %v retrying in %s", attempt, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("server %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			log.Printf("syncer: attempt %d server error retrying in %s", attempt, backoff)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		log.Printf("syncer: push OK %d sessions status=%d", len(sessions), resp.StatusCode)
		ids := make([]int64, len(sessions))
		for i, ss := range sessions {
			ids[i] = ss.ID
		}
		if err := s.db.MarkSynced(ids); err != nil {
			log.Printf("syncer: mark synced: %v", err)
		}
		return nil
	}
	return fmt.Errorf("all retries exhausted: %w", lastErr)
}

func (s *Syncer) cleanup(upToDate string) {
	t, err := time.ParseInLocation("2006-01-02", upToDate, time.Local)
	if err == nil {
		nextDay := t.AddDate(0, 0, 1).Format("2006-01-02")
		n, err := s.db.DeleteSyncedBefore(nextDay)
		if err != nil {
			log.Printf("syncer: delete synced: %v", err)
		} else if n > 0 {
			log.Printf("syncer: deleted %d synced sessions up to %s", n, upToDate)
		}
	}
}

func (s *Syncer) prune() {
	n, err := s.db.DeleteOlderThan(s.cfg.RetentionDays)
	if err != nil {
		log.Printf("syncer: prune: %v", err)
	} else if n > 0 {
		log.Printf("syncer: pruned %d old sessions", n)
	}
}

func toPayload(sessions []storage.Session) []sessionPayload {
	out := make([]sessionPayload, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, sessionPayload{
			ID:        s.ID,
			MachineID: s.MachineID,
			Date:      s.Date,
			StartTime: s.StartTime.Format(time.RFC3339),
			EndTime:   s.EndTime.Format(time.RFC3339),
			State:     string(s.State),
			Latitude:  s.Latitude,
			Longitude: s.Longitude,
		})
	}
	return out
}
