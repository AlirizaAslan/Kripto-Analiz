package marketdata

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"pulsealpha/api/internal/domain"
)

type Instrument struct {
	Symbol         string
	RawSymbol      string
	Name           string
	InstrumentType string
	Venue          string
	ExchangeCode   string
	SessionLabel   string
	RestBaseURL    string
	WSBaseURL      string
}

type Snapshot struct {
	Instrument      Instrument
	LastPrice       float64
	ChangePercent24 float64
	Volume          float64
	High24          float64
	Low24           float64
	Depth           domain.DepthSnapshot
	Candles         []domain.Candle
	ChartCandles    map[string][]domain.Candle
	ObservedAt      time.Time
}

type BinanceProvider struct {
	client      *http.Client
	instruments map[string]Instrument
	states      map[string]*symbolState
	startOnce   sync.Once
}

type chartIntervalRequest struct {
	interval string
	limit    int
}

var extendedChartIntervals = []chartIntervalRequest{
	{interval: "5m", limit: 720},
	{interval: "15m", limit: 720},
	{interval: "1h", limit: 720},
	{interval: "4h", limit: 720},
	{interval: "1d", limit: 365},
	{interval: "1w", limit: 260},
}

type symbolState struct {
	instrument Instrument
	mu         sync.RWMutex
	lastPrice  float64
	depth      domain.DepthSnapshot
	closed     []domain.Candle
	candles    []domain.Candle
	klineCache map[string]cachedKlines
	bucket     *candleBucket
	lastUpdate time.Time
}

type cachedKlines struct {
	candles   []domain.Candle
	fetchedAt time.Time
}

type candleBucket struct {
	second int64
	open   float64
	high   float64
	low    float64
	close  float64
	volume float64
}

func NewBinanceProvider(spotBaseURL, futuresBaseURL string, spotSymbols, futuresSymbols []string) *BinanceProvider {
	spotWSURL := "wss://stream.binance.com:9443/ws"
	futuresWSURL := "wss://fstream.binance.com/ws"
	instruments := make(map[string]Instrument)
	states := make(map[string]*symbolState)

	for _, symbol := range spotSymbols {
		raw := strings.ToUpper(strings.TrimSpace(symbol))
		if raw == "" {
			continue
		}
		key := raw + "-SPOT"
		instrument := Instrument{
			Symbol:         key,
			RawSymbol:      raw,
			Name:           baseAssetName(raw) + " / Binance Spot",
			InstrumentType: "crypto_spot",
			Venue:          "binance",
			ExchangeCode:   "BINANCE",
			SessionLabel:   "24/7 Spot",
			RestBaseURL:    strings.TrimRight(spotBaseURL, "/"),
			WSBaseURL:      spotWSURL,
		}
		instruments[key] = instrument
		states[key] = &symbolState{instrument: instrument, klineCache: make(map[string]cachedKlines)}
	}

	for _, symbol := range futuresSymbols {
		raw := strings.ToUpper(strings.TrimSpace(symbol))
		if raw == "" {
			continue
		}
		key := raw + "-PERP"
		instrument := Instrument{
			Symbol:         key,
			RawSymbol:      raw,
			Name:           baseAssetName(raw) + " / Binance Perpetual",
			InstrumentType: "crypto_perpetual",
			Venue:          "binance_futures",
			ExchangeCode:   "BINANCE",
			SessionLabel:   "24/7 Perpetual",
			RestBaseURL:    strings.TrimRight(futuresBaseURL, "/"),
			WSBaseURL:      futuresWSURL,
		}
		instruments[key] = instrument
		states[key] = &symbolState{instrument: instrument, klineCache: make(map[string]cachedKlines)}
	}

	provider := &BinanceProvider{
		client:      &http.Client{Timeout: 4 * time.Second},
		instruments: instruments,
		states:      states,
	}
	provider.Start()
	return provider
}

func (p *BinanceProvider) Enabled() bool {
	return p != nil && len(p.instruments) > 0
}

func (p *BinanceProvider) Instruments() []Instrument {
	items := make([]Instrument, 0, len(p.instruments))
	for _, instrument := range p.instruments {
		items = append(items, instrument)
	}
	sortInstruments(items)
	return items
}

