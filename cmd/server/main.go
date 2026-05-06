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
	"github.com/caspianex/exchange-backend/pkg/telegramgw"
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

	redisClient, err := database.NewRedis(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password)
	if err != nil {
		log.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()
	log.Info("Connected to Redis", "host", cfg.Redis.Host)

	redisCache := cache.NewRedisCache(redisClient)
	cacheService := cache.NewCacheService(redisCache)

	log.Info("Initialized cache service")

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

	// ── Telegram Gateway ────────────────────────────────────────────────────────
	var tgGateway telegramgw.Gateway
	if cfg.TelegramGW.APIToken != "" {
		tgGateway = telegramgw.NewHTTPGateway(cfg.TelegramGW.APIToken, cfg.TelegramGW.BaseURL)
		log.Info("Telegram Gateway integration enabled")
	} else {
		tgGateway = &telegramgw.NoOpGateway{}
		log.Info("Telegram Gateway not configured — 2FA via Telegram disabled (NoOp)")
	}

	// ── Repositories ───────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepository(db, cacheService)
	walletRepo := repository.NewWalletRepository(db, cacheService)
	exchangeRepo := repository.NewCurrencyExchangeRepository(db, cacheService)
	txRepo := repository.NewTransactionRepository(db)
	exchangeRateRepo := repository.NewExchangeRateRepository(db, cacheService)
	oauthRepo := repository.NewOAuthRepository(db)
	addrRepo := repository.NewDepositAddressRepository(db)
	feeRepo := repository.NewPlatformFeeRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	fingerprintRepo := repository.NewFingerprintRepository(db)

	// ── 2FA & WebAuthn services ────────────────────────────────────────────────
	twofaService := service.NewTwoFactorService(redisCache, tgGateway, log)

	var webAuthnService service.WebAuthnService
	if cfg.WebAuthn.RPID != "" {
		webAuthnService, err = service.NewWebAuthnService(
			cfg.WebAuthn.RPID,
			cfg.WebAuthn.RPDisplayName,
			cfg.WebAuthn.RPOrigins,
			passkeyRepo,
			redisCache,
			log,
		)
		if err != nil {
			log.Error("Failed to init WebAuthn service", "error", err)
			os.Exit(1)
		}
		log.Info("WebAuthn service enabled", "rp_id", cfg.WebAuthn.RPID)
	} else {
		webAuthnService = service.NewNoOpWebAuthnService()
		log.Info("WebAuthn not configured — passkey 2FA disabled (NoOp)")
	}

	// ── Business services ──────────────────────────────────────────────────────
	feeService := service.NewPlatformFeeService(feeRepo, log)
	authService := service.NewAuthService(
		userRepo, walletRepo, passkeyRepo, fingerprintRepo,
		jwtManager, emailService, twofaService, cfg.App.BcryptCost, log,
	)
	userService := service.NewUserService(userRepo, walletRepo, cfg.App.BcryptCost)
	walletService := service.NewWalletService(walletRepo, txRepo, feeService, twofaService)

	// Wire up crypto-gate
	var cgService *service.CryptoGateService
	if cfg.CryptoGate.BaseURL != "" && cfg.CryptoGate.Token != "" {
		// Fail-fast: webhook secret must not be empty in production. This is a
		// belt-and-suspenders guard alongside config.validate() — without it,
		// CryptoGateHandler.verifySecret() would reject every webhook (fail-closed),
		// so it is better to refuse to start than to silently break deposits.
		if cfg.Server.Env == "production" && cfg.CryptoGate.WebhookSecret == "" {
			log.Error("webhook secret must not be empty in production",
				"env", cfg.Server.Env,
				"hint", "set CRYPTO_GATE_WEBHOOK_SECRET")
			os.Exit(1)
		}

		cgClient := cryptogate.NewClient(cfg.CryptoGate.BaseURL, cfg.CryptoGate.Token, cfg.CryptoGate.Platform)
		cgService = service.NewCryptoGateService(cgClient, cfg.CryptoGate.Platform, addrRepo, walletRepo, txRepo, userRepo, twofaService, log)
		walletService.SetCryptoGateService(cgService)
		authService.SetCryptoGateService(cgService)
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

	rateUpdaterConfig := worker.DefaultRateUpdaterConfig()
	rateUpdater := worker.NewRateUpdater(rateUpdaterConfig, exchangeRatesService, log)

	router := setupRouter(
		cfg,
		log,
		jwtManager,
		redisCache,
		wsService,
		authService,
		userService,
		walletService,
		exchangeService,
		exchangeRatesService,
		oauthService,
		otcService,
		feeService,
		twofaService,
		webAuthnService,
		cgService,
		rateUpdater,
	)

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
