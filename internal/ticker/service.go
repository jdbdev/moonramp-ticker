package ticker

// Coinmarketcap (CMC) API Documentation: https://coinmarketcap.com/api/documentation/v1/
// CMC recommends using CoinMarketCap ID's instead of ID or other identifiers
// Common endpoints:
// https://pro-api.coinmarketcap.com/v1/cryptocurrency/listings/latest
// https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest

// Sample CMD ID's:
// Bitcoin CMC ID: 1
// Ethereum CMC ID: 1027
// Solana CMC ID: 5994
// Sui CMC ID: 20947
// Cardano CMC ID: 2010
// ICP: 8916

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/jdbdev/moonramp-ticker/config"
	"github.com/jdbdev/moonramp-ticker/internal/coins"
)

// TEMP SLICE ONLY. USE DB TABLE FOR ID MAP.
var coinIDMap []string = []string{"1", "1027", "5994", "20947", "2010", "8916"}

// TickerInterface has a singular method for TickerService to orchestrate the sync process from API to DB.
type TickerInterface interface {
	Sync(ctx context.Context) error
}

// TickerService implements the TickerInterface that can sync data from API to DB.
type TickerService struct {
	apiKey    string
	baseURL   string
	quotesURL string
	client    *http.Client
	logger    *slog.Logger
	coins     coins.CoinInterface
	// data    []TickerData // Add a field to store the decoded data
}

// NewTickerService creates a new instance of the TickerService struct
func NewTickerService(app *config.AppConfig, coinService coins.CoinInterface, logger *slog.Logger, client *http.Client) *TickerService {
	// Validate required dependencies (panic if missing)
	if app == nil {
		panic("App configuration required to create TickerService")
	}
	// Validate required dependencies (Warn if missing)
	if logger == nil {
		logger = slog.Default()
	}
	if app.CMC.APIKey == "" {
		logger.Warn("No API key provided - requires API key")
	}
	if app.CMC.QuotesURL == "" {
		logger.Warn("No quotes URL provided - requires quotes URL")
	}
	if client == nil {
		logger.Warn("No HTTP client provided - requires HTTP client")
	}
	logger.Info("TickerService initialized successfully")

	// Return struct with values
	return &TickerService{
		apiKey:    app.CMC.APIKey,
		baseURL:   app.CMC.BaseURL,
		quotesURL: app.CMC.QuotesURL,
		client:    client,
		logger:    logger,
		coins:     coinService,
	}
}

// Sync fetches data from CMC API, decodes to JSON and updates the database
func (t *TickerService) Sync(ctx context.Context) error {
	// Call and return data from CMC API as []byte
	data, err := t.CallAPI(ctx)
	if err != nil {
		t.logger.Error("failed to fetch and decode data", "error", err)
		return err
	}
	// Decode []byte data into CMCResponse struct
	myStruct, err := t.DecodeData(data)
	// Update the database with the new data from CMCResponse struct
	t.UpdateDB(myStruct)
	return nil
}

// CallAPI gets and decodes data from CMC and returns a []byte of the JSON response
func (t *TickerService) CallAPI(ctx context.Context) ([]byte, error) {

	// Create new request with context
	req, err := http.NewRequestWithContext(ctx, "GET", t.quotesURL, nil)
	if err != nil {
		t.logger.Error("failed to create request", "error", err)
		return nil, err
	}

	// Build query parameters
	q := url.Values{}

	// Collect all IDs from the map
	q.Add("id", strings.Join(coinIDMap, ",")) // Join IDs with commas and add to query
	q.Add("convert", "USD")

	// Only get requested fields (automatically get price, market_cap, volume_24h, etc. in "quotes"):
	// Available aux fields: num_market_pairs, cmc_rank, date_added, tags, platform, max_supply,
	// circulating_supply, total_supply, market_cap_by_total_supply, volume_24h_reported,
	// volume_7d, volume_7d_reported, volume_30d, volume_30d_reported, is_active, is_fiat
	q.Add("aux", "circulating_supply,total_supply,volume_24h_reported")

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CMC_PRO_API_KEY", t.apiKey)

	// Add query parameters to URL
	req.URL.RawQuery = q.Encode()

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		t.logger.Error("HTTP request failed", "error", err, "url", req.URL.String())
		return nil, err
	} else {
		t.logger.Info("HTTP request successful", "status", resp.Status, "url", req.URL.String())
	}
	defer resp.Body.Close()

	// Read and debug response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.logger.Error("failed to read response body", "error", err)
		return nil, err
	}

	return respBody, nil

	// // Unmarshal JSON response into CMCResponse struct
	// var cmcResponse CMCResponse
	// if err := json.Unmarshal(respBody, &cmcResponse); err != nil {
	// 	t.logger.Error("failed to unmarshal response", "error", err)
	// 	return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	// }

	// // Check for API errors
	// if cmcResponse.Status.ErrorCode != 0 {
	// 	errorMsg := "API error"
	// 	if cmcResponse.Status.ErrorMessage != nil {
	// 		errorMsg = *cmcResponse.Status.ErrorMessage
	// 	}
	// 	t.logger.Error("Coinmarketcap API returned error",
	// 		"error_code", cmcResponse.Status.ErrorCode,
	// 		"error_message", errorMsg,
	// 		"credit_count", cmcResponse.Status.CreditCount)
	// 	return nil, fmt.Errorf("API error (code %d): %s", cmcResponse.Status.ErrorCode, errorMsg)
	// }

	// t.logger.Info("Successfully fetched and decoded CMC data",
	// 	"coins_count", len(cmcResponse.Data),
	// 	"credit_count", cmcResponse.Status.CreditCount)
	// return &cmcResponse, nil
}

// DecodeData decodes a JSON[]byte into a CMCResponse struct
func (t *TickerService) DecodeData(data []byte) (*CMCResponse, error) {
	return nil, nil
}

// UpdateDB updates the database with data from CMCResponse struct
func (t *TickerService) UpdateDB(myStruct *CMCResponse) error {
	return nil
}