func (p *BinanceProvider) Start() {
	p.startOnce.Do(func() {
		for _, instrument := range p.instruments {
			go p.runTradeStream(instrument)
			go p.runDepthStream(instrument)
		}
	})
}

func (p *BinanceProvider) Snapshot(symbol string) (Snapshot, error) {
	return p.snapshotWithOptions(symbol, true)
}

func (p *BinanceProvider) LiteSnapshot(symbol string) (Snapshot, error) {
	return p.snapshotWithOptions(symbol, false)
}

func (p *BinanceProvider) snapshotWithOptions(symbol string, includeCharts bool) (Snapshot, error) {
	p.Start()
	state, ok := p.states[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok {
		return Snapshot{}, fmt.Errorf("symbol not configured")
	}

	var (
		price      float64
		ticker     tickerSnapshot
		bookTicker bookTickerSnapshot
		restDepth  domain.DepthSnapshot
	)

	var fetchWG sync.WaitGroup
	fetchWG.Add(4)
	go func() {
		defer fetchWG.Done()
		price, _ = p.fetchPrice(state.instrument)
	}()
	go func() {
		defer fetchWG.Done()
		ticker, _ = p.fetchTicker(state.instrument)
	}()
	go func() {
		defer fetchWG.Done()
		bookTicker, _ = p.fetchBookTicker(state.instrument)
	}()
	go func() {
		defer fetchWG.Done()
		restDepth, _ = p.fetchDepth(state.instrument, 20)
	}()
	fetchWG.Wait()

	state.mu.RLock()
	lastPrice := price
	if lastPrice == 0 {
		lastPrice = ticker.LastPrice
	}
	if lastPrice == 0 {
		lastPrice = state.lastPrice
	}
	depth := state.depth
	lastUpdate := state.lastUpdate
	state.mu.RUnlock()

	if lastPrice == 0 {
		return Snapshot{}, fmt.Errorf("live state not ready")
	}

	if len(restDepth.Bids) > 0 && len(restDepth.Asks) > 0 {
		depth = restDepth
	} else if bookTicker.BestBid > 0 && bookTicker.BestAsk > 0 {
		depth = buildDepthFromBookTicker(bookTicker, nonZeroTime(lastUpdate, time.Now().UTC()))
	} else if depth.BestBid == 0 || depth.BestAsk == 0 {
		depth = buildDepthFromPrice(lastPrice, lastUpdate)
	}

	oneMinute, err := p.fetchCachedKlines(state, "1m", 1440, lastPrice)
	if err != nil || len(oneMinute) == 0 {
		oneMinute = buildFlatCandles(lastPrice, 120)
	}
	chartCandles := map[string][]domain.Candle{
		"1m_recent": tailCandles(oneMinute, 60),
		"1m":        append([]domain.Candle(nil), oneMinute...),
	}

	if includeCharts {
		var chartWG sync.WaitGroup
		var chartMu sync.Mutex
		for _, request := range extendedChartIntervals {
			chartWG.Add(1)
			go func(req chartIntervalRequest) {
				defer chartWG.Done()
				if candles, err := p.fetchCachedKlines(state, req.interval, req.limit, lastPrice); err == nil && len(candles) > 0 {
					chartMu.Lock()
					chartCandles[req.interval] = candles
					chartMu.Unlock()
				}
			}(request)
		}
		chartWG.Wait()
	}

	return Snapshot{
		Instrument:      state.instrument,
		LastPrice:       lastPrice,
		ChangePercent24: ticker.PriceChangePercent,
		Volume:          ticker.Volume,
		High24:          nonZero(ticker.HighPrice, lastPrice),
		Low24:           nonZero(ticker.LowPrice, lastPrice),
		Depth:           depth,
		Candles:         tailCandles(oneMinute, 120),
		ChartCandles:    chartCandles,
		ObservedAt:      nonZeroTime(lastUpdate, time.Now().UTC()),
	}, nil
}

func (p *BinanceProvider) runTradeStream(instrument Instrument) {
	streamURL := instrument.WSBaseURL + "/" + strings.ToLower(instrument.RawSymbol) + "@trade"
	for {
		if err := p.consumeStream(streamURL, func(payload []byte) {
			var message tradeMessage
			if json.Unmarshal(payload, &message) != nil {
				return
			}
			price := parseFloat(message.Price)
			if price <= 0 {
				return
			}
			state := p.states[instrument.Symbol]
			state.applyTrade(price, parseFloat(message.Quantity), time.UnixMilli(message.TradeTime).UTC())
		}); err != nil {
			time.Sleep(2 * time.Second)
		}
	}
}

func (p *BinanceProvider) runDepthStream(instrument Instrument) {
	streamURL := instrument.WSBaseURL + "/" + strings.ToLower(instrument.RawSymbol) + "@depth20@100ms"
	for {
		if err := p.consumeStream(streamURL, func(payload []byte) {
			var message depthMessage
			if json.Unmarshal(payload, &message) != nil {
				return
			}
			state := p.states[instrument.Symbol]
			state.applyDepth(normalizeLevels(message.Bids, 6), normalizeLevels(message.Asks, 6))
		}); err != nil {
			time.Sleep(2 * time.Second)
		}
	}
}

func (p *BinanceProvider) consumeStream(target string, onMessage func([]byte)) error {
	conn, reader, err := dialWebSocket(target)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		payload, opcode, err := readFrame(reader)
		if err != nil {
			return err
		}
		switch opcode {
		case 0x1:
			onMessage(payload)
		case 0x8:
			return fmt.Errorf("websocket closed")
		case 0x9:
			if err := writeFrame(conn, 0xA, payload); err != nil {
				return err
			}
		}
	}
}

