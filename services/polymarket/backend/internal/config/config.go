package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Server      ServerConfig      `mapstructure:"server"`
	Log         LogConfig         `mapstructure:"log"`
	DB          DBConfig          `mapstructure:"db"`
	Cron        CronConfig        `mapstructure:"cron"`
	Gamma       GammaConfig       `mapstructure:"gamma"`
	CatalogSync CatalogSyncConfig `mapstructure:"catalog_sync"`
	ClobStream  ClobStreamConfig  `mapstructure:"clob_stream"`
	ClobREST    ClobRESTConfig    `mapstructure:"clob_rest"`

	// V2 extensions (L4-L6).
	StrategyEngine   StrategyEngineConfig   `mapstructure:"strategy_engine"`
	SignalSources    SignalSourcesConfig    `mapstructure:"signal_sources"`
	Risk             RiskConfig             `mapstructure:"risk"`
	Labeler          LabelerConfig          `mapstructure:"labeler"`
	SettlementIngest SettlementIngestConfig `mapstructure:"settlement_ingest"`
	AutoExecutor     AutoExecutorConfig     `mapstructure:"auto_executor"`
	StrategyDefaults map[string]any         `mapstructure:"strategy_defaults"`
}

type AppConfig struct {
	Env string `mapstructure:"env"`
}

type ServerConfig struct {
	HTTPAddr string `mapstructure:"http_addr"`
}

type LogConfig struct {
	Level             string `mapstructure:"level"`
	Encoding          string `mapstructure:"encoding"`
	Development       bool   `mapstructure:"development"`
	Sampling          bool   `mapstructure:"sampling"`
	DisableCaller     bool   `mapstructure:"disable_caller"`
	DisableStacktrace bool   `mapstructure:"disable_stacktrace"`
}

type DBConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	Timezone        string        `mapstructure:"timezone"`
}

type CronConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	CatalogSync string `mapstructure:"catalog_sync"`
}

type GammaConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type CatalogSyncConfig struct {
	Scope             string        `mapstructure:"scope"`
	PageLimit         int           `mapstructure:"page_limit"`
	MaxPages          int           `mapstructure:"max_pages"`
	Resume            bool          `mapstructure:"resume"`
	TagID             int           `mapstructure:"tag_id"`
	Closed            string        `mapstructure:"closed"`
	BookMaxAssets     int           `mapstructure:"book_max_assets"`
	BookBatchSize     int           `mapstructure:"book_batch_size"`
	BookSleepPerBatch time.Duration `mapstructure:"book_sleep_per_batch"`
}

type ClobStreamConfig struct {
	URL             string        `mapstructure:"url"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	MaxAssets       int           `mapstructure:"max_assets"`
}

type ClobRESTConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type StrategyEngineConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	ScanInterval     time.Duration `mapstructure:"scan_interval"`
	MaxOpportunities int           `mapstructure:"max_opportunities"`
}

type SignalSourcesConfig struct {
	BinanceWS    BinanceWSConfig        `mapstructure:"binance_ws"`
	BinancePrice BinancePriceConfig     `mapstructure:"binance_price"`
	WeatherAPI   WeatherAPIConfig       `mapstructure:"weather_api"`
	NewsRSS      NewsRSSConfig          `mapstructure:"news_rss"`
	PriceChange  PriceChangeConfig      `mapstructure:"price_change"`
	Orderbook    OrderbookPatternConfig `mapstructure:"orderbook_pattern"`
	Certainty    CertaintySweepConfig   `mapstructure:"certainty_sweep"`
}

type BinanceWSConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Symbol  string `mapstructure:"symbol"`
}

type BinancePriceConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Endpoint      string        `mapstructure:"endpoint"`
	PollInterval  time.Duration `mapstructure:"poll_interval"`
	WindowSeconds int           `mapstructure:"window_seconds"`
	TriggerPct    float64       `mapstructure:"trigger_pct"`
}

type WeatherAPIConfig struct {
	Enabled bool               `mapstructure:"enabled"`
	Sources []WeatherAPISource `mapstructure:"sources"`
	Cities  []string           `mapstructure:"cities"`
}

type WeatherAPISource struct {
	Name         string        `mapstructure:"name"`
	Kind         string        `mapstructure:"kind"`
	Endpoint     string        `mapstructure:"endpoint"`
	APIKeyEnv    string        `mapstructure:"api_key_env"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	Weight       float64       `mapstructure:"weight"`
}

type NewsRSSConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	Feeds        []string      `mapstructure:"feeds"`
	Keywords     []string      `mapstructure:"keywords"`
}

type PriceChangeConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Interval     time.Duration `mapstructure:"interval"`
	MinJumpBps   float64       `mapstructure:"min_jump_bps"`
	MaxSpreadBps float64       `mapstructure:"max_spread_bps"`
	Limit        int           `mapstructure:"limit"`
}

type OrderbookPatternConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Interval     time.Duration `mapstructure:"interval"`
	MinSpreadBps float64       `mapstructure:"min_spread_bps"`
	MinJumpBps   float64       `mapstructure:"min_jump_bps"`
	Limit        int           `mapstructure:"limit"`
}

type CertaintySweepConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Interval      time.Duration `mapstructure:"interval"`
	HoursToExpiry int           `mapstructure:"hours_to_expiry"`
	Limit         int           `mapstructure:"limit"`
}

type RiskConfig struct {
	MaxTotalExposureUSD  float64 `mapstructure:"max_total_exposure_usd"`
	MaxPerMarketUSD      float64 `mapstructure:"max_per_market_usd"`
	MaxPerStrategyUSD    float64 `mapstructure:"max_per_strategy_usd"`
	MaxDailyLossUSD      float64 `mapstructure:"max_daily_loss_usd"`
	KellyFractionCap     float64 `mapstructure:"kelly_fraction_cap"`
	DefaultKellyFraction float64 `mapstructure:"default_kelly_fraction"`
	MinDataFreshnessMs   int     `mapstructure:"min_data_freshness_ms"`
	StaleDataAction      string  `mapstructure:"stale_data_action"`
	RequirePreflightPass bool    `mapstructure:"require_preflight_pass"`
}

type LabelerConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	ScanInterval time.Duration `mapstructure:"scan_interval"`
}

type SettlementIngestConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	ScanInterval time.Duration `mapstructure:"scan_interval"`
	LookbackDays int           `mapstructure:"lookback_days"`
	BatchSize    int           `mapstructure:"batch_size"`
}

type AutoExecutorConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	ScanInterval         time.Duration `mapstructure:"scan_interval"`
	MaxOpportunities     int           `mapstructure:"max_opportunities"`
	DefaultMinConfidence float64       `mapstructure:"default_min_confidence"`
	DefaultMinEdgePct    float64       `mapstructure:"default_min_edge_pct"`
	DryRun               bool          `mapstructure:"dry_run"`
}

