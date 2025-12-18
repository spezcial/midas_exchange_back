package main

import (
	"net/http"

	"github.com/caspianex/exchange-backend/internal/api/admin"
	"github.com/caspianex/exchange-backend/internal/api/client"
	"github.com/caspianex/exchange-backend/internal/api/health"
	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/config"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/caspianex/exchange-backend/pkg/worker"
	"github.com/go-chi/chi/v5"
)

func setupRouter(
	cfg *config.Config,
	log *logger.Logger,
	jwtManager *auth.JWTManager,
	wsService *WebSocketService,
	authService *service.AuthService,
	userService *service.UserService,
	walletService *service.WalletService,
	exchangeService *service.CurrencyExchangeService,
	exchangeRateService *service.ExchangeRatesService,
	rateUpdater *worker.RateUpdater,
) http.Handler {
	r := chi.NewRouter()

	r.Get("/ws", wsService.handler)

	// Health check endpoints
	healthHandler := health.NewHealthHandler(rateUpdater)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/detailed", healthHandler.HealthDetailed)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	r.Mount("/api/v1", apiV1(cfg, log, jwtManager, authService, userService, walletService, exchangeService, exchangeRateService))

	return r
}

func apiV1(
	cfg *config.Config,
	log *logger.Logger,
	jwtManager *auth.JWTManager,
	authService *service.AuthService,
	userService *service.UserService,
	walletService *service.WalletService,
	exchangeService *service.CurrencyExchangeService,
	exchangeRateService *service.ExchangeRatesService,
) chi.Router {
	r := chi.NewRouter()

	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	// 🔹 All middlewares are defined BEFORE routes on this subrouter
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS(cfg.App.CORSAllowedOrigins))

	// Public authentication endpoints
	authHandler := client.NewAuthHandler(authService)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/refresh", authHandler.RefreshToken)
	r.Post("/auth/logout", authHandler.Logout)

	// Public exchange rates endpoint
	exchangePairHandler := client.NewExchangePairHandler(exchangeRateService)
	r.Get("/exchange-rates", exchangePairHandler.GetActiveRates)

	// Protected client endpoints
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		walletHandler := client.NewWalletHandler(walletService)
		r.Get("/wallet/currencies", walletHandler.GetAllCurrencies)
		r.Get("/wallets", walletHandler.GetWallets)
		r.Post("/wallets/deposit", walletHandler.Deposit)
		r.Post("/wallets/withdraw", walletHandler.Withdraw)
		r.Get("/transactions", walletHandler.GetTransactions)

		exchangeHandler := client.NewExchangeHandler(exchangeService)
		r.Post("/exchanges", exchangeHandler.CreateExchange)
		r.Get("/exchanges", exchangeHandler.GetExchanges)
		r.Get("/exchanges/{id}", exchangeHandler.GetExchange)
		r.Delete("/exchanges/{id}", exchangeHandler.CancelExchange)
	})

	// Admin endpoints
	r.Route("/admin", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireRole("admin"))

		userHandler := admin.NewUserHandler(userService)
		r.Get("/users", userHandler.ListUsers)
		r.Get("/users/{id}", userHandler.GetUser)

		exchangeHandler := admin.NewExchangeHandler(exchangeService)
		r.Get("/exchanges", exchangeHandler.ListExchanges)
		r.Get("/exchanges/{id}", exchangeHandler.GetExchange)

		rateHandler := admin.NewExchangeRatesHandler(exchangeRateService)
		r.Get("/exchange-rates", rateHandler.GetAllRates)
		r.Get("/exchange-rates/{id}", rateHandler.GetRate)
		r.Post("/exchange-rates", rateHandler.CreateRate)
		r.Put("/exchange-rates/{id}", rateHandler.UpdateRate)
		r.Delete("/exchange-rates/{id}", rateHandler.DeleteRate)
	})

	return r
}
