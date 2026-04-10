// backfill_deposit_addresses creates blockchain deposit addresses via crypto-gate
// for all existing client users who don't yet have one for each supported currency.
//
// Usage:
//
//	go run ./cmd/backfill_deposit_addresses
//
// Required env vars: CRYPTO_GATE_URL, CRYPTO_GATE_TOKEN, DB_PASSWORD, JWT_SECRET
// (same .env used by the main server)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/config"
	"github.com/caspianex/exchange-backend/pkg/cryptogate"
	"github.com/caspianex/exchange-backend/pkg/database"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("config error: %v\n", err)
		os.Exit(1)
	}

	if cfg.CryptoGate.BaseURL == "" || cfg.CryptoGate.Token == "" {
		fmt.Println("CRYPTO_GATE_URL and CRYPTO_GATE_TOKEN must be set")
		os.Exit(1)
	}

	log := logger.New(cfg.Server.Env)

	db, err := database.NewPostgres(cfg.GetDSN())
	if err != nil {
		log.Error("db connect", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	memCache := cache.NewMemoryCache(cache.NoExpiration, cache.NoExpiration)
	cacheService := cache.NewCacheService(memCache, log)

	userRepo := repository.NewUserRepository(db, cacheService)
	walletRepo := repository.NewWalletRepository(db, cacheService)
	txRepo := repository.NewTransactionRepository(db)
	addrRepo := repository.NewDepositAddressRepository(db)

	cgClient := cryptogate.NewClient(cfg.CryptoGate.BaseURL, cfg.CryptoGate.Token, cfg.CryptoGate.Platform)
	cgService := service.NewCryptoGateService(cgClient, cfg.CryptoGate.Platform, addrRepo, walletRepo, txRepo, log)

	ctx := context.Background()

	// Collect all supported crypto currencies that exist in the DB
	type currencyEntry struct {
		code string
		id   int64
	}
	var cryptoCurrencies []currencyEntry
	for code := range domain.CryptoGateCurrencies {
		currency, err := walletRepo.GetCurrencyByCode(ctx, code)
		if err != nil {
			log.Warn("currency not in DB, skipping", "code", code)
			continue
		}
		cryptoCurrencies = append(cryptoCurrencies, currencyEntry{code: code, id: int64(currency.ID)})
	}

	if len(cryptoCurrencies) == 0 {
		fmt.Println("No supported crypto currencies found in database")
		os.Exit(0)
	}

	log.Info("Starting deposit address backfill",
		"currencies", len(cryptoCurrencies))

	const (
		pageSize        = 100
		batchPause      = 200 * time.Millisecond // brief pause between user batches
		perRequestPause = 50 * time.Millisecond  // pause between individual crypto-gate calls
	)

	offset := 0
	totalUsers := 0
	totalOK := 0
	totalFailed := 0

	for {
		users, err := userRepo.List(ctx, pageSize, offset, "")
		if err != nil {
			log.Error("list users", "error", err)
			os.Exit(1)
		}
		if len(users) == 0 {
			break
		}

		for _, user := range users {
			totalUsers++
			for _, c := range cryptoCurrencies {
				_, err := cgService.GetOrCreateDepositAddress(ctx, user.ID, c.code, c.id)
				if err != nil {
					log.Warn("failed to create deposit address",
						"user_id", user.ID, "currency", c.code, "error", err)
					totalFailed++
				} else {
					totalOK++
				}
				time.Sleep(perRequestPause)
			}
		}

		offset += pageSize
		log.Info("progress", "users_processed", totalUsers)
		time.Sleep(batchPause)
	}

	fmt.Printf("\nBackfill complete:\n  users processed : %d\n  addresses ok    : %d\n  errors          : %d\n",
		totalUsers, totalOK, totalFailed)
}