func (p *BinanceProvider) fetchTicker(instrument Instrument) (tickerSnapshot, error) {
	path := "/api/v3/ticker/24hr"
	if instrument.InstrumentType == "crypto_perpetual" {
		path = "/fapi/v1/ticker/24hr"
	}
	target := instrument.RestBaseURL + path + "?symbol=" + url.QueryEscape(instrument.RawSymbol)
	var response ticker24hResponse
	if err := p.fetchJSON(target, &response); err != nil {
		return tickerSnapshot{}, err
	}
	return tickerSnapshot{
		LastPrice:          parseFloat(response.LastPrice),
		PriceChangePercent: parseFloat(response.PriceChangePercent),
		Volume:             parseFloat(response.Volume),
		HighPrice:          parseFloat(response.HighPrice),
		LowPrice:           parseFloat(response.LowPrice),
	}, nil
}

func (p *BinanceProvider) fetchPrice(instrument Instrument) (float64, error) {
	path := "/api/v3/ticker/price"
	if instrument.InstrumentType == "crypto_perpetual" {
		path = "/fapi/v1/ticker/price"
	}
	target := instrument.RestBaseURL + path + "?symbol=" + url.QueryEscape(instrument.RawSymbol)
	var response priceResponse
	if err := p.fetchJSON(target, &response); err != nil {
		return 0, err
	}
	return parseFloat(response.Price), nil
}

func (p *BinanceProvider) fetchBookTicker(instrument Instrument) (bookTickerSnapshot, error) {
	path := "/api/v3/ticker/bookTicker"
	if instrument.InstrumentType == "crypto_perpetual" {
		path = "/fapi/v1/ticker/bookTicker"
	}
	target := instrument.RestBaseURL + path + "?symbol=" + url.QueryEscape(instrument.RawSymbol)
	var response bookTickerResponse
	if err := p.fetchJSON(target, &response); err != nil {
		return bookTickerSnapshot{}, err
	}
	bestBid := parseFloat(response.BidPrice)
	bestAsk := parseFloat(response.AskPrice)
	lastPrice := 0.0
	if bestBid > 0 && bestAsk > 0 {
		lastPrice = (bestBid + bestAsk) / 2
	}
	return bookTickerSnapshot{
		BestBid:   bestBid,
		BestAsk:   bestAsk,
		BidSize:   parseFloat(response.BidQty),
		AskSize:   parseFloat(response.AskQty),
		LastPrice: lastPrice,
	}, nil
}

