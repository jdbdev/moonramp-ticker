package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jdbdev/moonramp-ticker/config"
	"github.com/jdbdev/moonramp-ticker/db"
	"github.com/jdbdev/moonramp-ticker/internal/coins"
	"github.com/jdbdev/moonramp-ticker/internal/mapper"
	"github.com/jdbdev/moonramp-ticker/internal/ticker"
	"github.com/joho/godotenv"
)

// Collector service (moonramp-ticker)requires three services to run: internal/mapper, internal/coins and internal/ticker.
// Mapper service generates an ID map based on coin lookups (symbols) using Coinmarketcap API for ID mapping.
// ticker service fetches up to date data for each token/coin in the DB.
// Services run concurrently at set intervals found in config/config.go file. Adjust based on API credit expenditure.
// Services update the database with up to date data.
// All configuration settings are stored in .env and loaded by config/config.go file.

// Services holds the interfaces for the mapper, ticker and coins services.
type Services struct {
	Mapper mapper.IDMapInterface
	Ticker ticker.TickerInterface
	Coins  coins.CoinInterface
}

func main() {

	//==========================================================================
	// Configuration & Initialization/Setup
	//==========================================================================

	// Initialize Logger first (required for Init() and rest of app)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("CMC API application starting - Version 0.1")
	// Initialize applicaiton configuration
	app := InitConfig(logger)
	// Initialize http client
	client := &http.Client{}
	// Initialize services Mapper, Ticker and Coins. Inject dependencies required.
	services := InitServices(app, logger, client)

	//==========================================================================
	// Database Setup
	//==========================================================================

	// Create connection to database
	database, err := InitDatabase(app, logger)
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
	}
	if database != nil {
		defer database.Close()
		logger.Info("Database connection successful")
	}

	//==========================================================================
	// Service Calls
	//==========================================================================

	// mapperService calls with context timeout
	mapperCtx, mapperCancel := context.WithTimeout(context.Background(), app.CMC.RequestTimeout)
	defer mapperCancel()

	initialCoins, err := services.Mapper.GetCMCTopCoins(mapperCtx, 5)
	if err != nil {
		logger.Error("Failed getting topcoins", "error", err)
	} else {
		logger.Info("Initial top coins: ", "data", string(initialCoins)) // convert []byte to string for testing only
	}

	// tickerService calls with context timeout
	// coinService calls with context timeout

	//==========================================================================
	// Go Routines
	//==========================================================================
	tickerCtx, tickerCancel := context.WithCancel(context.Background())
	defer tickerCancel()
	go updateCoinQuotes(tickerCtx, app, logger, services)

	//==========================================================================
	// Application Shutdown (blocks main() thread until shutdown)
	//==========================================================================

	// Wait for interrupt signal to gracefully shut down
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down gracefully...")
	tickerCancel() // cancel ticker when app shuts down
}

// InitConfig initializes the application configuration and prints to stdout basic information
func InitConfig(logger *slog.Logger) *config.AppConfig {
	// Load .env file from root directory (monorepo structure)
	if err := godotenv.Load(".env"); err != nil {
		logger.Warn("Error loading .env file", "error", err)
	}
	app := config.NewAppConfig()
	PrintSettings(app)
	return app
}

// InitServices initializes the internal services Mapper, Ticker and Coins.
func InitServices(app *config.AppConfig, logger *slog.Logger, client *http.Client) *Services {
	mapperService := mapper.NewIDMapService(app, logger, client)
	coinService := coins.NewCoinService(logger)
	tickerService := ticker.NewTickerService(app, coinService, logger, client)

	return &Services{
		Mapper: mapperService,
		Ticker: tickerService,
		Coins:  coinService,
	}
}

// InitDatabase initializes the database instance if enabled in settings
func InitDatabase(app *config.AppConfig, logger *slog.Logger) (*db.Database, error) {
	if !app.AppCfg.UseDB {
		logger.Info("Database disabled in settings - not in use")
		return nil, nil
	}
	// Create new Database instance in db/postgres.go
	database, err := db.NewDatabase(app)
	if err != nil {
		log.Fatal(err)
	}
	db.SetDatabase(database)
	return database, nil
}

// updateCoinQuotes orchestrates calls to the API and DB updates with new data on set time interval.
// Uses two contexts:
// 1. ctx from main thread called by tickerCancel() when app shuts down.
// 2. reqCtx with timeout for each API call.
func updateCoinQuotes(ctx context.Context, app *config.AppConfig, logger *slog.Logger, services *Services) {
	timeInterval := app.Interval.TickerInterval
	ticker := time.NewTicker(timeInterval) // returns a *time.Ticker channel that reads from the channel C at set interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): // read from main thread context when tickerCancel() is called
			logger.Info("tickerContext cancelled from main thread, shutting down ticker service")
			return // graceful shutdown, function exits
		case <-ticker.C: // read from ticker.C channel at set interval
			// Create a new request context for each API call with timeout
			reqCtx, reqCancel := context.WithTimeout(ctx, app.CMC.RequestTimeout)
			cmcResponse, err := services.Ticker.FetchAndDecodeData(reqCtx)
			if err != nil {
				logger.Error("failed to fetch and decode data", "error", err)
				reqCancel() // release resources if API call fails
				continue
			}
			reqCancel() // release resources if API call succeeds

			// DEBUGGING ONLY REMOVE BEFORE PRODUCTION.
			logger.Info("Response body unmarshalled intoCMCResponse", "data", cmcResponse.Data)

		}

	}
}

// TEMP HELPERS ONLY. REMOVE BEFORE PRODUCTION.
func PrintSettings(app *config.AppConfig) {
	fmt.Printf("App in production: %v\n", app.AppCfg.InProduciton)
	fmt.Printf("Use DB: %v\n", app.AppCfg.UseDB)
	fmt.Printf("Base URL: %v\n", app.CMC.BaseURL)
	fmt.Printf("Request Timeout: %v\n", app.CMC.RequestTimeout)
}
