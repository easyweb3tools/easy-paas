package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"polymarket/internal/client/polymarket/clob"
	polymarketgamma "polymarket/internal/client/polymarket/gamma"
	"polymarket/internal/config"
	cronrunner "polymarket/internal/cron"
	"polymarket/internal/db"
	"polymarket/internal/handler"
	"polymarket/internal/labeler"
	"polymarket/internal/logger"
	"polymarket/internal/opportunity"
	"polymarket/internal/paas"
	gormrepository "polymarket/internal/repository/gorm"
	"polymarket/internal/risk"
	"polymarket/internal/service"
	signalhub "polymarket/internal/signal"
	"polymarket/internal/strategy"

	_ "polymarket/docs"
)

func main() {
	cfgPath := os.Getenv("PM_CONFIG")
	if cfgPath == "" {
		cfgPath = "config/config.yaml"
	}

	envOnly := false
	if envOnlyRaw := os.Getenv("PM_ENV_ONLY"); envOnlyRaw != "" {
		envOnly = strings.EqualFold(envOnlyRaw, "true") || envOnlyRaw == "1"
	}

	cfg, err := config.Load(cfgPath, envOnly)
	if err != nil {
		panic(err)
	}

	logger, err := logger.New(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	dbConn, err := db.Open(cfg.DB)
	if err != nil {
		logger.Fatal("db open failed", zap.Error(err))
	}
	defer db.Close(dbConn)

	if err := db.SetTimezone(dbConn, cfg.DB.Timezone); err != nil {
		logger.Warn("failed to set timezone", zap.Error(err))
	}
	if err := db.AutoMigrate(dbConn); err != nil {
		logger.Fatal("auto-migrate failed", zap.Error(err))
	}

	gammaHTTP := &http.Client{Timeout: cfg.Gamma.Timeout}
	gammaClient := polymarketgamma.NewClientWithHost(gammaHTTP, cfg.Gamma.BaseURL)
	clobHTTP := &http.Client{Timeout: cfg.ClobREST.Timeout}
	clobClient := clob.NewClient(clobHTTP, cfg.ClobREST.BaseURL)
	store := gormrepository.New(dbConn.Gorm)
	settingsSvc := &service.SystemSettingsService{Repo: store}
	if err := settingsSvc.EnsureDefaultSwitches(context.Background()); err != nil {
		logger.Warn("init default system switches failed", zap.Error(err))
	}
	catalogService := &service.CatalogSyncService{
		Store:  store,
		Gamma:  gammaClient,
		Clob:   clobClient,
		Logger: logger,
	}
	queryService := &service.CatalogQueryService{Repo: store}
	streamService := &service.CLOBStreamService{Repo: store, Logger: logger}

	var marketLabeler *labeler.MarketLabeler
	marketLabeler = &labeler.MarketLabeler{
		Repo:   store,
		Logger: logger,
	}

	if strings.EqualFold(cfg.App.Env, "dev") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	paasClient := initPaaSClient(logger)
	engine.Use(paas.RequireBearerMiddleware())
	engine.Use(paas.InjectClientMiddleware(paasClient))
	engine.Use(paas.PaaSWriteAuditMiddleware(paasClient, logger))

	healthHandler := &handler.HealthHandler{DB: dbConn.Gorm}
	healthHandler.Register(engine)
	paas.RegisterDocs(engine)
	catalogHandler := &handler.CatalogHandler{
		Service:      catalogService,
		QueryService: queryService,
		Logger:       logger,
	}
	catalogHandler.Register(engine)

	// V2 API (read-mostly skeleton; strategy engine wiring is added in later phases).
	v2Signals := &handler.V2SignalHandler{Repo: store}
	v2Signals.Register(engine)
	v2Strategies := &handler.V2StrategyHandler{Repo: store}
	v2Strategies.Register(engine)
	riskMgr := &risk.Manager{Config: cfg.Risk, Repo: store, Logger: logger}
	v2Opps := &handler.V2OpportunityHandler{Repo: store, Risk: riskMgr}
	v2Opps.Register(engine)
	v2Labels := &handler.V2LabelHandler{Repo: store, Labeler: marketLabeler}
	v2Labels.Register(engine)
	journalSvc := &service.JournalService{Repo: store}
	positionSyncSvc := &service.PositionSyncService{Repo: store, Logger: logger, Flags: settingsSvc}
	execMode := "live"
	if cfg.AutoExecutor.DryRun {
		execMode = "dry-run"
	}
	clobExecutor := &service.CLOBExecutor{
		Repo:         store,
		Risk:         riskMgr,
		Logger:       logger,
		PositionSync: positionSyncSvc,
		Client:       clobClient,
		Config: service.ExecutorConfig{
			Mode:                 execMode,
			MaxOrderSizeUSD:      decimal.Zero,
			SlippageToleranceBps: 200,
		},
	}
	v2Positions := &handler.V2PositionHandler{Repo: store}
	v2Positions.Register(engine)
	v2Exec := &handler.V2ExecutionHandler{Repo: store, Risk: riskMgr}
	v2Exec.Journal = journalSvc
	v2Exec.PositionSync = positionSyncSvc
	v2Exec.Register(engine)
	v2Analytics := &handler.V2AnalyticsHandler{Repo: store}
	v2Analytics.Register(engine)
	v2Review := &handler.V2ReviewHandler{Repo: store}
	v2Review.Register(engine)
	v2Settlements := &handler.V2SettlementHandler{Repo: store}
	v2Settlements.Register(engine)
	v2Rules := &handler.V2ExecutionRuleHandler{Repo: store}
	v2Rules.Register(engine)
	v2Orders := &handler.V2OrderHandler{Repo: store, Executor: clobExecutor}
	v2Orders.Register(engine)
	v2Journal := &handler.V2JournalHandler{Repo: store}
	v2Journal.Register(engine)
	v2Settings := &handler.V2SystemSettingsHandler{Repo: store, Settings: settingsSvc}
	v2Settings.Register(engine)
	v2Pipeline := &handler.V2PipelineHandler{Repo: store}
	v2Pipeline.Register(engine)

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	srv := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: engine,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	baseCtx := ctx
	if paasClient != nil {
		baseCtx = paas.WithClient(ctx, paasClient)
	}

	cronRunner := cronrunner.New(logger, baseCtx)
	scope := cfg.CatalogSync.Scope
	limit := cfg.CatalogSync.PageLimit
	maxPages := cfg.CatalogSync.MaxPages
	resume := cfg.CatalogSync.Resume
	var tagID *int
	if cfg.CatalogSync.TagID > 0 {
		tagID = &cfg.CatalogSync.TagID
	}
	closed := parseClosedFilter(cfg.CatalogSync.Closed)

	_, err = cronRunner.Add(cfg.Cron.CatalogSync, func(ctx context.Context) {
		if !settingsSvc.IsEnabled(ctx, service.FeatureCatalogSync, true) {
			return
		}
		result, err := catalogService.Sync(ctx, service.SyncOptions{
			Scope:             scope,
			Limit:             limit,
			MaxPages:          maxPages,
			Resume:            resume,
			TagID:             tagID,
			Closed:            closed,
			BookMaxAssets:     cfg.CatalogSync.BookMaxAssets,
			BookBatchSize:     cfg.CatalogSync.BookBatchSize,
			BookSleepPerBatch: cfg.CatalogSync.BookSleepPerBatch,
		})
		if err != nil {
			logger.Warn("cron catalog sync failed", zap.Error(err))
			if paasClient != nil {
				ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_ = paasClient.CreateLog(ctx2, paas.CreateLogRequest{
					Agent:  "polymarket-service",
					Action: "polymarket_cron_catalog_sync_failed",
					Level:  "warn",
					Details: map[string]any{
						"error": err.Error(),
					},
					SessionKey: "",
					Metadata:   map[string]any{},
				})
				cancel()
			}
			return
		}
		logger.Info("cron catalog sync ok",
			zap.String("scope", result.Scope),
			zap.Int("pages", result.Pages),
			zap.Int("events", result.Events),
			zap.Int("markets", result.Markets),
			zap.Int("tokens", result.Tokens),
			zap.Int("series", result.Series),
			zap.Int("tags", result.Tags),
		)
		if paasClient != nil {
			ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = paasClient.CreateLog(ctx2, paas.CreateLogRequest{
				Agent:  "polymarket-service",
				Action: "polymarket_cron_catalog_sync_ok",
				Level:  "info",
				Details: map[string]any{
					"scope":   result.Scope,
					"pages":   result.Pages,
					"events":  result.Events,
					"markets": result.Markets,
					"tokens":  result.Tokens,
					"series":  result.Series,
					"tags":    result.Tags,
				},
				SessionKey: "",
				Metadata:   map[string]any{},
			})
			cancel()
		}
	})
	if err != nil {
		logger.Warn("cron register catalog sync failed", zap.Error(err))
	}

	_, err = cronRunner.Add("@every 30s", func(ctx context.Context) {
		if err := positionSyncSvc.RefreshOpenPositionsPrices(ctx); err != nil {
			logger.Warn("position price refresh failed", zap.Error(err))
		}
	})
	if err != nil {
		logger.Warn("cron register position refresh failed", zap.Error(err))
	}

	_, err = cronRunner.Add("@every 1h", func(ctx context.Context) {
		if err := positionSyncSvc.SnapshotPortfolio(ctx); err != nil {
			logger.Warn("portfolio snapshot failed", zap.Error(err))
		}
	})
	if err != nil {
		logger.Warn("cron register portfolio snapshot failed", zap.Error(err))
	}

	_, err = cronRunner.Add("@every 5s", func(ctx context.Context) {
		if err := clobExecutor.PollOrders(ctx); err != nil {
			logger.Warn("order poll failed", zap.Error(err))
		}
	})
	if err != nil {
		logger.Warn("cron register order poll failed", zap.Error(err))
	}
	cronRunner.Start()
	defer cronRunner.Stop()

	// Prefer cron scheduling for labeler with DB switch.
	if marketLabeler != nil {
		spec := "@every " + cfg.Labeler.ScanInterval.String()
		_, err := cronRunner.Add(spec, func(ctx context.Context) {
			if !settingsSvc.IsEnabled(ctx, service.FeatureLabeler, false) {
				return
			}
			if err := marketLabeler.LabelMarkets(ctx); err != nil {
				logger.Warn("labeler run failed", zap.Error(err))
			}
		})
		if err != nil {
			logger.Warn("cron register labeler failed", zap.Error(err))
		}
	}

	if settingsSvc.IsEnabled(baseCtx, service.FeatureCLOBStream, true) {
		go func() {
			err := streamService.RunMarketStream(baseCtx, service.CLOBStreamOptions{
				URL:             cfg.ClobStream.URL,
				RefreshInterval: cfg.ClobStream.RefreshInterval,
				MaxAssets:       cfg.ClobStream.MaxAssets,
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("clob stream stopped", zap.Error(err))
			}
		}()
	}

	// Run labeler once before strategy engine so label-dependent signals
	// (no_bias, fdv_overpriced) have data from the first scan tick.
	if marketLabeler != nil {
		logger.Info("running initial label pass before strategy engine")
		if err := marketLabeler.LabelMarkets(baseCtx); err != nil {
			logger.Warn("initial label pass failed (continuing)", zap.Error(err))
		} else {
			logger.Info("initial label pass complete")
		}
	}

	// Bootstrap orderbook data via REST so strategy engine has prices on first tick.
	{
		logger.Info("running initial orderbook bootstrap before strategy engine")
		bookResult, err := catalogService.Sync(baseCtx, service.SyncOptions{
			Scope:             "books_only",
			BookMaxAssets:     cfg.CatalogSync.BookMaxAssets,
			BookBatchSize:     cfg.CatalogSync.BookBatchSize,
			BookSleepPerBatch: cfg.CatalogSync.BookSleepPerBatch,
		})
		if err != nil {
			logger.Warn("initial orderbook bootstrap failed (continuing)", zap.Error(err))
		} else {
			logger.Info("initial orderbook bootstrap complete",
				zap.Int("assets", bookResult.BookAssets),
				zap.Int("errors", bookResult.BookErrors),
			)
		}
	}

	if settingsSvc.IsEnabled(baseCtx, service.FeatureStrategyEngine, false) {
		hub := signalhub.NewHub(store, logger)
		hub.Register(&signalhub.SettlementHistoryCollector{
			Repo:       store,
			Logger:     logger,
			Interval:   30 * time.Minute,
			MinSamples: 10,
		})
		hub.Register(&signalhub.InternalScanCollector{
			Repo:     store,
			Logger:   logger,
			Interval: cfg.StrategyEngine.ScanInterval,
		})
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalWeatherAPI, false) {
			hub.Register(&signalhub.WeatherAPICollector{
				Logger:  logger,
				Cities:  cfg.SignalSources.WeatherAPI.Cities,
				Sources: cfg.SignalSources.WeatherAPI.Sources,
			})
		}
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalBinanceWS, false) {
			hub.Register(&signalhub.BinanceDepthCollector{
				Logger: logger,
				URL:    cfg.SignalSources.BinanceWS.URL,
				Symbol: cfg.SignalSources.BinanceWS.Symbol,
			})
		}
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalBinancePrice, false) {
			hub.Register(&signalhub.BinancePriceCollector{
				Logger:        logger,
				Endpoint:      cfg.SignalSources.BinancePrice.Endpoint,
				PollInterval:  cfg.SignalSources.BinancePrice.PollInterval,
				WindowSeconds: cfg.SignalSources.BinancePrice.WindowSeconds,
				TriggerPct:    cfg.SignalSources.BinancePrice.TriggerPct,
			})
		}
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalPriceChange, false) {
			hub.Register(&signalhub.PriceChangeCollector{
				Repo:   store,
				Logger: logger,
				Config: cfg.SignalSources.PriceChange,
			})
		}
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalOrderbook, false) {
			hub.Register(&signalhub.OrderbookPatternCollector{
				Repo:   store,
				Logger: logger,
				Config: cfg.SignalSources.Orderbook,
			})
		}
		if settingsSvc.IsEnabled(baseCtx, service.FeatureSignalCertainty, false) {
			hub.Register(&signalhub.CertaintySweepCollector{
				Repo:   store,
				Logger: logger,
				Config: cfg.SignalSources.Certainty,
			})
		}
		stratEngine := &strategy.Engine{
			Repo:             store,
			Hub:              hub,
			Logger:           logger,
			Risk:             riskMgr,
			Opps:             &opportunity.Manager{Repo: store, Logger: logger, MaxActive: cfg.StrategyEngine.MaxOpportunities},
			StrategyDefaults: cfg.StrategyDefaults,
			Evaluators: []strategy.StrategyEvaluator{
				&strategy.ArbitrageSumStrategy{Repo: store, Logger: logger},
				&strategy.SystematicNOStrategy{Repo: store, Logger: logger},
				&strategy.PreMarketFDVStrategy{Repo: store, Logger: logger},
				&strategy.NewsAlphaStrategy{Repo: store, Logger: logger},
				&strategy.VolatilityArbStrategy{Repo: store, Logger: logger},
				&strategy.WeatherStrategy{Repo: store, Logger: logger},
				&strategy.BTCShortTermStrategy{Repo: store, Logger: logger},
				&strategy.ContrarianFearStrategy{Repo: store, Logger: logger},
				&strategy.MMBehaviorStrategy{Repo: store, Logger: logger},
				&strategy.CertaintySweepStrategy{Repo: store, Logger: logger},
				&strategy.LiquidityRewardStrategy{Repo: store, Logger: logger},
			&strategy.MarketAnomalyStrategy{Repo: store, Logger: logger},
			},
		}
		go func() {
			if err := hub.Run(baseCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("signal hub stopped", zap.Error(err))
			}
		}()
		go func() {
			if err := stratEngine.Run(baseCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("strategy engine stopped", zap.Error(err))
			}
		}()
		go func() {
			updater := &strategy.StatsUpdater{
				Repo:     store,
				Logger:   logger,
				Interval: 5 * time.Minute,
			}
			if err := updater.Run(baseCtx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("strategy stats updater stopped", zap.Error(err))
			}
		}()

		// Periodic cleanup: remove expired signals to prevent unbounded growth.
		_, err := cronRunner.Add("@every 10m", func(ctx context.Context) {
			n, err := store.DeleteExpiredSignals(ctx, time.Now().UTC())
			if err != nil {
				logger.Warn("delete expired signals failed", zap.Error(err))
				return
			}
			if n > 0 {
				logger.Info("deleted expired signals", zap.Int64("count", n))
			}
		})
		if err != nil {
			logger.Warn("cron register signal cleanup failed", zap.Error(err))
		}
	}

	ingestor := &service.SettlementIngestService{
		Repo:   store,
		Gamma:  gammaClient,
		Config: cfg.SettlementIngest,
		Logger: logger,
		Flags:  settingsSvc,
	}
	go func() {
		if err := ingestor.Run(baseCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("settlement ingestor stopped", zap.Error(err))
		}
	}()

	auto := &service.AutoExecutorService{
		Repo:     store,
		Risk:     riskMgr,
		Logger:   logger,
		Config:   cfg.AutoExecutor,
		Flags:    settingsSvc,
		Executor: clobExecutor,
	}
	go func() {
		if err := auto.Run(baseCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("auto executor stopped", zap.Error(err))
		}
	}()

	positionManager := &service.PositionManager{
		Repo:   store,
		Logger: logger,
		Flags:  settingsSvc,
	}
	go func() {
		if err := positionManager.Run(baseCtx, 30*time.Second); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("position manager stopped", zap.Error(err))
		}
	}()

	dailyStats := &service.DailyStatsService{
		Repo:   store,
		Logger: logger,
		Flags:  settingsSvc,
	}
	go func() {
		if err := dailyStats.Run(baseCtx, 6*time.Hour); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("daily stats service stopped", zap.Error(err))
		}
	}()

	reviewSvc := &service.ReviewService{
		Repo:   store,
		Logger: logger,
		Flags:  settingsSvc,
	}
	go func() {
		if err := reviewSvc.Run(baseCtx, 6*time.Hour); err != nil && !errors.Is(err, context.Canceled) {
			logger.Warn("review service stopped", zap.Error(err))
		}
	}()

	errCh := make(chan error, 2)

	go func() {
		logger.Info("http server starting", zap.String("addr", cfg.Server.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		logger.Error("server error", zap.Error(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func parseClosedFilter(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open":
		v := false
		return &v
	case "closed":
		v := true
		return &v
	default:
		return nil
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func initPaaSClient(logger *zap.Logger) *paas.Client {
	base := strings.TrimSpace(os.Getenv("EASYWEB3_API_BASE"))
	apiKey := strings.TrimSpace(os.Getenv("EASYWEB3_API_KEY"))
	if base == "" || apiKey == "" {
		return nil
	}

	p := &paas.Client{BaseURL: base, APIKey: apiKey}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := p.Login(ctx); err != nil {
		if logger != nil {
			logger.Warn("paas login failed (logs/notify disabled)", zap.Error(err))
		}
		return nil
	}
	if logger != nil {
		logger.Info("paas login ok")
	}
	return p
}
