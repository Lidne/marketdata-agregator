package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	appinterfaces "main/internal/application/interfaces"
	appinstruments "main/internal/application/service/instruments"
	appmarketdata "main/internal/application/service/marketdata"
	domaininstruments "main/internal/domain/entity/instruments"
	domainmarketdata "main/internal/domain/entity/marketdata"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	instrumentsBasePath = "/api/v1/instruments"
	marketdataBasePath  = "/api/v1/marketdata"
)

var (
	errMissingUID        = errors.New("missing uid")
	errMissingInstrument = errors.New("instrument_uid query param required")
	errMissingRange      = errors.New("from/to query params required")
)

type Handler struct {
	router      *gin.Engine
	instruments *appinstruments.Service
	marketdata  *appmarketdata.Service
	cache       *redis.Client
	cacheTTL    time.Duration
}

var _ appinterfaces.HTTPHandler = (*Handler)(nil)

func NewHandler(inst *appinstruments.Service, md *appmarketdata.Service, cache *redis.Client, cacheTTL time.Duration) *Handler {
	router := gin.New()
	router.Use(gin.Recovery())

	h := &Handler{
		router:      router,
		instruments: inst,
		marketdata:  md,
		cache:       cache,
		cacheTTL:    cacheTTL,
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	inst := h.router.Group(instrumentsBasePath)
	if h.cache != nil {
		inst.Use(h.cacheMiddleware())
	}
	{
		inst.POST("/", h.createInstrument)
		inst.PUT("/", h.updateInstrument)
		inst.GET("/", h.getInstrument)
		inst.DELETE("/", h.deleteInstrument)

		inst.GET("/shares/:uid", h.getShare)
		inst.GET("/bonds/:uid", h.getBond)
		inst.GET("/futures/:uid", h.getFuture)
		inst.GET("/currencies/:uid", h.getCurrency)
		inst.GET("/etfs/:uid", h.getEtf)
	}

	md := h.router.Group(marketdataBasePath)
	if h.cache != nil {
		md.Use(h.cacheMiddleware())
	}
	{
		trades := md.Group("/trades")
		{
			trades.POST("/", h.addTrade)
			trades.POST("/batch", h.addTradesBatch)
			trades.GET("/", h.getTradesRange)
			trades.GET("/last", h.getTradesLast)
		}

		candles := md.Group("/candles")
		{
			candles.POST("/", h.addCandle)
			candles.POST("/batch", h.addCandlesBatch)
			candles.GET("/", h.getCandlesRange)
			candles.GET("/last", h.getCandlesLast)
		}

		orderbooks := md.Group("/orderbooks")
		{
			orderbooks.POST("/", h.addOrderBook)
			orderbooks.POST("/batch", h.addOrderBooksBatch)
			orderbooks.GET("/", h.getOrderBooksRange)
			orderbooks.GET("/last", h.getOrderBooksLast)
		}
	}
}

// Instruments handlers

func (h *Handler) createInstrument(c *gin.Context) {
	var payload instrumentPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	inst, err := payload.toDomain()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateInstrument(c.Request.Context(), inst); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, inst)
}

func (h *Handler) updateInstrument(c *gin.Context) {
	var payload instrumentPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	inst, err := payload.toDomain()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateInstrument(c.Request.Context(), inst); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, inst)
}

func (h *Handler) getInstrument(c *gin.Context) {
	uidStr := c.Query("uid")
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	inst, err := h.instruments.GetInstrument(c.Request.Context(), uid)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, inst)
}

func (h *Handler) deleteInstrument(c *gin.Context) {
	uidStr := c.Query("uid")
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	if err := h.instruments.DeleteInstrument(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) getShare(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetShare(ctx, uid)
	})
}

func (h *Handler) getBond(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetBond(ctx, uid)
	})
}

func (h *Handler) getFuture(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetFuture(ctx, uid)
	})
}

func (h *Handler) getCurrency(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetCurrency(ctx, uid)
	})
}

func (h *Handler) getEtf(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetEtf(ctx, uid)
	})
}

