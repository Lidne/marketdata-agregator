// @title           Market Data Aggregator API
// @version         1.0
// @description     API for managing financial instruments and market data
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

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
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
	h.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	inst := h.router.Group(instrumentsBasePath)
	if h.cache != nil {
		inst.Use(h.cacheMiddleware())
	}
	{
		inst.POST("/", h.createInstrument)
		inst.PUT("/", h.updateInstrument)
		inst.GET("/", h.getInstrument)
		inst.DELETE("/", h.deleteInstrument)

		inst.POST("/shares", h.createShare)
		inst.PUT("/shares", h.updateShare)
		inst.DELETE("/shares/:uid", h.deleteShare)
		inst.GET("/shares/:uid", h.getShare)

		inst.POST("/bonds", h.createBond)
		inst.PUT("/bonds", h.updateBond)
		inst.DELETE("/bonds/:uid", h.deleteBond)
		inst.GET("/bonds/:uid", h.getBond)

		inst.POST("/futures", h.createFuture)
		inst.PUT("/futures", h.updateFuture)
		inst.DELETE("/futures/:uid", h.deleteFuture)
		inst.GET("/futures/:uid", h.getFuture)

		inst.POST("/currencies", h.createCurrency)
		inst.PUT("/currencies", h.updateCurrency)
		inst.DELETE("/currencies/:uid", h.deleteCurrency)
		inst.GET("/currencies/:uid", h.getCurrency)

		inst.POST("/etfs", h.createEtf)
		inst.PUT("/etfs", h.updateEtf)
		inst.DELETE("/etfs/:uid", h.deleteEtf)
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

// createInstrument creates a new instrument
// @Summary      Create instrument
// @Description  Create a new financial instrument
// @Tags         instruments
// @Accept       json
// @Produce      json
// @Param        instrument  body      instrumentPayload  true  "Instrument data"
// @Success      201         {object}  domaininstruments.Instrument
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /instruments [post]
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

// updateInstrument updates an existing instrument
// @Summary      Update instrument
// @Description  Update an existing financial instrument
// @Tags         instruments
// @Accept       json
// @Produce      json
// @Param        instrument  body      instrumentPayload  true  "Instrument data with UID"
// @Success      200         {object}  domaininstruments.Instrument
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /instruments [put]
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

// getInstrument retrieves an instrument by UID
// @Summary      Get instrument
// @Description  Get a financial instrument by UID
// @Tags         instruments
// @Accept       json
// @Produce      json
// @Param        uid   query     string  true  "Instrument UID"
// @Success      200   {object}  domaininstruments.Instrument
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments [get]
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

// deleteInstrument deletes an instrument by UID
// @Summary      Delete instrument
// @Description  Delete a financial instrument by UID
// @Tags         instruments
// @Accept       json
// @Produce      json
// @Param        uid   query     string  true  "Instrument UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments [delete]
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

// createShare creates a new share instrument
// @Summary      Create share
// @Description  Create a share instrument along with its base instrument record
// @Tags         shares
// @Accept       json
// @Produce      json
// @Param        share  body      sharePayload  true  "Share data"
// @Success      201    {object}  domaininstruments.Share
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Router       /instruments/shares [post]
func (h *Handler) createShare(c *gin.Context) {
	var payload sharePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	share, err := payload.toDomainShare()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateShare(c.Request.Context(), share); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, share)
}

// updateShare updates an existing share instrument
// @Summary      Update share
// @Description  Update a share instrument and its base data
// @Tags         shares
// @Accept       json
// @Produce      json
// @Param        share  body      sharePayload  true  "Share data with UID"
// @Success      200    {object}  domaininstruments.Share
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Router       /instruments/shares [put]
func (h *Handler) updateShare(c *gin.Context) {
	var payload sharePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	share, err := payload.toDomainShare()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateShare(c.Request.Context(), share); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, share)
}

