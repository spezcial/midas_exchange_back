package main

import (
	"net/http"

	"github.com/caspianex/exchange-backend/internal/api/admin"
	"github.com/caspianex/exchange-backend/internal/api/client"
	"github.com/caspianex/exchange-backend/internal/api/health"
	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/api/webhook"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/auth"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/config"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/caspianex/exchange-backend/pkg/worker"
	"github.com/go-chi/chi/v5"
)

func setupRouter(
	cfg *config.Config,
	log *logger.Logger,
	jwtManager *auth.JWTManager,
	tokenBlocklist cache.Store,
	wsService *WebSocketService,
	authService *service.AuthService,
	userService *service.UserService,
	walletService *service.WalletService,
	exchangeService *service.CurrencyExchangeService,
	exchangeRateService *service.ExchangeRatesService,
	oauthService *service.OAuthService,
	otcService *service.OTCService,
	feeService *service.PlatformFeeService,
	twofaService service.TwoFactorService,
	webAuthnService service.WebAuthnService,
	cgService *service.CryptoGateService,
	rateUpdater *worker.RateUpdater,
) http.Handler {
	r := chi.NewRouter()

	otcHub := NewOTCHub(jwtManager, log)

	r.Get("/ws", wsService.handler)
	r.Get("/ws/otc/{uid}", otcHub.ServeOTCChat)

	healthHandler := health.NewHealthHandler(rateUpdater)
	r.Get("/health", healthHandler.Health)
	r.Get("/health/detailed", healthHandler.HealthDetailed)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	if cgService != nil {
		cgHandler := webhook.NewCryptoGateHandler(cgService, cfg.CryptoGate.WebhookSecret, log)
		r.Post("/cg/deposit", cgHandler.HandleDeposit)
		r.Post("/cg/withdraw", cgHandler.HandleWithdraw)
	}

	r.Mount("/api/v1", apiV1(cfg, log, jwtManager, tokenBlocklist, authService, userService, walletService, exchangeService, exchangeRateService, oauthService, otcService, feeService, twofaService, webAuthnService, cgService, otcHub))

	return r
}

