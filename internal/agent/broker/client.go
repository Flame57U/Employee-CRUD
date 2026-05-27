// Package broker wraps a broker's REST API for order placement and balance queries.
// The implementation below targets the Huatai (华泰) OpenAPI as a concrete example.
// To integrate a different broker, replace signedPost/signedGet and the JSON
// request/response structs while keeping the PlaceOrder / GetBalances signatures.
package broker

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/quantsaas/platform/internal/agent/types"
)

const (
	htscProdBase = "https://open.htsc.com.cn/api/v1"
	htscSimBase  = "https://open.htsc.com.cn/api/v1/simulated"

	httpTimeout = 10 * time.Second

	// Huatai trade-type codes
	tradeTypeBuy  = 0
	tradeTypeSell = 1

	// Huatai price-type code for market orders
	priceTypeMarket = 0
)

// Client wraps the broker REST API. All credential fields are sourced from the
// local config file and are never forwarded to SaaS.
type Client struct {
	apiKey    string
	secretKey string
	tradePass string
	baseURL   string
	http      *http.Client
}

// New constructs a broker Client.
func New(apiKey, secretKey, tradePass string, simulated bool) *Client {
	base := htscProdBase
	if simulated {
		base = htscSimBase
	}
	return &Client{
		apiKey:    apiKey,
		secretKey: secretKey,
		tradePass: tradePass,
		baseURL:   base,
		http:      &http.Client{Timeout: httpTimeout},
	}
}

// PlaceOrder executes a market order.
//
//   - BUY:  issues a buy-by-value market order spending AmountCNY yuan.
//   - SELL: issues a sell-by-quantity market order for QtyAsset shares.
func (c *Client) PlaceOrder(cmd types.TradeCommand) (types.Execution, error) {
	type orderReq struct {
		StockCode   string `json:"stockCode"`
		TradeType   int    `json:"tradeType"`             // 0=buy, 1=sell
		PriceType   int    `json:"priceType"`             // 0=market
		OrderAmount string `json:"orderAmount,omitempty"` // CNY, for buy-by-value
		OrderVolume string `json:"orderVolume,omitempty"` // shares, for sell
		TradePass   string `json:"tradePassword,omitempty"`
	}

	req := orderReq{
		StockCode: cmd.Symbol,
		PriceType: priceTypeMarket,
		TradePass: c.tradePass,
	}
	switch cmd.Action {
	case "BUY":
		req.TradeType = tradeTypeBuy
		req.OrderAmount = cmd.AmountCNY
	case "SELL":
		req.TradeType = tradeTypeSell
		req.OrderVolume = cmd.QtyAsset
	default:
		return types.Execution{}, fmt.Errorf("unknown action %q", cmd.Action)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return types.Execution{}, fmt.Errorf("marshal order request: %w", err)
	}

	resp, err := c.signedPost("/trade/order", body)
	if err != nil {
		return types.Execution{}, fmt.Errorf("broker POST /trade/order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.Execution{Status: "failed"},
			fmt.Errorf("broker returned HTTP %d", resp.StatusCode)
	}

	var brokerResp struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		FilledQty string `json:"filledQty"`
		FilledAmt string `json:"filledAmt"` // total CNY value filled
		Fee       string `json:"fee"`
		Status    string `json:"status"` // "filled" / "partial" / "failed"
	}
	if err := json.NewDecoder(resp.Body).Decode(&brokerResp); err != nil {
		return types.Execution{}, fmt.Errorf("decode broker response: %w", err)
	}
	if brokerResp.Code != 0 {
		return types.Execution{
			ClientOrderID: cmd.ClientOrderID,
			Symbol:        cmd.Symbol,
			Action:        cmd.Action,
			Status:        "failed",
		}, fmt.Errorf("broker error %d: %s", brokerResp.Code, brokerResp.Message)
	}

	filledQty, _ := strconv.ParseFloat(brokerResp.FilledQty, 64)
	filledAmt, _ := strconv.ParseFloat(brokerResp.FilledAmt, 64)
	fee, _ := strconv.ParseFloat(brokerResp.Fee, 64)

	var filledPrice float64
	if filledQty > 0 {
		filledPrice = filledAmt / filledQty
	}

	return types.Execution{
		ClientOrderID: cmd.ClientOrderID,
		Symbol:        cmd.Symbol,
		Action:        cmd.Action,
		FilledQty:     filledQty,
		FilledPrice:   filledPrice,
		Fee:           fee,
		Status:        "filled",
	}, nil
}

// GetBalances returns all asset balances (shares and CNY cash) from the broker.
func (c *Client) GetBalances() ([]types.Balance, error) {
	resp, err := c.signedGet("/account/balance")
	if err != nil {
		return nil, fmt.Errorf("broker GET /account/balance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("broker returned HTTP %d", resp.StatusCode)
	}

	var brokerResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Assets  []struct {
			Asset     string `json:"asset"`
			Available string `json:"available"`
			Frozen    string `json:"frozen"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&brokerResp); err != nil {
		return nil, fmt.Errorf("decode balance response: %w", err)
	}
	if brokerResp.Code != 0 {
		return nil, fmt.Errorf("broker error %d: %s", brokerResp.Code, brokerResp.Message)
	}

	balances := make([]types.Balance, 0, len(brokerResp.Assets))
	for _, a := range brokerResp.Assets {
		avail, _ := strconv.ParseFloat(a.Available, 64)
		frozen, _ := strconv.ParseFloat(a.Frozen, 64)
		balances = append(balances, types.Balance{
			Asset:     a.Asset,
			Available: avail,
			Frozen:    frozen,
		})
	}
	return balances, nil
}

// -- HTTP helpers --

// signedPost issues an HMAC-SHA256-signed POST to the broker API.
func (c *Client) signedPost(path string, body []byte) (*http.Response, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := c.sign("POST", path, ts)

	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req, ts, sig)
	return c.http.Do(req)
}

// signedGet issues an HMAC-SHA256-signed GET to the broker API.
func (c *Client) signedGet(path string) (*http.Response, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := c.sign("GET", path, ts)

	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req, ts, sig)
	return c.http.Do(req)
}

func (c *Client) setAuthHeaders(req *http.Request, timestamp, sig string) {
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("X-TIMESTAMP", timestamp)
	req.Header.Set("X-SIGNATURE", sig)
}

// sign computes HMAC-SHA256(method + path + timestamp, secretKey).
func (c *Client) sign(method, path, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(method + path + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}