func (p *BinanceProvider) fetchDepth(instrument Instrument, limit int) (domain.DepthSnapshot, error) {
	path := "/api/v3/depth"
	if instrument.InstrumentType == "crypto_perpetual" {
		path = "/fapi/v1/depth"
	}
	target := fmt.Sprintf("%s%s?symbol=%s&limit=%d",
		instrument.RestBaseURL,
		path,
		url.QueryEscape(instrument.RawSymbol),
		limit,
	)
	var response depthRESTResponse
	if err := p.fetchJSON(target, &response); err != nil {
		return domain.DepthSnapshot{}, err
	}
	bids := normalizeLevels(response.Bids, 6)
	asks := normalizeLevels(response.Asks, 6)
	if len(bids) == 0 || len(asks) == 0 {
		return domain.DepthSnapshot{}, fmt.Errorf("empty depth")
	}
	priceBase := 0.0
	if bids[0].Price > 0 && asks[0].Price > 0 {
		priceBase = (bids[0].Price + asks[0].Price) / 2
	}
	bestBid, bestAsk, spreadBps, microPrice, depthImbalance := depthMetrics(bids, asks, priceBase)
	return domain.DepthSnapshot{
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		SpreadBps:      spreadBps,
		DepthImbalance: depthImbalance,
		MicroPrice:     microPrice,
		Bids:           bids,
		Asks:           asks,
		LastUpdatedAt:  time.Now().UTC(),
		SignalQuality:  domain.SignalQualityFullDepth,
	}, nil
}

func (p *BinanceProvider) fetchCachedKlines(state *symbolState, interval string, limit int, fallbackPrice float64) ([]domain.Candle, error) {
	state.mu.RLock()
	cache, ok := state.klineCache[interval]
	state.mu.RUnlock()
	if ok && time.Since(cache.fetchedAt) < 15*time.Second && len(cache.candles) > 0 {
		return append([]domain.Candle(nil), cache.candles...), nil
	}

	candles, err := p.fetchKlines(state.instrument, interval, limit)
	if err != nil {
		if ok && len(cache.candles) > 0 {
			return append([]domain.Candle(nil), cache.candles...), nil
		}
		if fallbackPrice > 0 {
			return buildFlatCandles(fallbackPrice, intervalLimit(interval)), nil
		}
		return nil, err
	}

	state.mu.Lock()
	if state.klineCache == nil {
		state.klineCache = make(map[string]cachedKlines)
	}
	state.klineCache[interval] = cachedKlines{
		candles:   append([]domain.Candle(nil), candles...),
		fetchedAt: time.Now().UTC(),
	}
	state.mu.Unlock()
	return candles, nil
}

func (p *BinanceProvider) fetchKlines(instrument Instrument, interval string, limit int) ([]domain.Candle, error) {
	path := "/api/v3/klines"
	if instrument.InstrumentType == "crypto_perpetual" {
		path = "/fapi/v1/klines"
	}

	target := fmt.Sprintf("%s%s?symbol=%s&interval=%s&limit=%d",
		instrument.RestBaseURL,
		path,
		url.QueryEscape(instrument.RawSymbol),
		url.QueryEscape(interval),
		limit,
	)
	var response klineResponse
	if err := p.fetchJSON(target, &response); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	candles := make([]domain.Candle, 0, len(response))
	for index, entry := range response {
		if len(entry) < 6 {
			continue
		}
		openTime, ok := numberFromAny(entry[0])
		if !ok {
			continue
		}
		open := parseStringAny(entry[1])
		high := parseStringAny(entry[2])
		low := parseStringAny(entry[3])
		closeValue := parseStringAny(entry[4])
		volume := parseStringAny(entry[5])
		timestamp := time.UnixMilli(int64(openTime)).UTC()
		isLive := index == len(response)-1 && now.Sub(timestamp) < intervalDuration(interval)
		candles = append(candles, domain.Candle{
			Timestamp: timestamp,
			Open:      round(open, 6),
			High:      round(high, 6),
			Low:       round(low, 6),
			Close:     round(closeValue, 6),
			Volume:    round(volume, 4),
			IsLive:    isLive,
		})
	}
	return candles, nil
}

func (p *BinanceProvider) fetchJSON(target string, out any) error {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "PulseAlpha/1.0")
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("binance status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func buildFlatCandles(price float64, count int) []domain.Candle {
	now := time.Now().UTC()
	candles := make([]domain.Candle, 0, count)
	for i := 0; i < count; i++ {
		ts := now.Add(-time.Duration(count-1-i) * time.Minute).Truncate(time.Minute)
		candles = append(candles, domain.Candle{
			Timestamp: ts,
			Open:      price,
			High:      price,
			Low:       price,
			Close:     price,
			Volume:    0,
			IsLive:    i == count-1,
		})
	}
	return candles
}