// deleteShare deletes a share instrument
// @Summary      Delete share
// @Description  Delete a share instrument by UID
// @Tags         shares
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Share UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/shares/{uid} [delete]
func (h *Handler) deleteShare(c *gin.Context) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.DeleteShare(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// createBond creates a bond instrument
// @Summary      Create bond
// @Description  Create a bond instrument along with its base instrument record
// @Tags         bonds
// @Accept       json
// @Produce      json
// @Param        bond  body      bondPayload  true  "Bond data"
// @Success      201   {object}  domaininstruments.Bond
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/bonds [post]
func (h *Handler) createBond(c *gin.Context) {
	var payload bondPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	bond, err := payload.toDomainBond()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateBond(c.Request.Context(), bond); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, bond)
}

// updateBond updates a bond instrument
// @Summary      Update bond
// @Description  Update a bond instrument and its base data
// @Tags         bonds
// @Accept       json
// @Produce      json
// @Param        bond  body      bondPayload  true  "Bond data with UID"
// @Success      200   {object}  domaininstruments.Bond
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/bonds [put]
func (h *Handler) updateBond(c *gin.Context) {
	var payload bondPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	bond, err := payload.toDomainBond()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateBond(c.Request.Context(), bond); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, bond)
}

// deleteBond deletes a bond instrument
// @Summary      Delete bond
// @Description  Delete a bond instrument by UID
// @Tags         bonds
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Bond UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/bonds/{uid} [delete]
func (h *Handler) deleteBond(c *gin.Context) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.DeleteBond(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// createFuture creates a future instrument
// @Summary      Create future
// @Description  Create a future instrument along with its base instrument record
// @Tags         futures
// @Accept       json
// @Produce      json
// @Param        future  body      futurePayload  true  "Future data"
// @Success      201     {object}  domaininstruments.Future
// @Failure      400     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /instruments/futures [post]
func (h *Handler) createFuture(c *gin.Context) {
	var payload futurePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	future, err := payload.toDomainFuture()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateFuture(c.Request.Context(), future); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, future)
}

// updateFuture updates a future instrument
// @Summary      Update future
// @Description  Update a future instrument and its base data
// @Tags         futures
// @Accept       json
// @Produce      json
// @Param        future  body      futurePayload  true  "Future data with UID"
// @Success      200     {object}  domaininstruments.Future
// @Failure      400     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /instruments/futures [put]
func (h *Handler) updateFuture(c *gin.Context) {
	var payload futurePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	future, err := payload.toDomainFuture()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateFuture(c.Request.Context(), future); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, future)
}

// deleteFuture deletes a future instrument
// @Summary      Delete future
// @Description  Delete a future instrument by UID
// @Tags         futures
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Future UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/futures/{uid} [delete]
func (h *Handler) deleteFuture(c *gin.Context) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.DeleteFuture(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// createCurrency creates a currency instrument
// @Summary      Create currency
// @Description  Create a currency instrument along with its base instrument record
// @Tags         currencies
// @Accept       json
// @Produce      json
// @Param        currency  body      currencyPayload  true  "Currency data"
// @Success      201       {object}  domaininstruments.Currency
// @Failure      400       {object}  map[string]string
// @Failure      500       {object}  map[string]string
// @Router       /instruments/currencies [post]
func (h *Handler) createCurrency(c *gin.Context) {
	var payload currencyPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	currency, err := payload.toDomainCurrency()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateCurrency(c.Request.Context(), currency); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, currency)
}

// updateCurrency updates a currency instrument
// @Summary      Update currency
// @Description  Update a currency instrument and its base data
// @Tags         currencies
// @Accept       json
// @Produce      json
// @Param        currency  body      currencyPayload  true  "Currency data with UID"
// @Success      200       {object}  domaininstruments.Currency
// @Failure      400       {object}  map[string]string
// @Failure      500       {object}  map[string]string
// @Router       /instruments/currencies [put]
func (h *Handler) updateCurrency(c *gin.Context) {
	var payload currencyPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	currency, err := payload.toDomainCurrency()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateCurrency(c.Request.Context(), currency); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, currency)
}