func Load(path string, envOnly bool) (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("PM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetDefault("app.env", "dev")
	v.SetDefault("server.http_addr", ":8080")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "console")
	v.SetDefault("log.development", true)
	v.SetDefault("log.sampling", false)
	v.SetDefault("log.disable_caller", false)
	v.SetDefault("log.disable_stacktrace", false)
	v.SetDefault("db.max_open_conns", 20)
	v.SetDefault("db.max_idle_conns", 5)
	v.SetDefault("db.conn_max_lifetime", "30m")
	v.SetDefault("db.conn_max_idle_time", "5m")
	v.SetDefault("db.timezone", "UTC")
	v.SetDefault("cron.enabled", true)
	v.SetDefault("cron.catalog_sync", "@every 10m")
	v.SetDefault("gamma.base_url", "https://gamma-api.polymarket.com")
	v.SetDefault("gamma.timeout", "15s")
	v.SetDefault("catalog_sync.enabled", true)
	v.SetDefault("catalog_sync.scope", "all")
	v.SetDefault("catalog_sync.page_limit", 200)
	v.SetDefault("catalog_sync.max_pages", 5)
	v.SetDefault("catalog_sync.resume", true)
	v.SetDefault("catalog_sync.tag_id", 0)
	v.SetDefault("catalog_sync.closed", "open")
	v.SetDefault("catalog_sync.book_max_assets", 200)
	v.SetDefault("catalog_sync.book_batch_size", 20)
	v.SetDefault("catalog_sync.book_sleep_per_batch", "3s")
	v.SetDefault("clob_stream.url", "")
	v.SetDefault("clob_stream.refresh_interval", "30s")
	v.SetDefault("clob_stream.max_assets", 200)
	v.SetDefault("clob_rest.base_url", "https://clob.polymarket.com")
	v.SetDefault("clob_rest.timeout", "15s")

	// V2 defaults: keep disabled by default to avoid behavior changes until engine is wired.
	v.SetDefault("strategy_engine.enabled", false)
	v.SetDefault("strategy_engine.scan_interval", "5s")
	v.SetDefault("strategy_engine.max_opportunities", 100)

	v.SetDefault("signal_sources.binance_ws.enabled", false)
	v.SetDefault("signal_sources.binance_ws.url", "wss://stream.binance.com:9443/ws/btcusdt@depth20@100ms")
	v.SetDefault("signal_sources.binance_ws.symbol", "BTCUSDT")

	v.SetDefault("signal_sources.binance_price.enabled", false)
	v.SetDefault("signal_sources.binance_price.endpoint", "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT")
	v.SetDefault("signal_sources.binance_price.poll_interval", "2s")
	v.SetDefault("signal_sources.binance_price.window_seconds", 300)
	v.SetDefault("signal_sources.binance_price.trigger_pct", 2.0)

	v.SetDefault("signal_sources.weather_api.enabled", false)
	v.SetDefault("signal_sources.news_rss.enabled", false)
	v.SetDefault("signal_sources.news_rss.poll_interval", "2m")
	v.SetDefault("settlement_ingest.enabled", false)
	v.SetDefault("settlement_ingest.scan_interval", "6h")
	v.SetDefault("settlement_ingest.lookback_days", 14)
	v.SetDefault("settlement_ingest.batch_size", 200)
	v.SetDefault("auto_executor.enabled", false)
	v.SetDefault("auto_executor.scan_interval", "10s")
	v.SetDefault("auto_executor.max_opportunities", 100)
	v.SetDefault("auto_executor.default_min_confidence", 0.8)
	v.SetDefault("auto_executor.default_min_edge_pct", 0.05)
	v.SetDefault("auto_executor.dry_run", true)

	v.SetDefault("signal_sources.price_change.enabled", false)
	v.SetDefault("signal_sources.price_change.interval", "5s")
	v.SetDefault("signal_sources.price_change.min_jump_bps", 500)
	v.SetDefault("signal_sources.price_change.max_spread_bps", 400)
	v.SetDefault("signal_sources.price_change.limit", 50)

	v.SetDefault("signal_sources.orderbook_pattern.enabled", false)
	v.SetDefault("signal_sources.orderbook_pattern.interval", "10s")
	v.SetDefault("signal_sources.orderbook_pattern.min_spread_bps", 400)
	v.SetDefault("signal_sources.orderbook_pattern.min_jump_bps", 600)
	v.SetDefault("signal_sources.orderbook_pattern.limit", 100)

	v.SetDefault("signal_sources.certainty_sweep.enabled", false)
	v.SetDefault("signal_sources.certainty_sweep.interval", "30s")
	v.SetDefault("signal_sources.certainty_sweep.hours_to_expiry", 6)
	v.SetDefault("signal_sources.certainty_sweep.limit", 50)

	v.SetDefault("risk.max_total_exposure_usd", 5000)
	v.SetDefault("risk.max_per_market_usd", 500)
	v.SetDefault("risk.max_per_strategy_usd", 2000)
	v.SetDefault("risk.max_daily_loss_usd", 500)
	v.SetDefault("risk.kelly_fraction_cap", 0.06)
	v.SetDefault("risk.default_kelly_fraction", 0.06)
	v.SetDefault("risk.min_data_freshness_ms", 5000)
	v.SetDefault("risk.stale_data_action", "warn")
	v.SetDefault("risk.require_preflight_pass", false)

	v.SetDefault("labeler.enabled", false)
	v.SetDefault("labeler.scan_interval", "5m")

	if !envOnly {
		if err := v.ReadInConfig(); err != nil {
			return Config{}, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