func tailCandles(candles []domain.Candle, count int) []domain.Candle {
	if len(candles) <= count {
		return append([]domain.Candle(nil), candles...)
	}
	return append([]domain.Candle(nil), candles[len(candles)-count:]...)
}

func intervalLimit(interval string) int {
	switch interval {
	case "1w":
		return 260
	case "1d":
		return 365
	case "4h", "1h", "15m", "5m", "1m":
		return 240
	default:
		return 120
	}
}

func intervalDuration(interval string) time.Duration {
	switch interval {
	case "1w":
		return 7 * 24 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "1h":
		return time.Hour
	case "15m":
		return 15 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "1m":
		return time.Minute
	default:
		return time.Second
	}
}

func parseStringAny(value any) float64 {
	switch current := value.(type) {
	case string:
		return parseFloat(current)
	case float64:
		return current
	default:
		return 0
	}
}

func numberFromAny(value any) (float64, bool) {
	switch current := value.(type) {
	case float64:
		return current, true
	case int64:
		return float64(current), true
	case int:
		return float64(current), true
	case json.Number:
		parsed, err := current.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (s *symbolState) applyTrade(price float64, quantity float64, timestamp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sec := timestamp.Truncate(time.Second).Unix()
	if s.bucket == nil {
		s.bucket = &candleBucket{second: sec, open: price, high: price, low: price, close: price, volume: quantity}
	} else if sec == s.bucket.second {
		if price > s.bucket.high {
			s.bucket.high = price
		}
		if price < s.bucket.low {
			s.bucket.low = price
		}
		s.bucket.close = price
		s.bucket.volume += quantity
	} else if sec > s.bucket.second {
		prevClose := s.bucket.close
		s.appendBucketLocked(*s.bucket)
		for gap := s.bucket.second + 1; gap < sec; gap++ {
			s.appendBucketLocked(candleBucket{second: gap, open: prevClose, high: prevClose, low: prevClose, close: prevClose})
		}
		s.bucket = &candleBucket{second: sec, open: price, high: price, low: price, close: price, volume: quantity}
	}

	s.lastPrice = price
	s.lastUpdate = timestamp
	s.refreshLiveBucketLocked()
}

func (s *symbolState) applyDepth(bids, asks []domain.DepthLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bestBid, bestAsk, spreadBps, microPrice, depthImbalance := depthMetrics(bids, asks, s.lastPrice)
	s.depth = domain.DepthSnapshot{
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		SpreadBps:      spreadBps,
		DepthImbalance: depthImbalance,
		MicroPrice:     microPrice,
		Bids:           bids,
		Asks:           asks,
		LastUpdatedAt:  time.Now().UTC(),
		SignalQuality:  domain.SignalQualityFullDepth,
	}
	s.lastUpdate = time.Now().UTC()
}

func (s *symbolState) appendBucketLocked(bucket candleBucket) {
	candle := domain.Candle{
		Timestamp: time.Unix(bucket.second, 0).UTC(),
		Open:      round(bucket.open, 6),
		High:      round(bucket.high, 6),
		Low:       round(bucket.low, 6),
		Close:     round(bucket.close, 6),
		Volume:    round(bucket.volume, 4),
		IsLive:    false,
	}
	s.closed = append(s.closed, candle)
	if len(s.closed) > 600 {
		s.closed = s.closed[len(s.closed)-600:]
	}
}

func (s *symbolState) refreshLiveBucketLocked() {
	base := append([]domain.Candle(nil), s.closed...)
	if s.bucket != nil {
		base = append(base, domain.Candle{
			Timestamp: time.Unix(s.bucket.second, 0).UTC(),
			Open:      round(s.bucket.open, 6),
			High:      round(s.bucket.high, 6),
			Low:       round(s.bucket.low, 6),
			Close:     round(s.bucket.close, 6),
			Volume:    round(s.bucket.volume, 4),
			IsLive:    true,
		})
	}
	if len(base) > 180 {
		base = base[len(base)-180:]
	}
	s.candles = base
}

type ticker24hResponse struct {
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
}

type tickerSnapshot struct {
	LastPrice          float64
	PriceChangePercent float64
	Volume             float64
	HighPrice          float64
	LowPrice           float64
}

type priceResponse struct {
	Price string `json:"price"`
}

type bookTickerResponse struct {
	BidPrice string `json:"bidPrice"`
	BidQty   string `json:"bidQty"`
	AskPrice string `json:"askPrice"`
	AskQty   string `json:"askQty"`
}

type bookTickerSnapshot struct {
	BestBid   float64
	BestAsk   float64
	BidSize   float64
	AskSize   float64
	LastPrice float64
}

type depthRESTResponse struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type klineResponse [][]any

type depthMessage struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

type tradeMessage struct {
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	TradeTime int64  `json:"T"`
}

func dialWebSocket(target string) (net.Conn, *bufio.Reader, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, nil, err
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		if parsed.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var conn net.Conn
	if parsed.Scheme == "wss" {
		conn, err = tls.Dial("tcp", host, &tls.Config{ServerName: strings.Split(parsed.Host, ":")[0]})
	} else {
		conn, err = net.Dial("tcp", host)
	}
	if err != nil {
		return nil, nil, err
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := parsed.RequestURI()
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, parsed.Host, key)
	if _, err := conn.Write([]byte(request)); err != nil {
		conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, nil, fmt.Errorf("websocket upgrade failed: %s", strings.TrimSpace(statusLine))
	}

	headers := map[string]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	expectedAccept := websocketAccept(key)
	if headers["sec-websocket-accept"] != expectedAccept {
		conn.Close()
		return nil, nil, fmt.Errorf("invalid websocket accept")
	}

	return conn, reader, nil
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func readFrame(reader *bufio.Reader) ([]byte, byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, 0, err
	}
	opcode := header[0] & 0x0F
	payloadLen := int(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return nil, 0, err
		}
		payloadLen = int(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return nil, 0, err
		}
		payloadLen = int(binary.BigEndian.Uint64(extended))
	}

	masked := header[1]&0x80 != 0
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return nil, 0, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, 0, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return payload, opcode, nil
}

func writeFrame(conn net.Conn, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	maskBit := byte(0x80)
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, maskBit|byte(length))
	case length <= 65535:
		header = append(header, maskBit|126)
		extended := make([]byte, 2)
		binary.BigEndian.PutUint16(extended, uint16(length))
		header = append(header, extended...)
	default:
		header = append(header, maskBit|127)
		extended := make([]byte, 8)
		binary.BigEndian.PutUint64(extended, uint64(length))
		header = append(header, extended...)
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		mask[0] = byte(mathrand.Intn(255))
		mask[1] = byte(mathrand.Intn(255))
		mask[2] = byte(mathrand.Intn(255))
		mask[3] = byte(mathrand.Intn(255))
	}
	header = append(header, mask...)

	maskedPayload := make([]byte, len(payload))
	for i := range payload {
		maskedPayload[i] = payload[i] ^ mask[i%4]
	}
	if _, err := conn.Write(header); err != nil {
		return err
	}
	_, err := conn.Write(maskedPayload)
	return err
}

