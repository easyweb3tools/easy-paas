# Codex Development Guide

This document describes the codebase conventions and patterns for the polymarket backend service. Follow these patterns exactly when adding new features.

## Project Layout

```
services/polymarket/backend/
  cmd/monitor/main.go              # Entry point, wires all dependencies
  internal/
    models/                        # GORM models (one file per entity)
    handler/                       # Gin HTTP handlers (one file per API group)
    repository/
      repository.go                # Repository interface (all method signatures)
      gorm/repository.go           # GORM implementation
    service/                       # Business logic services
    config/                        # Config structs + YAML loader
    db/
      db.go                        # DB connection
      migrate.go                   # AutoMigrate registration
    risk/                          # Risk management
    signal/                        # Signal collectors
    strategy/                      # Strategy evaluators
    paas/                          # PaaS audit logging client

services/polymarket/frontend/
  app/                             # Next.js 15 App Router pages
    v2/                            # V2 versioned pages
  lib/api.ts                       # API client (apiGet/apiPost/apiPut/apiDelete)
  components/                      # Shared UI components
```

## Adding a New Feature: Step-by-Step

### 1. Define Model (`internal/models/<entity>.go`)

```go
package models

import (
    "time"
    "github.com/shopspring/decimal"
    "gorm.io/datatypes"
)

type Position struct {
    ID        uint64          `gorm:"primaryKey;autoIncrement"`
    TokenID   string          `gorm:"type:varchar(100);not null;index"`
    // ... fields
    CreatedAt time.Time       `gorm:"type:timestamptz;autoCreateTime;index"`
    UpdatedAt time.Time       `gorm:"type:timestamptz;autoUpdateTime"`
}

func (Position) TableName() string { return "positions" }
```

Rules:
- `uint64` for primary keys with `autoIncrement`
- `decimal.Decimal` with `gorm:"type:numeric(30,10)"` for all monetary values (NEVER float64)
- `datatypes.JSON` with `gorm:"type:jsonb"` for nested structures
- Pointer types (`*string`, `*time.Time`) for nullable columns
- `timestamptz` for all time columns
- Add `index` tag to columns used in WHERE/ORDER BY
- Implement `TableName()` returning the snake_case table name

### 2. Register Migration (`internal/db/migrate.go`)

Add the new model to the `AutoMigrate` call list:

```go
if err := db.Gorm.AutoMigrate(
    // ... existing models ...
    &models.Position{},  // <-- append here
); err != nil {
    return err
}
```

### 3. Add Repository Interface Methods (`internal/repository/repository.go`)

Add methods to the `Repository` interface:

```go
type Repository interface {
    CatalogRepository
    // ... existing methods ...

    // L7: positions
    UpsertPosition(ctx context.Context, item *models.Position) error
    GetPositionByTokenID(ctx context.Context, tokenID string) (*models.Position, error)
    ListPositions(ctx context.Context, params ListPositionsParams) ([]models.Position, error)
    CountPositions(ctx context.Context, params ListPositionsParams) (int64, error)
}
```

Add the corresponding params struct:

```go
type ListPositionsParams struct {
    Limit   int
    Offset  int
    Status  *string
    OrderBy string
    Asc     *bool
}
```

### 4. Implement Repository (`internal/repository/gorm/repository.go`)

Follow these patterns exactly:

```go
// Get by ID: return nil, nil on not found (NOT an error)
func (s *Store) GetPositionByTokenID(ctx context.Context, tokenID string) (*models.Position, error) {
    if s == nil || s.db == nil { return nil, nil }
    var item models.Position
    err := s.db.WithContext(ctx).Where("token_id = ?", tokenID).First(&item).Error
    if err == gorm.ErrRecordNotFound { return nil, nil }
    if err != nil { return nil, err }
    return &item, nil
}

// List with filters: apply optional params, normalize limit/offset
func (s *Store) ListPositions(ctx context.Context, params repository.ListPositionsParams) ([]models.Position, error) {
    if s == nil || s.db == nil { return nil, nil }
    query := s.db.WithContext(ctx).Model(&models.Position{})
    if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
        query = query.Where("status = ?", strings.TrimSpace(*params.Status))
    }
    query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
    var items []models.Position
    if err := query.Limit(normalizeLimit(params.Limit, 200)).Offset(normalizeOffset(params.Offset)).Find(&items).Error; err != nil {
        return nil, err
    }
    return items, nil
}

// Upsert with ON CONFLICT
func (s *Store) UpsertPosition(ctx context.Context, item *models.Position) error {
    if s == nil || s.db == nil || item == nil { return nil }
    return s.db.WithContext(ctx).Clauses(clause.OnConflict{
        Columns:   []clause.Column{{Name: "token_id"}},
        DoUpdates: clause.AssignmentColumns([]string{"quantity", "avg_entry_price", "current_price", "unrealized_pnl", "status", "updated_at"}),
    }).Create(item).Error
}
```

### 5. Create Handler (`internal/handler/v2_<entity>.go`)

```go
package handler

type V2PositionHandler struct {
    Repo repository.Repository
}

func (h *V2PositionHandler) Register(r *gin.Engine) {
    g := r.Group("/api/v2/positions")
    g.GET("", h.list)
    g.GET("/summary", h.summary)
    g.GET("/:id", h.get)
}

func (h *V2PositionHandler) list(c *gin.Context) {
    if h.Repo == nil {
        Error(c, http.StatusInternalServerError, "repo unavailable", nil)
        return
    }
    // parse query params ...
    items, err := h.Repo.ListPositions(c.Request.Context(), params)
    if err != nil {
        Error(c, http.StatusBadGateway, err.Error(), nil)
        return
    }
    total, err := h.Repo.CountPositions(c.Request.Context(), params)
    if err != nil {
        Error(c, http.StatusBadGateway, err.Error(), nil)
        return
    }
    Ok(c, items, paginationMeta(limit, offset, total))
}
```

Rules:
- Handler struct holds `Repo repository.Repository` (and optionally `Risk`, `Logger`)
- `Register(r *gin.Engine)` groups routes under `/api/v2/<entity>`
- Always check `h.Repo == nil` at method start
- Use `c.Request.Context()` for all repo calls
- Parse query params with helpers: `intQuery(c, "limit", 50)`, `c.Query("status")`
- Request bodies: define a private struct, bind with `c.ShouldBindJSON(&req)`
- Responses: `Ok(c, data, meta)` for success, `Error(c, statusCode, msg, details)` for errors
- Use `paas.LogBestEffort()` for audit logging on write operations

### 6. Wire in main.go (`cmd/monitor/main.go`)

```go
// In the handler registration section:
v2Positions := &handler.V2PositionHandler{Repo: store}
v2Positions.Register(engine)
```

### 7. Add Frontend Page (`app/v2/<entity>/page.tsx`)

Follow the existing page pattern:
- Use `"use client"` directive
- Fetch data with `apiGet<T>("/api/v2/...")` from `lib/api.ts`
- Use `useState` for local state, `useEffect` for initial load, `useCallback` for refresh
- Table layout with glass-panel styling (see existing pages for CSS classes)
- Add navigation link in `app/layout.tsx`

## Key Libraries

- **Go**: `gin` (HTTP), `gorm` (ORM), `shopspring/decimal` (money), `datatypes` (JSONB), `zap` (logging)
- **Frontend**: Next.js 15, React 18, Tailwind CSS 4, TypeScript

## Response Format

All API responses follow this envelope:

```json
// Success
{ "data": <payload>, "meta": { "limit": 50, "offset": 0, "total": 123 } }

// Error
{ "error": "message", "details": null }
```

## Important Rules

1. NEVER use `float64` for prices, sizes, PnL, or any monetary value. Use `decimal.Decimal`.
2. NEVER return an error for "not found" in repository. Return `nil, nil`.
3. ALWAYS add defensive nil checks at the start of every repository and handler method.
4. ALWAYS pass `context.Context` through all layers.
5. ALWAYS add new models to `AutoMigrate` in `internal/db/migrate.go`.
6. ALWAYS add new repository methods to both the interface and the GORM implementation.
