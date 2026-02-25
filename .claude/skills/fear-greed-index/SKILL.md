---
name: bitcoin-fng-sync
description: Adds Bitcoin Fear and Greed Index syncing capability to an existing Golang project. Fetches data from https://api.alternative.me/fng/ and persists it to the local project. Use this skill whenever the user wants to: sync or fetch Bitcoin fear and greed index data, add FNG (fear and greed) tracking to a Go project, integrate with alternative.me API, schedule periodic crypto sentiment data syncing, store or query fear/greed index history, or display current Bitcoin market sentiment in a Go application. Trigger even if the user just says "add fear and greed index" or "sync FNG data" without specifying implementation details.
---

# Bitcoin Fear and Greed Index Sync Skill (Golang)

## Overview

This skill adds a **Fear and Greed Index** sync module to an existing Golang project. It fetches data from `https://api.alternative.me/fng/`(Just one data currently.) and stores it locally (database or file). The implementation follows idiomatic Go patterns. 
put on limit parameter to read all data,fear greed history data，https://api.alternative.me/fng/?limit=0

---

## Step 1: Understand the Existing Project

Before writing any code, inspect the project structure:

```bash
find . -name "*.go" | head -30
ls -la
cat go.mod
```

Identify:
- **Module name** from `go.mod`
- **Directory structure** (cmd/, internal/, pkg/, etc.)
- **Database layer** (GORM, sqlx, raw SQL, none?)
- **HTTP client patterns** already in use
- **Config/env loading** approach (viper, godotenv, os.Getenv, etc.)
- **Existing model/entity patterns**

---

## Step 2: Create the Data Model

Create the Fear and Greed model matching the API response. Place it in the appropriate models/entities directory of the project.

**File: `internal/model/fear_greed.go`** (adjust path to match project structure)

```go
package model

import "time"

// FearGreedIndex represents a single Fear and Greed Index data point
type FearGreedIndex struct {
    ID                  uint      `json:"id" gorm:"primaryKey;autoIncrement"`
    Value               int       `json:"value"`
    ValueClassification string    `json:"value_classification"`
    Timestamp           time.Time `json:"timestamp"`
    CreatedAt           time.Time `json:"created_at"`
}

// FearGreedAPIResponse is the raw API response from alternative.me
type FearGreedAPIResponse struct {
    Name     string             `json:"name"`
    Data     []FearGreedData    `json:"data"`
    Metadata FearGreedMetadata  `json:"metadata"`
}

type FearGreedData struct {
    Value               string `json:"value"`
    ValueClassification string `json:"value_classification"`
    Timestamp           string `json:"timestamp"`
    TimeUntilUpdate     string `json:"time_until_update,omitempty"`
}

type FearGreedMetadata struct {
    Error *string `json:"error"`
}
```

---

## Step 3: Create the Sync Service

**File: `internal/service/fear_greed_service.go`** (adjust path to match project)

