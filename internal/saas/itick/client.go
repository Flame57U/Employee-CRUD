// Package itick is a thin server-side client for the itick.org market-data REST
// API. It exists so the ITICK_TOKEN secret stays on the SaaS server (Iron Rule
// #5) and never reaches the browser — the frontend calls /api/v1/market/* and
// this client attaches the token out of band.
package itick

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Quote mirrors the itick "quote" payload. Field tags map the terse upstream
// keys to readable names.
type Quote struct {
	Symbol    string  `json:"s"`
	Last      float64 `json:"ld"`
	Open      float64 `json:"o"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Time      int64   `json:"t"` // epoch milliseconds
	Volume    float64 `json:"v"`
	Turnover  float64 `json:"tu"`
	Type      string  `json:"type"`
	Region    string  `json:"r"`
	Change    float64 `json:"ch"`
	ChangePct float64 `json:"chp"`
}

type apiResponse struct {
	Code int             `json:"code"`
	Msg  *string         `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// Client talks to a single itick REST host with a fixed token.
type Client struct {
	token string
	host  string
	hc    *http.Client
}

// NewClient builds a Client. An empty host defaults to the paid endpoint.
func NewClient(token, host string) *Client {
	if host == "" {
		host = "api0.itick.org"
	}
	return &Client{token: token, host: host, hc: &http.Client{Timeout: 10 * time.Second}}
}

// Enabled reports whether a token is configured.
func (c *Client) Enabled() bool { return c != nil && c.token != "" }

// allowedAssets gates the path segment so a caller can't inject arbitrary URLs.
var allowedAssets = map[string]bool{"forex": true, "crypto": true, "stock": true}

// Quote fetches a single quote for code in region under the given asset class.
func (c *Client) Quote(ctx context.Context, asset, region, code string) (*Quote, error) {
	if !allowedAssets[asset] {
		return nil, fmt.Errorf("invalid asset %q (want forex|crypto|stock)", asset)
	}
	u := fmt.Sprintf("https://%s/%s/quote?region=%s&code=%s",
		c.host, asset, url.QueryEscape(region), url.QueryEscape(code))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("token", c.token)
	req.Header.Set("accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("itick http %d", resp.StatusCode)
	}

	var ar apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}
	if ar.Code != 0 {
		msg := "upstream error"
		if ar.Msg != nil && *ar.Msg != "" {
			msg = *ar.Msg
		}
		return nil, fmt.Errorf("itick code %d: %s", ar.Code, msg)
	}
	var q Quote
	if err := json.Unmarshal(ar.Data, &q); err != nil {
		return nil, err
	}
	return &q, nil
}