func normalizeLevels(levels [][]string, limit int) []domain.DepthLevel {
	out := make([]domain.DepthLevel, 0, limit)
	for index, level := range levels {
		if len(out) >= limit || len(level) < 2 {
			break
		}
		out = append(out, domain.DepthLevel{
			Price:  parseFloat(level[0]),
			Size:   parseFloat(level[1]),
			Orders: 1 + index,
		})
	}
	return out
}

func depthMetrics(bids, asks []domain.DepthLevel, fallbackPrice float64) (float64, float64, float64, float64, float64) {
	bestBid := fallbackPrice
	bestAsk := fallbackPrice
	if len(bids) > 0 {
		bestBid = bids[0].Price
	}
	if len(asks) > 0 {
		bestAsk = asks[0].Price
	}
	priceBase := fallbackPrice
	if priceBase <= 0 {
		priceBase = (bestBid + bestAsk) / 2
	}
	spreadBps := 0.0
	if priceBase > 0 {
		spreadBps = ((bestAsk - bestBid) / priceBase) * 10000
	}

	topBid := 0.0
	topAsk := 0.0
	for i := 0; i < minInt(len(bids), 3); i++ {
		topBid += bids[i].Size
	}
	for i := 0; i < minInt(len(asks), 3); i++ {
		topAsk += asks[i].Size
	}
	depthImbalance := 0.5
	if topBid+topAsk > 0 {
		depthImbalance = clamp(0.5+((topBid-topAsk)/(topBid+topAsk))/2, 0.02, 0.98)
	}

	microPrice := priceBase
	if len(bids) > 0 && len(asks) > 0 && bids[0].Size+asks[0].Size > 0 {
		microPrice = (bestAsk*bids[0].Size + bestBid*asks[0].Size) / (bids[0].Size + asks[0].Size)
	}
	return bestBid, bestAsk, round(spreadBps, 2), round(microPrice, 6), round(depthImbalance, 4)
}