func (h *Handler) handleTypedInstrument(c *gin.Context, fn func(ctx context.Context, uid uuid.UUID) (interface{}, error)) {
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	result, err := fn(c.Request.Context(), uid)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Market data handlers

func (h *Handler) addTrade(c *gin.Context) {
	var trade domainmarketdata.Trade
	if err := c.ShouldBindJSON(&trade); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddTrade(c.Request.Context(), &trade); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) addTradesBatch(c *gin.Context) {
	var trades []domainmarketdata.Trade
	if err := c.ShouldBindJSON(&trades); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddTrades(c.Request.Context(), trades); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) getTradesRange(c *gin.Context) {
	instrumentUID, err := parseUUIDQuery(c, "instrument_uid")
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingInstrument)
		return
	}
	from, to, err := parseTimeRange(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingRange)
		return
	}
	trades, err := h.marketdata.GetTradesBetween(c.Request.Context(), instrumentUID, from, to)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, trades)
}

func (h *Handler) getTradesLast(c *gin.Context) {
	instrumentUID, limit, err := h.parseInstrumentAndLimit(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	trades, err := h.marketdata.GetLastTrades(c.Request.Context(), instrumentUID, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, trades)
}

func (h *Handler) addCandle(c *gin.Context) {
	var candle domainmarketdata.Candle
	if err := c.ShouldBindJSON(&candle); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddCandle(c.Request.Context(), &candle); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) addCandlesBatch(c *gin.Context) {
	var candles []domainmarketdata.Candle
	if err := c.ShouldBindJSON(&candles); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddCandles(c.Request.Context(), candles); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) getCandlesRange(c *gin.Context) {
	instrumentUID, err := parseUUIDQuery(c, "instrument_uid")
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingInstrument)
		return
	}
	from, to, err := parseTimeRange(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingRange)
		return
	}
	intervalSeconds, err := parseInt64Query(c, "interval_seconds")
	if err != nil {
		writeError(c, http.StatusBadRequest, fmt.Errorf("interval_seconds query param required"))
		return
	}
	candles, err := h.marketdata.GetCandlesBetween(c.Request.Context(), instrumentUID, intervalSeconds, from, to)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, candles)
}

func (h *Handler) getCandlesLast(c *gin.Context) {
	instrumentUID, limit, interval, err := h.parseInstrumentLimitInterval(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	candles, err := h.marketdata.GetLastCandles(c.Request.Context(), instrumentUID, interval, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, candles)
}

func (h *Handler) addOrderBook(c *gin.Context) {
	var snapshot domainmarketdata.OrderBookSnapshot
	if err := c.ShouldBindJSON(&snapshot); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddOrderBookSnapshot(c.Request.Context(), &snapshot); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) addOrderBooksBatch(c *gin.Context) {
	var snapshots []domainmarketdata.OrderBookSnapshot
	if err := c.ShouldBindJSON(&snapshots); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.marketdata.AddOrderBookSnapshots(c.Request.Context(), snapshots); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusCreated)
}

func (h *Handler) getOrderBooksRange(c *gin.Context) {
	instrumentUID, err := parseUUIDQuery(c, "instrument_uid")
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingInstrument)
		return
	}
	from, to, err := parseTimeRange(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, errMissingRange)
		return
	}
	depth, err := parseIntQuery(c, "depth")
	if err != nil {
		writeError(c, http.StatusBadRequest, fmt.Errorf("depth query param required"))
		return
	}
	snapshots, err := h.marketdata.GetOrderBookSnapshotsBetween(c.Request.Context(), instrumentUID, int32(depth), from, to)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