// deleteCurrency deletes a currency instrument
// @Summary      Delete currency
// @Description  Delete a currency instrument by UID
// @Tags         currencies
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Currency UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/currencies/{uid} [delete]
func (h *Handler) deleteCurrency(c *gin.Context) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.DeleteCurrency(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// createEtf creates an ETF instrument
// @Summary      Create ETF
// @Description  Create an ETF instrument along with its base instrument record
// @Tags         etfs
// @Accept       json
// @Produce      json
// @Param        etf  body      etfPayload  true  "ETF data"
// @Success      201  {object}  domaininstruments.Etf
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /instruments/etfs [post]
func (h *Handler) createEtf(c *gin.Context) {
	var payload etfPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	etf, err := payload.toDomainEtf()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.CreateEtf(c.Request.Context(), etf); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusCreated, etf)
}

// updateEtf updates an ETF instrument
// @Summary      Update ETF
// @Description  Update an ETF instrument and its base data
// @Tags         etfs
// @Accept       json
// @Produce      json
// @Param        etf  body      etfPayload  true  "ETF data with UID"
// @Success      200  {object}  domaininstruments.Etf
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /instruments/etfs [put]
func (h *Handler) updateEtf(c *gin.Context) {
	var payload etfPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if payload.UID == "" {
		writeError(c, http.StatusBadRequest, errMissingUID)
		return
	}
	etf, err := payload.toDomainEtf()
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.UpdateEtf(c.Request.Context(), etf); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, etf)
}

// deleteEtf deletes an ETF instrument
// @Summary      Delete ETF
// @Description  Delete an ETF instrument by UID
// @Tags         etfs
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "ETF UID"
// @Success      204   "No Content"
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/etfs/{uid} [delete]
func (h *Handler) deleteEtf(c *gin.Context) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.instruments.DeleteEtf(c.Request.Context(), uid); err != nil {
		writeError(c, http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// getShare retrieves a share instrument by UID
// @Summary      Get share
// @Description  Get a share instrument by UID
// @Tags         shares
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Share UID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/shares/{uid} [get]
func (h *Handler) getShare(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetShare(ctx, uid)
	})
}

// getBond retrieves a bond instrument by UID
// @Summary      Get bond
// @Description  Get a bond instrument by UID
// @Tags         bonds
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Bond UID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/bonds/{uid} [get]
func (h *Handler) getBond(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetBond(ctx, uid)
	})
}

// getFuture retrieves a future instrument by UID
// @Summary      Get future
// @Description  Get a future instrument by UID
// @Tags         futures
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Future UID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/futures/{uid} [get]
func (h *Handler) getFuture(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetFuture(ctx, uid)
	})
}

// getCurrency retrieves a currency instrument by UID
// @Summary      Get currency
// @Description  Get a currency instrument by UID
// @Tags         currencies
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "Currency UID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/currencies/{uid} [get]
func (h *Handler) getCurrency(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetCurrency(ctx, uid)
	})
}

// getEtf retrieves an ETF instrument by UID
// @Summary      Get ETF
// @Description  Get an ETF instrument by UID
// @Tags         etfs
// @Accept       json
// @Produce      json
// @Param        uid   path      string  true  "ETF UID"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /instruments/etfs/{uid} [get]
func (h *Handler) getEtf(c *gin.Context) {
	h.handleTypedInstrument(c, func(ctx context.Context, uid uuid.UUID) (interface{}, error) {
		return h.instruments.GetEtf(ctx, uid)
	})
}

