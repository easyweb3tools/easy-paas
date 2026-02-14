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
	); err != nil {
		return err
	}
	if db.Gorm.Migrator().HasColumn(&models.Market{}, "stream_enabled") {
		if err := db.Gorm.Migrator().DropColumn(&models.Market{}, "stream_enabled"); err != nil {
			return err
		}
	}
	return nil
}