func (h *Handler) getOrderBooksLast(c *gin.Context) {
	instrumentUID, limit, err := h.parseInstrumentAndLimit(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	depth, err := parseIntQuery(c, "depth")
	if err != nil {
		writeError(c, http.StatusBadRequest, fmt.Errorf("depth query param required"))
		return
	}
	snapshots, err := h.marketdata.GetLastOrderBookSnapshots(c.Request.Context(), instrumentUID, int32(depth), limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

// Helpers

type instrumentPayload struct {
	UID       string `json:"uid,omitempty"`
	Figi      string `json:"figi"`
	Ticker    string `json:"ticker"`
	Lot       int32  `json:"lot"`
	ClassCode string `json:"class_code"`
	LogoURL   string `json:"logo_url"`
}

func (p instrumentPayload) toDomain() (*domaininstruments.Instrument, error) {
	inst := &domaininstruments.Instrument{
		Figi:      p.Figi,
		Ticker:    p.Ticker,
		Lot:       p.Lot,
		ClassCode: p.ClassCode,
		LogoURL:   p.LogoURL,
	}
	if p.UID != "" {
		uid, err := uuid.Parse(p.UID)
		if err != nil {
			return nil, err
		}
		inst.UID = uid
	}
	return inst, nil
}

func (h *Handler) parseInstrumentAndLimit(c *gin.Context) (uuid.UUID, int, error) {
	instrumentUID, err := parseUUIDQuery(c, "instrument_uid")
	if err != nil {
		return uuid.UUID{}, 0, errMissingInstrument
	}
	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return uuid.UUID{}, 0, fmt.Errorf("limit query param required")
	}
	if limit <= 0 {
		return uuid.UUID{}, 0, fmt.Errorf("limit must be positive")
	}
	return instrumentUID, limit, nil
}

func (h *Handler) parseInstrumentLimitInterval(c *gin.Context) (uuid.UUID, int, int64, error) {
	instrumentUID, limit, err := h.parseInstrumentAndLimit(c)
	if err != nil {
		return uuid.UUID{}, 0, 0, err
	}
	intervalSeconds, err := parseInt64Query(c, "interval_seconds")
	if err != nil {
		return uuid.UUID{}, 0, 0, fmt.Errorf("interval_seconds query param required")
	}
	return instrumentUID, limit, intervalSeconds, nil
}

func writeError(c *gin.Context, status int, err error) {
	if err == nil {
		status = http.StatusInternalServerError
		err = errors.New("unknown error")
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

// cacheMiddleware caches GET responses in Redis.
func (h *Handler) cacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.cache == nil || c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		key := h.cacheKey(c)
		ctx := c.Request.Context()

		if cached, err := h.cache.Get(ctx, key).Result(); err == nil {
			c.Data(http.StatusOK, "application/json", []byte(cached))
			c.Abort()
			return
		}

		recorder := &responseRecorder{
			ResponseWriter: c.Writer,
			status:         http.StatusOK,
			body:           &bytes.Buffer{},
		}
		c.Writer = recorder

		c.Next()

		if recorder.status >= 200 && recorder.status < 300 && recorder.body.Len() > 0 {
			_ = h.cache.Set(ctx, key, recorder.body.Bytes(), h.cacheTTL).Err()
		}
	}
}

type responseRecorder struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if len(data) > 0 {
		r.body.Write(data)
	}
	return r.ResponseWriter.Write(data)
}

func (h *Handler) cacheKey(c *gin.Context) string {
	return fmt.Sprintf("cache:%s:%s?%s", c.Request.Method, c.FullPath(), c.Request.URL.RawQuery)
}

func parseUUIDQuery(c *gin.Context, key string) (uuid.UUID, error) {
	value := c.Query(key)
	if value == "" {
		return uuid.Nil, fmt.Errorf("%s query param required", key)
	}
	return uuid.Parse(value)
}

func parseIntQuery(c *gin.Context, key string) (int, error) {
	value := c.Query(key)
	if value == "" {
		return 0, fmt.Errorf("%s query param required", key)
	}
	return strconv.Atoi(value)
}

func parseInt64Query(c *gin.Context, key string) (int64, error) {
	value := c.Query(key)
	if value == "" {
		return 0, fmt.Errorf("%s query param required", key)
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		return time.Time{}, time.Time{}, errMissingRange
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, to, nil
}