func (h *Handler) handleTypedInstrument(c *gin.Context, fn func(ctx context.Context, uid uuid.UUID) (interface{}, error)) {
	uid, err := parseUIDParam(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err)
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

// addTrade adds a single trade
// @Summary      Add trade
// @Description  Add a single trade record
// @Tags         trades
// @Accept       json
// @Produce      json
// @Param        trade  body      domainmarketdata.Trade  true  "Trade data"
// @Success      201    "Created"
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Router       /marketdata/trades [post]
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

// addTradesBatch adds multiple trades in a batch
// @Summary      Add trades batch
// @Description  Add multiple trade records in a single request
// @Tags         trades
// @Accept       json
// @Produce      json
// @Param        trades  body      []domainmarketdata.Trade  true  "Array of trade data"
// @Success      201     "Created"
// @Failure      400     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /marketdata/trades/batch [post]
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

// getTradesRange retrieves trades within a time range
// @Summary      Get trades range
// @Description  Get trades for an instrument within a time range
// @Tags         trades
// @Accept       json
// @Produce      json
// @Param        instrument_uid  query     string  true  "Instrument UID"
// @Param        from            query     string  true  "Start time (RFC3339)"
// @Param        to              query     string  true  "End time (RFC3339)"
// @Success      200             {array}   domainmarketdata.Trade
// @Failure      400             {object}  map[string]string
// @Failure      500             {object}  map[string]string
// @Router       /marketdata/trades [get]
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

// getTradesLast retrieves the last N trades
// @Summary      Get last trades
// @Description  Get the last N trades for an instrument
// @Tags         trades
// @Accept       json
// @Produce      json
// @Param        instrument_uid  query     string  true  "Instrument UID"
// @Param        limit           query     int     true  "Number of trades to retrieve"
// @Success      200             {array}   domainmarketdata.Trade
// @Failure      400             {object}  map[string]string
// @Failure      500             {object}  map[string]string
// @Router       /marketdata/trades/last [get]
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

// addCandle adds a single candle
// @Summary      Add candle
// @Description  Add a single candle record
// @Tags         candles
// @Accept       json
// @Produce      json
// @Param        candle  body      domainmarketdata.Candle  true  "Candle data"
// @Success      201     "Created"
// @Failure      400     {object}  map[string]string
// @Failure      500     {object}  map[string]string
// @Router       /marketdata/candles [post]
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

// addCandlesBatch adds multiple candles in a batch
// @Summary      Add candles batch
// @Description  Add multiple candle records in a single request
// @Tags         candles
// @Accept       json
// @Produce      json
// @Param        candles  body      []domainmarketdata.Candle  true  "Array of candle data"
// @Success      201      "Created"
// @Failure      400      {object}  map[string]string
// @Failure      500      {object}  map[string]string
// @Router       /marketdata/candles/batch [post]
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

// getCandlesRange retrieves candles within a time range
// @Summary      Get candles range
// @Description  Get candles for an instrument within a time range
// @Tags         candles
// @Accept       json
// @Produce      json
// @Param        instrument_uid   query     string  true  "Instrument UID"
// @Param        interval_seconds query     int64   true  "Candle interval in seconds"
// @Param        from             query     string  true  "Start time (RFC3339)"
// @Param        to               query     string  true  "End time (RFC3339)"
// @Success      200              {array}   domainmarketdata.Candle
// @Failure      400              {object}  map[string]string
// @Failure      500              {object}  map[string]string
// @Router       /marketdata/candles [get]
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

// getCandlesLast retrieves the last N candles
// @Summary      Get last candles
// @Description  Get the last N candles for an instrument
// @Tags         candles
// @Accept       json
// @Produce      json
// @Param        instrument_uid   query     string  true  "Instrument UID"
// @Param        interval_seconds query     int64   true  "Candle interval in seconds"
// @Param        limit            query     int     true  "Number of candles to retrieve"
// @Success      200              {array}   domainmarketdata.Candle
// @Failure      400              {object}  map[string]string
// @Failure      500              {object}  map[string]string
// @Router       /marketdata/candles/last [get]
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

// addOrderBook adds a single order book snapshot
// @Summary      Add order book
// @Description  Add a single order book snapshot
// @Tags         orderbooks
// @Accept       json
// @Produce      json
// @Param        orderbook  body      domainmarketdata.OrderBookSnapshot  true  "Order book snapshot data"
// @Success      201        "Created"
// @Failure      400        {object}  map[string]string
// @Failure      500        {object}  map[string]string
// @Router       /marketdata/orderbooks [post]
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

// addOrderBooksBatch adds multiple order book snapshots in a batch
// @Summary      Add order books batch
// @Description  Add multiple order book snapshots in a single request
// @Tags         orderbooks
// @Accept       json
// @Produce      json
// @Param        orderbooks  body      []domainmarketdata.OrderBookSnapshot  true  "Array of order book snapshot data"
// @Success      201         "Created"
// @Failure      400         {object}  map[string]string
// @Failure      500         {object}  map[string]string
// @Router       /marketdata/orderbooks/batch [post]
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

// getOrderBooksRange retrieves order book snapshots within a time range
// @Summary      Get order books range
// @Description  Get order book snapshots for an instrument within a time range
// @Tags         orderbooks
// @Accept       json
// @Produce      json
// @Param        instrument_uid  query     string  true  "Instrument UID"
// @Param        depth           query     int     true  "Order book depth"
// @Param        from            query     string  true  "Start time (RFC3339)"
// @Param        to              query     string  true  "End time (RFC3339)"
// @Success      200             {array}   domainmarketdata.OrderBookSnapshot
// @Failure      400             {object}  map[string]string
// @Failure      500             {object}  map[string]string
// @Router       /marketdata/orderbooks [get]
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

// getOrderBooksLast retrieves the last N order book snapshots
// @Summary      Get last order books
// @Description  Get the last N order book snapshots for an instrument
// @Tags         orderbooks
// @Accept       json
// @Produce      json
// @Param        instrument_uid  query     string  true  "Instrument UID"
// @Param        depth           query     int     true  "Order book depth"
// @Param        limit           query     int     true  "Number of snapshots to retrieve"
// @Success      200             {array}   domainmarketdata.OrderBookSnapshot
// @Failure      400             {object}  map[string]string
// @Failure      500             {object}  map[string]string
// @Router       /marketdata/orderbooks/last [get]
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

type sharePayload struct {
	instrumentPayload
}

func (p sharePayload) toDomainShare() (*domaininstruments.Share, error) {
	inst, err := p.instrumentPayload.toDomain()
	if err != nil {
		return nil, err
	}
	return &domaininstruments.Share{Instrument: *inst}, nil
}

type bondPayload struct {
	instrumentPayload
	Nominal  float64 `json:"nominal"`
	AciValue float64 `json:"aci_value"`
}

func (p bondPayload) toDomainBond() (*domaininstruments.Bond, error) {
	inst, err := p.instrumentPayload.toDomain()
	if err != nil {
		return nil, err
	}
	return &domaininstruments.Bond{
		Instrument: *inst,
		Nominal:    p.Nominal,
		AciValue:   p.AciValue,
	}, nil
}

type futurePayload struct {
	instrumentPayload
	MinPriceIncrement       float64 `json:"min_price_increment"`
	MinPriceIncrementAmount float64 `json:"min_price_increment_amount"`
	AssetType               string  `json:"asset_type"`
}

func (p futurePayload) toDomainFuture() (*domaininstruments.Future, error) {
	inst, err := p.instrumentPayload.toDomain()
	if err != nil {
		return nil, err
	}
	assetType, err := domaininstruments.NewAssetType(p.AssetType)
	if err != nil {
		return nil, err
	}
	return &domaininstruments.Future{
		Instrument:              *inst,
		MinPriceIncrement:       p.MinPriceIncrement,
		MinPriceIncrementAmount: p.MinPriceIncrementAmount,
		AssetType:               assetType,
	}, nil
}

type currencyPayload struct {
	instrumentPayload
}

func (p currencyPayload) toDomainCurrency() (*domaininstruments.Currency, error) {
	inst, err := p.instrumentPayload.toDomain()
	if err != nil {
		return nil, err
	}
	return &domaininstruments.Currency{Instrument: *inst}, nil
}

type etfPayload struct {
	instrumentPayload
	MinPriceIncrement float64 `json:"min_price_increment"`
}

func (p etfPayload) toDomainEtf() (*domaininstruments.Etf, error) {
	inst, err := p.instrumentPayload.toDomain()
	if err != nil {
		return nil, err
	}
	return &domaininstruments.Etf{
		Instrument:        *inst,
		MinPriceIncrement: p.MinPriceIncrement,
	}, nil
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

func parseUIDParam(c *gin.Context) (uuid.UUID, error) {
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		return uuid.Nil, errMissingUID
	}
	return uid, nil
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
