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
	oauthService *service.OAuthService,
	otcService *service.OTCService,
	rateUpdater *worker.RateUpdater,
) http.Handler {
	r := chi.NewRouter()

	otcHub := NewOTCHub(jwtManager, log)

	r.Get("/ws", wsService.handler)
	r.Get("/ws/otc/{uid}", otcHub.ServeOTCChat)

	// Health check endpoints
	healthHandler := health.NewHealthHandler(rateUpdater)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/detailed", healthHandler.HealthDetailed)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	r.Mount("/api/v1", apiV1(cfg, log, jwtManager, authService, userService, walletService, exchangeService, exchangeRateService, oauthService, otcService, otcHub))

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
	oauthService *service.OAuthService,
	otcService *service.OTCService,
	otcHub *OTCHub,
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

	// Public OAuth endpoints
	oauthHandler := client.NewOAuthHandler(oauthService)
	r.Get("/auth/google/url", oauthHandler.GetGoogleAuthURL)
	r.Post("/auth/google/callback", oauthHandler.GoogleCallback)

	// Public exchange rates endpoint
	exchangePairHandler := client.NewExchangePairHandler(exchangeRateService)
	r.Get("/exchange-rates", exchangePairHandler.GetActiveRates)

	// Protected client endpoints
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		r.Put("/auth/password", authHandler.ChangePassword)

		profileHandler := client.NewProfileHandler(userService)
		r.Get("/profile", profileHandler.GetProfile)
		r.Put("/profile", profileHandler.UpdateProfile)

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

		otcHandler := client.NewOTCHandler(otcService, otcHub)
		r.Route("/otc", func(r chi.Router) {
			r.Get("/config", otcHandler.GetActiveConfigs)
			r.Post("/orders", otcHandler.CreateOrder)
			r.Get("/orders", otcHandler.ListOrders)
			r.Get("/orders/{uid}", otcHandler.GetOrder)
			r.Post("/orders/{uid}/messages", otcHandler.SendMessage)
			r.Post("/orders/{uid}/offers", otcHandler.SendOffer)
			r.Put("/orders/{uid}/offers/{messageID}/accept", otcHandler.AcceptOffer)
			r.Put("/orders/{uid}/offers/{messageID}/reject", otcHandler.RejectOffer)
			r.Delete("/orders/{uid}", otcHandler.CancelOrder)
		})
	})

	// Admin endpoints
	r.Route("/admin", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireRole("admin", "super_admin"))

		userHandler := admin.NewUserHandler(userService)
		r.Get("/users", userHandler.ListUsers)
		r.Get("/users/{id}", userHandler.GetUser)
		r.Get("/users/{id}/profile", userHandler.GetProfile)
		r.Post("/users/{id}/profile", userHandler.SetProfile)
		r.Put("/users/{id}/profile", userHandler.UpdateProfile)

		exchangeHandler := admin.NewExchangeHandler(exchangeService)
		r.Get("/exchanges", exchangeHandler.ListExchanges)
		r.Get("/exchanges/{id}", exchangeHandler.GetExchange)

		rateHandler := admin.NewExchangeRatesHandler(exchangeRateService)
		r.Get("/exchange-rates", rateHandler.GetAllRates)
		r.Get("/exchange-rates/{id}", rateHandler.GetRate)
		r.Post("/exchange-rates", rateHandler.CreateRate)
		r.Put("/exchange-rates/{id}", rateHandler.UpdateRate)
		r.Delete("/exchange-rates/{id}", rateHandler.DeleteRate)

		walletHandler := admin.NewWalletHandler(walletService)
		r.Post("/wallets/deposit", walletHandler.ManualDeposit)

		// Super-admin only: staff management
		staffHandler := admin.NewStaffHandler(userService)
		r.Route("/super", func(r chi.Router) {
			r.Use(authMiddleware.RequireRole("super_admin"))
			r.Get("/staff", staffHandler.ListStaff)
			r.Post("/staff", staffHandler.CreateStaff)
			r.Put("/staff/{id}", staffHandler.UpdateStaff)
			r.Delete("/staff/{id}", staffHandler.DeactivateStaff)
		})

		// OTC orders (admin + operator access)
		otcAdminHandler := admin.NewOTCHandler(otcService, log, otcHub)
		r.Route("/otc", func(r chi.Router) {
			r.Use(authMiddleware.RequireRole("admin", "super_admin", "operator"))
			// Config (pair settings) — admin/super_admin only
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Get("/config", otcAdminHandler.GetConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Post("/config", otcAdminHandler.CreateConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Put("/config/{id}", otcAdminHandler.UpdateConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Delete("/config/{id}", otcAdminHandler.DeleteConfig)
			// Analytics & reporting
			r.Get("/analytics", otcAdminHandler.GetAnalytics)
			r.Get("/orders/export", otcAdminHandler.ExportOrders)
			// Orders
			r.Get("/orders", otcAdminHandler.ListOrders)
			r.Get("/orders/{uid}", otcAdminHandler.GetOrder)
			r.Get("/orders/{uid}/audit-log", otcAdminHandler.GetAuditLog)
			r.Put("/orders/{uid}/take", otcAdminHandler.TakeOrder)
			r.Post("/orders/{uid}/messages", otcAdminHandler.SendMessage)
			r.Post("/orders/{uid}/offers", otcAdminHandler.SendOffer)
			r.Put("/orders/{uid}/offers/{id}/accept", otcAdminHandler.AcceptOffer)
			r.Put("/orders/{uid}/offers/{id}/reject", otcAdminHandler.RejectOffer)
			r.Put("/orders/{uid}/payment-received", otcAdminHandler.ConfirmPaymentReceived)
			r.Put("/orders/{uid}/complete", otcAdminHandler.CompleteOrder)
			r.Delete("/orders/{uid}", otcAdminHandler.CancelOrder)
		})
	})

	return r
}