```go
package service

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "strconv"
    "time"

    "YOUR_MODULE/internal/model" // replace with actual module path
)

const fearGreedAPIURL = "https://api.alternative.me/fng/"

// FearGreedRepository defines the storage interface
type FearGreedRepository interface {
    Save(ctx context.Context, index *model.FearGreedIndex) error
    GetLatest(ctx context.Context) (*model.FearGreedIndex, error)
    GetAll(ctx context.Context) ([]model.FearGreedIndex, error)
}

// FearGreedService handles fetching and syncing Fear and Greed Index data
type FearGreedService struct {
    repo       FearGreedRepository
    httpClient *http.Client
}

// NewFearGreedService creates a new FearGreedService
func NewFearGreedService(repo FearGreedRepository) *FearGreedService {
    return &FearGreedService{
        repo: repo,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// Fetch fetches the latest Fear and Greed Index from the API
func (s *FearGreedService) Fetch(ctx context.Context) (*model.FearGreedIndex, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, fearGreedAPIURL, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Accept", "application/json")

    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching fear greed index: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("reading response body: %w", err)
    }

    var apiResp model.FearGreedAPIResponse
    if err := json.Unmarshal(body, &apiResp); err != nil {
        return nil, fmt.Errorf("parsing response: %w", err)
    }

    if apiResp.Metadata.Error != nil {
        return nil, fmt.Errorf("API error: %s", *apiResp.Metadata.Error)
    }

    if len(apiResp.Data) == 0 {
        return nil, fmt.Errorf("no data returned from API")
    }

    return parseAPIData(apiResp.Data[0])
}

// Sync fetches the latest index and saves it to the repository
func (s *FearGreedService) Sync(ctx context.Context) (*model.FearGreedIndex, error) {
    index, err := s.Fetch(ctx)
    if err != nil {
        return nil, fmt.Errorf("fetch: %w", err)
    }

    if err := s.repo.Save(ctx, index); err != nil {
        return nil, fmt.Errorf("save: %w", err)
    }

    log.Printf("[FearGreed] Synced: value=%d classification=%s timestamp=%s",
        index.Value, index.ValueClassification, index.Timestamp.Format(time.RFC3339))

    return index, nil
}

// GetLatest returns the most recently stored index
func (s *FearGreedService) GetLatest(ctx context.Context) (*model.FearGreedIndex, error) {
    return s.repo.GetLatest(ctx)
}

// parseAPIData converts raw API data to the domain model
func parseAPIData(d model.FearGreedData) (*model.FearGreedIndex, error) {
    value, err := strconv.Atoi(d.Value)
    if err != nil {
        return nil, fmt.Errorf("parsing value %q: %w", d.Value, err)
    }

    ts, err := strconv.ParseInt(d.Timestamp, 10, 64)
    if err != nil {
        return nil, fmt.Errorf("parsing timestamp %q: %w", d.Timestamp, err)
    }

    return &model.FearGreedIndex{
        Value:               value,
        ValueClassification: d.ValueClassification,
        Timestamp:           time.Unix(ts, 0).UTC(),
    }, nil
}
```

---

## Step 4: Create the Repository Implementation

Choose the implementation based on what the project uses.

### Option A: GORM (if project uses GORM)

**File: `internal/repository/fear_greed_repository.go`**

```go
package repository

import (
    "context"
    "YOUR_MODULE/internal/model"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

type fearGreedRepo struct {
    db *gorm.DB
}

func NewFearGreedRepository(db *gorm.DB) service.FearGreedRepository {
    // Auto-migrate the table
    db.AutoMigrate(&model.FearGreedIndex{})
    return &fearGreedRepo{db: db}
}

func (r *fearGreedRepo) Save(ctx context.Context, index *model.FearGreedIndex) error {
    return r.db.WithContext(ctx).
        Clauses(clause.OnConflict{DoNothing: true}).
        Create(index).Error
}

func (r *fearGreedRepo) GetLatest(ctx context.Context) (*model.FearGreedIndex, error) {
    var index model.FearGreedIndex
    err := r.db.WithContext(ctx).
        Order("timestamp DESC").
        First(&index).Error
    if err != nil {
        return nil, err
    }
    return &index, nil
}

func (r *fearGreedRepo) GetAll(ctx context.Context) ([]model.FearGreedIndex, error) {
    var items []model.FearGreedIndex
    err := r.db.WithContext(ctx).
        Order("timestamp DESC").
        Find(&items).Error
    return items, err
}
```

### Option B: JSON File (if project has no DB)

**File: `internal/repository/fear_greed_file_repository.go`**