func apiV1(
	cfg *config.Config,
	log *logger.Logger,
	jwtManager *auth.JWTManager,
	tokenBlocklist cache.Store,
	authService *service.AuthService,
	userService *service.UserService,
	walletService *service.WalletService,
	exchangeService *service.CurrencyExchangeService,
	exchangeRateService *service.ExchangeRatesService,
	oauthService *service.OAuthService,
	otcService *service.OTCService,
	feeService *service.PlatformFeeService,
	twofaService service.TwoFactorService,
	webAuthnService service.WebAuthnService,
	cgService *service.CryptoGateService,
	otcHub *OTCHub,
) chi.Router {
	r := chi.NewRouter()

	authMiddleware := middleware.NewAuthMiddleware(jwtManager, tokenBlocklist)

	r.Use(middleware.Recovery)
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS(cfg.App.CORSAllowedOrigins))

	// ── Public auth ────────────────────────────────────────────────────────────
	authHandler := client.NewAuthHandler(authService, authMiddleware)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)
	r.Post("/auth/refresh", authHandler.RefreshToken)

	// 2FA login completion (public — user is not yet authenticated)
	twofaHandler := client.NewTwoFAHandler(authService, twofaService)
	r.Post("/auth/2fa/telegram/send", twofaHandler.SendLoginTelegram)
	r.Post("/auth/2fa/telegram/verify", twofaHandler.VerifyLoginTelegram)

	// Passkey login 2FA
	passkeyHandler := client.NewPasskeyHandler(webAuthnService, authService, twofaService)
	r.Post("/auth/2fa/passkey/begin", passkeyHandler.BeginLoginAuth)
	r.Post("/auth/2fa/passkey/finish", passkeyHandler.FinishLoginAuth)

	// Forgot-password (public — user is not logged in)
	r.Post("/auth/forgot-password/send", twofaHandler.ForgotPasswordSend)
	r.Post("/auth/forgot-password/verify", twofaHandler.ForgotPasswordVerify)
	r.Post("/auth/forgot-password/reset", twofaHandler.ForgotPasswordReset)

	// OAuth (kept for internal use; hidden from frontend)
	oauthHandler := client.NewOAuthHandler(oauthService)
	r.Get("/auth/google/url", oauthHandler.GetGoogleAuthURL)
	r.Post("/auth/google/callback", oauthHandler.GoogleCallback)

	// Public exchange rates
	exchangePairHandler := client.NewExchangePairHandler(exchangeRateService)
	r.Get("/exchange-rates", exchangePairHandler.GetActiveRates)

	// ── Protected client endpoints ─────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// Logout (authenticated so we can blocklist the access token)
		r.Get("/auth/me", authHandler.Me)
		r.Post("/auth/logout", authHandler.Logout)
		r.Put("/auth/password", authHandler.ChangePassword)

		// Profile
		profileHandler := client.NewProfileHandler(userService)
		r.Get("/profile", profileHandler.GetProfile)
		r.Put("/profile", profileHandler.UpdateProfile)

		// Phone 2FA management
		r.Post("/profile/phone/send-otp", twofaHandler.SendPhoneOTP)
		r.Post("/profile/phone/verify", twofaHandler.VerifyPhone)
		r.Delete("/profile/phone", twofaHandler.RemovePhone)

		// Passkey management
		r.Post("/profile/passkeys/register/begin", passkeyHandler.BeginRegistration)
		r.Post("/profile/passkeys/register/finish", passkeyHandler.FinishRegistration)
		r.Get("/profile/passkeys", passkeyHandler.ListPasskeys)
		r.Delete("/profile/passkeys/{id}", passkeyHandler.DeletePasskey)

		// Action challenges (pre-authorise sensitive actions)
		r.Post("/auth/action-challenge", twofaHandler.ActionChallengeTelegram)
		r.Post("/auth/action-verify/telegram", twofaHandler.ActionVerifyTelegram)
		r.Post("/auth/action-verify/passkey/begin", passkeyHandler.BeginActionAuth)
		r.Post("/auth/action-verify/passkey/finish", passkeyHandler.FinishActionAuth)

		// Wallets & transactions
		walletHandler := client.NewWalletHandler(walletService)
		r.Get("/wallet/currencies", walletHandler.GetAllCurrencies)
		r.Get("/wallets", walletHandler.GetWallets)
		r.Post("/wallets/withdraw", walletHandler.Withdraw)
		r.Get("/transactions", walletHandler.GetTransactions)

		if cgService != nil {
			depositAddrHandler := client.NewDepositAddressHandler(cgService, log)
			r.Get("/wallets/deposit-address", depositAddrHandler.GetDepositAddress)
		}

		// Exchanges
		exchangeHandler := client.NewExchangeHandler(exchangeService)
		r.Post("/exchanges", exchangeHandler.CreateExchange)
		r.Get("/exchanges", exchangeHandler.GetExchanges)
		r.Get("/exchanges/{id}", exchangeHandler.GetExchange)

		// OTC
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

	// ── Admin endpoints ────────────────────────────────────────────────────────
	r.Route("/admin", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireRole("admin", "super_admin"))

		userHandler := admin.NewUserHandler(userService)
		r.Get("/users", userHandler.ListUsers)
		r.Get("/users/{id}", userHandler.GetUser)
		r.Get("/users/{id}/profile", userHandler.GetProfile)
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

		feeHandler := admin.NewPlatformFeeHandler(feeService)
		r.Get("/platform-fees", feeHandler.ListFees)

		if cgService != nil {
			cryptoHandler := admin.NewCryptoHandler(cgService, log)
			r.With(authMiddleware.RequireRole("super_admin")).Get("/crypto/audit", cryptoHandler.AuditAddresses)
		}

		staffHandler := admin.NewStaffHandler(userService)
		r.Route("/super", func(r chi.Router) {
			r.Use(authMiddleware.RequireRole("super_admin"))
			r.Get("/staff", staffHandler.ListStaff)
			r.Post("/staff", staffHandler.CreateStaff)
			r.Put("/staff/{id}", staffHandler.UpdateStaff)
			r.Delete("/staff/{id}", staffHandler.DeactivateStaff)
		})

		otcAdminHandler := admin.NewOTCHandler(otcService, log, otcHub)
		r.Route("/otc", func(r chi.Router) {
			r.Use(authMiddleware.RequireRole("admin", "super_admin", "operator"))
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Get("/config", otcAdminHandler.GetConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Post("/config", otcAdminHandler.CreateConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Put("/config/{id}", otcAdminHandler.UpdateConfig)
			r.With(authMiddleware.RequireRole("admin", "super_admin")).Delete("/config/{id}", otcAdminHandler.DeleteConfig)
			r.Get("/analytics", otcAdminHandler.GetAnalytics)
			r.Get("/orders/export", otcAdminHandler.ExportOrders)
			r.Get("/orders", otcAdminHandler.ListOrders)
			r.Get("/orders/{uid}", otcAdminHandler.GetOrder)
			r.Get("/orders/{uid}/audit-log", otcAdminHandler.GetAuditLog)
			r.Put("/orders/{uid}/take", otcAdminHandler.TakeOrder)
			r.Put("/orders/{uid}/accept-proposed", otcAdminHandler.AcceptAsProposed)
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
