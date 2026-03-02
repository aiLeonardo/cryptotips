package app

import (
	"strings"
	"sync"
	"time"

	"github.com/aiLeonardo/cryptotips/indicator"
	"github.com/aiLeonardo/cryptotips/models"
)

const regimeCacheTTL = 3 * time.Minute

var regimeAnalysisStartUTC = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)

type regimeCacheEntry struct {
	DailyLatestUnix  int64
	WeeklyLatestUnix int64
	CachedAt         time.Time
	Items            []RegimeStartpointItem
}

var realtimeRegimeCache = struct {
	mu      sync.RWMutex
	entries map[string]regimeCacheEntry
}{
	entries: make(map[string]regimeCacheEntry),
}

func (a *goapi) getRealtimeRegimeStartpoints(symbol string, start, end time.Time) ([]RegimeStartpointItem, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, nil
	}

	m := &models.KLineRecord{}
	dailyLatest, err := m.GetLatestKLineTime(a.db, symbol, "1d")
	if err != nil {
		return nil, err
	}
	weeklyLatest, err := m.GetLatestKLineTime(a.db, symbol, "1w")
	if err != nil {
		return nil, err
	}

	dailyLatestUnix := latestUnix(dailyLatest)
	weeklyLatestUnix := latestUnix(weeklyLatest)
	if dailyLatestUnix == 0 || weeklyLatestUnix == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	realtimeRegimeCache.mu.RLock()
	entry, ok := realtimeRegimeCache.entries[symbol]
	realtimeRegimeCache.mu.RUnlock()
	if ok && entry.DailyLatestUnix == dailyLatestUnix && entry.WeeklyLatestUnix == weeklyLatestUnix && now.Sub(entry.CachedAt) <= regimeCacheTTL {
		return filterRegimeByRange(entry.Items, start, end), nil
	}

	dailyRecords, err := m.GetKLines(a.db, symbol, "1d", regimeAnalysisStartUTC, time.Time{})
	if err != nil {
		return nil, err
	}
	weeklyRecords, err := m.GetKLines(a.db, symbol, "1w", regimeAnalysisStartUTC, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(dailyRecords) == 0 || len(weeklyRecords) == 0 {
		return nil, nil
	}

	dailyBars := make([]indicator.RegimeBar, 0, len(dailyRecords))
	for _, r := range dailyRecords {
		dailyBars = append(dailyBars, indicator.RegimeBar{
			Time:  r.OpenTime.UTC(),
			Close: r.Close,
		})
	}

	weeklyBars := make([]indicator.RegimeBar, 0, len(weeklyRecords))
	for _, r := range weeklyRecords {
		weeklyBars = append(weeklyBars, indicator.RegimeBar{
			Time:  r.OpenTime.UTC(),
			Close: r.Close,
		})
	}

	startpoints := indicator.ComputeRegimeStartpoints(dailyBars, weeklyBars)
	items := make([]RegimeStartpointItem, 0, len(startpoints))
	for _, sp := range startpoints {
		items = append(items, RegimeStartpointItem{
			Time:       sp.Time.Unix(),
			State:      sp.State,
			Confidence: sp.Confidence,
		})
	}

	realtimeRegimeCache.mu.Lock()
	realtimeRegimeCache.entries[symbol] = regimeCacheEntry{
		DailyLatestUnix:  dailyLatestUnix,
		WeeklyLatestUnix: weeklyLatestUnix,
		CachedAt:         now,
		Items:            cloneRegimeItems(items),
	}
	realtimeRegimeCache.mu.Unlock()

	return filterRegimeByRange(items, start, end), nil
}

func latestUnix(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UTC().Unix()
}

func filterRegimeByRange(items []RegimeStartpointItem, start, end time.Time) []RegimeStartpointItem {
	if len(items) == 0 {
		return nil
	}
	if start.IsZero() && end.IsZero() {
		return cloneRegimeItems(items)
	}

	out := make([]RegimeStartpointItem, 0, len(items))
	for _, it := range items {
		t := time.Unix(it.Time, 0).UTC()
		if !start.IsZero() && t.Before(start.UTC()) {
			continue
		}
		if !end.IsZero() && t.After(end.UTC()) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func cloneRegimeItems(items []RegimeStartpointItem) []RegimeStartpointItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]RegimeStartpointItem, len(items))
	copy(out, items)
	return out
}