```go
package repository

import (
    "context"
    "encoding/json"
    "os"
    "sort"
    "sync"
    "YOUR_MODULE/internal/model"
)

type fearGreedFileRepo struct {
    mu       sync.RWMutex
    filePath string
}

func NewFearGreedFileRepository(filePath string) *fearGreedFileRepo {
    return &fearGreedFileRepo{filePath: filePath}
}

func (r *fearGreedFileRepo) load() ([]model.FearGreedIndex, error) {
    data, err := os.ReadFile(r.filePath)
    if os.IsNotExist(err) {
        return []model.FearGreedIndex{}, nil
    }
    if err != nil {
        return nil, err
    }
    var items []model.FearGreedIndex
    return items, json.Unmarshal(data, &items)
}

func (r *fearGreedFileRepo) save(items []model.FearGreedIndex) error {
    data, err := json.MarshalIndent(items, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(r.filePath, data, 0644)
}

func (r *fearGreedFileRepo) Save(ctx context.Context, index *model.FearGreedIndex) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    items, err := r.load()
    if err != nil {
        return err
    }
    // Deduplicate by timestamp
    for _, existing := range items {
        if existing.Timestamp.Equal(index.Timestamp) {
            return nil
        }
    }
    items = append(items, *index)
    return r.save(items)
}

func (r *fearGreedFileRepo) GetLatest(ctx context.Context) (*model.FearGreedIndex, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    items, err := r.load()
    if err != nil || len(items) == 0 {
        return nil, err
    }
    sort.Slice(items, func(i, j int) bool {
        return items[i].Timestamp.After(items[j].Timestamp)
    })
    return &items[0], nil
}

func (r *fearGreedFileRepo) GetAll(ctx context.Context) ([]model.FearGreedIndex, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.load()
}
```

---

## Step 5: Add Scheduler (Optional)

If the project uses cron or has a scheduler, add periodic syncing.

**File: `internal/scheduler/fear_greed_scheduler.go`**

```go
package scheduler

import (
    "context"
    "log"
    "time"

    "YOUR_MODULE/internal/service"
)

// StartFearGreedSync starts a background goroutine that syncs
// the Fear and Greed Index at the given interval.
// The API updates once per day, so interval should be >= 1 hour.
func StartFearGreedSync(ctx context.Context, svc *service.FearGreedService, interval time.Duration) {
    go func() {
        // Sync immediately on start
        if _, err := svc.Sync(ctx); err != nil {
            log.Printf("[FearGreed] Initial sync failed: %v", err)
        }

        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                log.Println("[FearGreed] Scheduler stopped")
                return
            case <-ticker.C:
                if _, err := svc.Sync(ctx); err != nil {
                    log.Printf("[FearGreed] Periodic sync failed: %v", err)
                }
            }
        }
    }()
}
```

---

## Step 6: Wire It Up

In the project's main initialization (e.g., `main.go`, `cmd/server/main.go`, `internal/app/app.go`):

```go
// Initialize repository (choose based on project's DB layer)
fngRepo := repository.NewFearGreedRepository(db)           // GORM
// fngRepo := repository.NewFearGreedFileRepository("data/fng.json") // File

// Initialize service
fngService := service.NewFearGreedService(fngRepo)

// Option 1: Sync once
ctx := context.Background()
index, err := fngService.Sync(ctx)
if err != nil {
    log.Printf("FNG sync failed: %v", err)
} else {
    log.Printf("FNG: %d (%s)", index.Value, index.ValueClassification)
}

// Option 2: Start background scheduler (syncs every 1 hour)
scheduler.StartFearGreedSync(ctx, fngService, 1*time.Hour)
```

---

## Step 7: Fetch Multiple Days (Optional)

The API supports fetching historical data via query param `?limit=N`:

```go
const fearGreedAPIURL = "https://api.alternative.me/fng/?limit=30"
```

To expose this as a parameter in the service:

```go
func (s *FearGreedService) FetchN(ctx context.Context, limit int) ([]model.FearGreedIndex, error) {
    url := fmt.Sprintf("%s?limit=%d", fearGreedAPIURL, limit)
    // ... same fetch logic, return slice
}
```

---

## Key Notes

- **API rate limit**: The API is free and public, but sync no more than once per hour. The index only updates daily.
- **Deduplication**: Always deduplicate by `timestamp` to avoid storing the same data point twice.
- **Error handling**: Wrap all errors with `fmt.Errorf("...: %w", err)` for proper error chains.
- **Context**: Always pass `context.Context` through all layers for cancellation support.
- **Module path**: Replace `YOUR_MODULE` with the actual module name from `go.mod`.
- **Adjust paths**: Match the project's existing directory conventions (`pkg/` vs `internal/`, flat vs nested).
