// Command itick-watch streams live ticks from itick.org via WebSocket.
//
// Usage:
//
//	itick-watch -asset crypto -symbol BTCUSDT -region BA
//	itick-watch -asset forex  -symbol GBPUSD  -region GB
//	itick-watch -asset stock  -symbol AAPL    -region US
//
// Token is read from ITICK_TOKEN. If the env var is unset, the program
// attempts to load it from a .env file in the current directory.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type assetSpec struct {
	wsPath  string // WS URL path
	restSeg string // REST URL segment (unused here, for reference)
}

var assets = map[string]assetSpec{
	"crypto": {wsPath: "/cws", restSeg: "crypto"},
	"forex":  {wsPath: "/fws", restSeg: "forex"},
	"stock":  {wsPath: "/sws", restSeg: "stock"},
}

type frame struct {
	AC     string          `json:"ac"`
	Params string          `json:"params,omitempty"`
	Types  string          `json:"types,omitempty"`
	Code   int             `json:"code,omitempty"`
	Msg    string          `json:"msg,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
	Type   string          `json:"type,omitempty"`
}

func main() {
	asset := flag.String("asset", "crypto", "asset class: crypto | forex | stock")
	symbol := flag.String("symbol", "BTCUSDT", "symbol code, e.g. BTCUSDT, GBPUSD, AAPL")
	region := flag.String("region", "BA", "region/exchange code, e.g. BA (Binance), GB, US")
	types := flag.String("types", "quote", "comma-separated subscription types: quote,depth,trade,kline")
	host := flag.String("host", envOrDefault("ITICK_WS_HOST", "api-free.itick.org"), "iTick WS host (free=api-free.itick.org, paid=api0.itick.org)")
	flag.Parse()

	spec, ok := assets[strings.ToLower(*asset)]
	if !ok {
		log.Fatalf("unknown -asset %q (want crypto|forex|stock)", *asset)
	}

	token := loadToken()
	if token == "" {
		log.Fatal("ITICK_TOKEN is not set (env var or .env file)")
	}

	url := "wss://" + *host + spec.wsPath
	log.Printf("connecting %s", url)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(frame{AC: "auth", Params: token}); err != nil {
		log.Fatalf("auth send: %v", err)
	}
	sub := fmt.Sprintf("%s$%s", strings.ToUpper(*symbol), strings.ToUpper(*region))
	if err := conn.WriteJSON(frame{AC: "subscribe", Params: sub, Types: *types}); err != nil {
		log.Fatalf("subscribe send: %v", err)
	}
	log.Printf("subscribed %s types=%s", sub, *types)

	// Background ping every 15s — itick closes idle sockets ~30s.
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := conn.WriteJSON(frame{AC: "ping"}); err != nil {
					log.Printf("ping: %v", err)
					return
				}
			}
		}
	}()

	// Close gracefully on Ctrl+C.
	go func() {
		<-ctx.Done()
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"),
			time.Now().Add(2*time.Second),
		)
		_ = conn.Close()
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("shutdown")
				return
			}
			log.Fatalf("read: %v", err)
		}
		printTick(raw)
	}
}

func printTick(raw []byte) {
	var f frame
	if err := json.Unmarshal(raw, &f); err != nil {
		fmt.Println(string(raw))
		return
	}
	if f.Code != 0 && f.Msg != "" {
		log.Printf("server: code=%d msg=%s", f.Code, f.Msg)
		return
	}
	if len(f.Data) == 0 {
		log.Printf("ctrl: %s", string(raw))
		return
	}
	fmt.Printf("%s %s\n", time.Now().Format("15:04:05.000"), string(f.Data))
}

func loadToken() string {
	if v := os.Getenv("ITICK_TOKEN"); v != "" {
		return v
	}
	// Fallback: read .env in CWD. Minimal parser — KEY=VALUE, ignores #-comments.
	f, err := os.Open(".env")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "ITICK_TOKEN" {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