func buildDepthFromPrice(price float64, observedAt time.Time) domain.DepthSnapshot {
	step := math.Max(price*0.0003, 0.01)
	bids := make([]domain.DepthLevel, 0, 6)
	asks := make([]domain.DepthLevel, 0, 6)
	for i := 0; i < 6; i++ {
		bids = append(bids, domain.DepthLevel{Price: round(price-step*float64(i+1), 6), Size: round(4.5-float64(i)*0.4, 4), Orders: 1 + i})
		asks = append(asks, domain.DepthLevel{Price: round(price+step*float64(i+1), 6), Size: round(4.2-float64(i)*0.35, 4), Orders: 1 + i})
	}
	bestBid, bestAsk, spreadBps, microPrice, depthImbalance := depthMetrics(bids, asks, price)
	return domain.DepthSnapshot{
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		SpreadBps:      spreadBps,
		DepthImbalance: depthImbalance,
		MicroPrice:     microPrice,
		Bids:           bids,
		Asks:           asks,
		LastUpdatedAt:  observedAt,
		SignalQuality:  domain.SignalQualityDegraded,
	}
}

func buildDepthFromBookTicker(book bookTickerSnapshot, observedAt time.Time) domain.DepthSnapshot {
	bestBid := round(book.BestBid, 6)
	bestAsk := round(book.BestAsk, 6)
	bidSize := math.Max(round(book.BidSize, 4), 1)
	askSize := math.Max(round(book.AskSize, 4), 1)
	priceBase := book.LastPrice
	if priceBase <= 0 {
		priceBase = (bestBid + bestAsk) / 2
	}
	step := math.Max(priceBase*0.00015, 0.01)
	bids := make([]domain.DepthLevel, 0, 6)
	asks := make([]domain.DepthLevel, 0, 6)
	bids = append(bids, domain.DepthLevel{Price: bestBid, Size: bidSize, Orders: 1})
	asks = append(asks, domain.DepthLevel{Price: bestAsk, Size: askSize, Orders: 1})
	for i := 1; i < 6; i++ {
		bids = append(bids, domain.DepthLevel{
			Price:  round(bestBid-step*float64(i), 6),
			Size:   math.Max(round(bidSize-float64(i)*0.2, 4), 1),
			Orders: 1 + i,
		})
		asks = append(asks, domain.DepthLevel{
			Price:  round(bestAsk+step*float64(i), 6),
			Size:   math.Max(round(askSize-float64(i)*0.2, 4), 1),
			Orders: 1 + i,
		})
	}
	_, _, spreadBps, microPrice, depthImbalance := depthMetrics(bids, asks, priceBase)
	return domain.DepthSnapshot{
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		SpreadBps:      spreadBps,
		DepthImbalance: depthImbalance,
		MicroPrice:     microPrice,
		Bids:           bids,
		Asks:           asks,
		LastUpdatedAt:  observedAt,
		SignalQuality:  domain.SignalQualityFullDepth,
	}
}

func baseAssetName(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "BTC"):
		return "Bitcoin"
	case strings.HasPrefix(symbol, "ETH"):
		return "Ethereum"
	case strings.HasPrefix(symbol, "SOL"):
		return "Solana"
	default:
		return symbol
	}
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func clamp(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func round(value float64, digits int) float64 {
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}

func sortInstruments(items []Instrument) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Symbol < items[i].Symbol {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nonZero(value, fallback float64) float64 {
	if value != 0 {
		return value
	}
	return fallback
}

func nonZeroTime(value, fallback time.Time) time.Time {
	if !value.IsZero() {
		return value
	}
	return fallback
}
