package db

import (
	"polymarket/internal/models"
)

func AutoMigrate(db *DB) error {
	if db == nil || db.Gorm == nil || db.SQL == nil {
		return nil
	}

	if err := db.Gorm.AutoMigrate(
		&models.Series{},
		&models.Event{},
		&models.Market{},
		&models.Token{},
		&models.Tag{},
		&models.EventTag{},
		&models.SyncState{},
		&models.OrderbookLatest{},
		&models.MarketDataHealth{},
		&models.LastTradePrice{},
		&models.RawWSEvent{},
		&models.RawRESTSnapshot{},
		// L4-L6 (V2)
		&models.Signal{},
		&models.SignalSource{},
		&models.Strategy{},
		&models.Opportunity{},
		&models.MarketLabel{},
		&models.ExecutionPlan{},
		&models.Fill{},
		&models.PnLRecord{},
		&models.MarketSettlementHistory{},
		&models.ExecutionRule{},
		&models.TradeJournal{},
		&models.SystemSetting{},
		&models.Position{},
		&models.PortfolioSnapshot{},
		&models.Order{},
		&models.StrategyDailyStats{},
		&models.MarketReview{},
	); err != nil {
		return err
	}

	// Backward-compatible renames:
	// GORM's default naming splits "PnL" into "pn_l". We want stable column names:
	// - realized_pnl
	// - realized_roi
	//
	// If the DB was created before we pinned explicit column names, rename columns.
	if db.Gorm.Migrator().HasColumn(&models.PnLRecord{}, "realized_pn_l") && !db.Gorm.Migrator().HasColumn(&models.PnLRecord{}, "realized_pnl") {
		if err := db.Gorm.Migrator().RenameColumn(&models.PnLRecord{}, "realized_pn_l", "realized_pnl"); err != nil {
			return err
		}
	}
	if db.Gorm.Migrator().HasColumn(&models.PnLRecord{}, "realized_ro_i") && !db.Gorm.Migrator().HasColumn(&models.PnLRecord{}, "realized_roi") {
		if err := db.Gorm.Migrator().RenameColumn(&models.PnLRecord{}, "realized_ro_i", "realized_roi"); err != nil {
			return err
		}
	}

	if db.Gorm.Migrator().HasColumn(&models.Market{}, "stream_enabled") {
		if err := db.Gorm.Migrator().DropColumn(&models.Market{}, "stream_enabled"); err != nil {
			return err
		}
	}
	return nil
}
