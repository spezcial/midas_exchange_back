package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/config"
	"github.com/caspianex/exchange-backend/pkg/cryptogate"
	"github.com/caspianex/exchange-backend/pkg/database"
	"github.com/caspianex/exchange-backend/pkg/email"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/caspianex/exchange-backend/pkg/worker"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Server.Env)
	log.Info("Starting CaspianEx OTC Exchange Backend", "env", cfg.Server.Env)

	db, err := database.NewPostgres(cfg.GetDSN())
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("Connected to database")

	// Initialize in-memory cache
	memCache := cache.NewMemoryCache(cache.NoExpiration, cache.NoExpiration) // No TTL

	cacheService := cache.NewCacheService(memCache, log)

	log.Info("Initialized cache service with write workers")

	// Initialize cache loader and warm up cache
	cacheLoader := cache.NewCacheLoader(db, cacheService, log)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := cacheLoader.WarmUpCache(ctx); err != nil {
		log.Error("Failed to warm up cache", "error", err)
		cancel()
		os.Exit(1)
	}
	cancel()

	jwtManager := auth.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessTokenExpiry,
		cfg.JWT.RefreshTokenExpiry,
	)

	emailService := email.NewEmailService(
		cfg.Email.SMTPHost,
		cfg.Email.SMTPPort,
		cfg.Email.SMTPUsername,
		cfg.Email.SMTPPassword,
		cfg.Email.SMTPFrom,
	)

	// Initialize repositories with cache service
	userRepo := repository.NewUserRepository(db, cacheService)
	walletRepo := repository.NewWalletRepository(db, cacheService)
	exchangeRepo := repository.NewCurrencyExchangeRepository(db, cacheService)
	txRepo := repository.NewTransactionRepository(db)
	exchangeRateRepo := repository.NewExchangeRateRepository(db, cacheService)
	oauthRepo := repository.NewOAuthRepository(db)
	addrRepo := repository.NewDepositAddressRepository(db)
	feeRepo := repository.NewPlatformFeeRepository(db)

	// Initialize services
	feeService := service.NewPlatformFeeService(feeRepo, log)
	authService := service.NewAuthService(userRepo, walletRepo, jwtManager, emailService, cfg.App.BcryptCost, log)
	userService := service.NewUserService(userRepo, walletRepo, cfg.App.BcryptCost)
	walletService := service.NewWalletService(walletRepo, txRepo, feeService)

	// Wire up crypto-gate integration when configured
	var cgService *service.CryptoGateService
	if cfg.CryptoGate.BaseURL != "" && cfg.CryptoGate.Token != "" {
		cgClient := cryptogate.NewClient(cfg.CryptoGate.BaseURL, cfg.CryptoGate.Token, cfg.CryptoGate.Platform)
		cgService = service.NewCryptoGateService(cgClient, cfg.CryptoGate.Platform, addrRepo, walletRepo, txRepo, log)
		walletService.SetCryptoGateService(cgService)
		authService.SetCryptoGateService(cgService)
		//pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		//if err := cgClient.Ping(pingCtx); err != nil {
		//	log.Error("crypto-gate ping failed — check CRYPTO_GATE_URL and CRYPTO_GATE_TOKEN", "error", err)
		//	pingCancel()
		//	os.Exit(1)
		//}
		//pingCancel()
		log.Info("Crypto-gate integration enabled", "url", cfg.CryptoGate.BaseURL)
	} else {
		log.Info("Crypto-gate integration disabled (CRYPTO_GATE_URL/TOKEN not set)")
	}
	exchangeService := service.NewCurrencyExchangeService(exchangeRepo, walletRepo, userRepo, emailService, feeService)
	exchangeRatesService := service.NewExchangeRatesService(exchangeRateRepo, log)
	oauthService := service.NewOAuthService(oauthRepo, userRepo, walletRepo, jwtManager, emailService, &cfg.OAuth, log)

	notifService := &service.NoOpNotificationService{}
	otcRepo := repository.NewOTCRepository(db, cacheService)
	otcService := service.NewOTCService(otcRepo, userRepo, walletRepo, notifService)

	wsService := NewWebSocketService(exchangeRatesService, log, cfg.WebSocket.AllowedOrigins, cfg.WebSocket.ReadBufferSize, cfg.WebSocket.WriteBufferSize)

	// Initialize background exchange rate updater worker
	rateUpdaterConfig := worker.DefaultRateUpdaterConfig()
	/* worker.RateUpdaterConfig{
		UpdateInterval: cfg.Worker.RateUpdateInterval,
		UpdateTimeout:  cfg.Worker.RateUpdateTimeout,
		MaxRetries:     cfg.Worker.RateUpdateRetries,
		RetryBackoff:   cfg.Worker.RateRetryBackoff,
	}*/
	rateUpdater := worker.NewRateUpdater(rateUpdaterConfig, exchangeRatesService, log)

	router := setupRouter(
		cfg,
		log,
		jwtManager,
		wsService,
		authService,
		userService,
		walletService,
		exchangeService,
		exchangeRatesService,
		oauthService,
		otcService,
		feeService,
		cgService,
		rateUpdater,
	)

	// Start background workers
	backgroundCtx, backgroundCancel := context.WithCancel(context.Background())
	defer backgroundCancel()

	rateUpdater.Start(backgroundCtx)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("Server starting", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Error("Server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		log.Info("Shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Stop background workers first
		log.Info("Stopping background workers")
		rateUpdater.Stop()

		log.Info("Shutting down server")
		if err := server.Shutdown(ctx); err != nil {
			log.Error("Server shutdown error", "error", err)
			if err := server.Close(); err != nil {
				log.Error("Server close error", "error", err)
			}
		}

		log.Info("Server stopped gracefully")
	}
}
