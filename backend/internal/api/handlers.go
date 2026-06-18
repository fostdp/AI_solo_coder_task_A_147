package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/life_predictor"
	"noria-bearing-system/internal/modules/messages"
	"noria-bearing-system/internal/modules/wear_simulator"
)

var Version = "dev"

func SetVersion(v string) {
	Version = v
}

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

func CORSMiddleware(origins []string) gin.HandlerFunc {
	originMap := make(map[string]bool)
	for _, o := range origins {
		originMap[o] = true
	}

	return func(c *gin.Context) {
		reqOrigin := c.Request.Header.Get("Origin")
		if _, ok := originMap[reqOrigin]; ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", reqOrigin)
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (h *Handler) GetNoriaWheels(c *gin.Context) {
	ctx := context.Background()
	wheels, err := database.Instance.GetAllNoriaWheels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wheels)
}

func (h *Handler) GetBearings(c *gin.Context) {
	ctx := context.Background()
	noriaIDStr := c.Query("noria_wheel_id")
	var bearings []models.Bearing
	var err error

	if noriaIDStr != "" {
		noriaID, _ := strconv.Atoi(noriaIDStr)
		bearings, err = database.Instance.GetBearingsByNoriaWheel(ctx, noriaID)
	} else {
		bearings, err = database.Instance.GetAllBearings(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bearings)
}

func (h *Handler) GetBearingByID(c *gin.Context) {
	ctx := context.Background()
	idStr := c.Param("id")
	bearingID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, b := range bearings {
		if b.ID == bearingID {
			c.JSON(http.StatusOK, b)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "轴承未找到"})
}

func (h *Handler) GetSensorData(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	hoursStr := c.DefaultQuery("hours", "24")
	hours, _ := strconv.Atoi(hoursStr)
	if hours <= 0 {
		hours = 24
	}

	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	data, err := database.Instance.GetSensorDataByTimeRange(ctx, bearingID, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetLatestSensorData(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	data, err := database.Instance.GetLatestSensorData(ctx, bearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetWearHistory(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	data, err := database.Instance.GetWearHistory(ctx, bearingID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetLatestWearResult(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	data, err := database.Instance.GetLatestWearResult(ctx, bearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetLatestLifePrediction(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	data, err := database.Instance.GetLatestLifePrediction(ctx, bearingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetBearingStatuses(c *gin.Context) {
	ctx := context.Background()
	statuses, err := database.Instance.GetBearingLatestStatus(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, statuses)
}

func (h *Handler) GetRecentAlerts(c *gin.Context) {
	ctx := context.Background()
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	alerts, err := database.Instance.GetRecentAlerts(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alerts)
}

func (h *Handler) PostSensorData(c *gin.Context) {
	ctx := context.Background()
	var data models.SensorData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.Time.IsZero() {
		data.Time = time.Now()
	}
	if data.Source == "" {
		data.Source = "api"
	}

	if err := database.Instance.InsertSensorData(ctx, &data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "数据已保存", "data": data})
}

func (h *Handler) GetOilFilmMap(c *gin.Context) {
	ctx := context.Background()
	bearingID, err := strconv.Atoi(c.Param("bearing_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的轴承ID"})
		return
	}

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var bearing *models.Bearing
	for i := range bearings {
		if bearings[i].ID == bearingID {
			bearing = &bearings[i]
			break
		}
	}

	if bearing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "轴承未找到"})
		return
	}

	sensorData, err := database.Instance.GetLatestSensorData(ctx, bearingID)
	if err != nil {
		log.Printf("获取传感器数据失败: %v", err)
	}

	avgLoad := 5000.0
	avgSpeed := 15.0
	avgTemp := 35.0
	avgFilm := bearing.OilViscosityPaS * 100
	if avgFilm > 5 {
		avgFilm = 3
	}

	if sensorData != nil {
		avgLoad = sensorData.RadialLoad
		avgSpeed = sensorData.RotationalSpeed
		avgTemp = sensorData.Temperature
		avgFilm = sensorData.OilFilmThickness
	}

	grid := wear_simulator.GenerateOilFilmMap(*bearing, avgLoad, avgSpeed, avgTemp, avgFilm)

	c.JSON(http.StatusOK, gin.H{
		"bearing_id":  bearingID,
		"grid_size_x": grid.GridSizeX,
		"grid_size_y": grid.GridSizeY,
		"captured_at": time.Now(),
		"data":        grid.Data,
	})
}

func (h *Handler) TriggerCalculation(c *gin.Context) {
	ctx := context.Background()
	type reqBody struct {
		BearingID int `json:"bearing_id"`
	}
	var body reqBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var targetBearing *models.Bearing
	for i := range bearings {
		if bearings[i].ID == body.BearingID {
			targetBearing = &bearings[i]
			break
		}
	}

	if targetBearing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "轴承未找到"})
		return
	}

	now := time.Now()
	periodStart := now.Add(-1 * time.Hour)
	sensorData, _ := database.Instance.GetSensorDataByTimeRange(ctx, body.BearingID, periodStart, now)

	previousTotal := targetBearing.InitialWearMicrom
	lastWear, err := database.Instance.GetLatestWearResult(ctx, body.BearingID)
	if err == nil && lastWear != nil {
		previousTotal = lastWear.TotalWearMicrom
	}

	wearReq := &messages.WearCalcRequest{
		Bearing:       *targetBearing,
		SensorData:    sensorData,
		PreviousTotal: previousTotal,
		PeriodStart:   periodStart,
		PeriodEnd:     now,
		RequestID:     "api-" + strconv.Itoa(body.BearingID),
	}

	tempWearChan := make(chan messages.WearCalcResult, 1)
	syncWearCalc := wear_simulator.NewWearSimulator(make(chan messages.WearCalcRequest, 1), tempWearChan)
	wearResult := syncWearCalc.Calculate(wearReq)

	wearDB := &models.WearResult{
		BearingID:             body.BearingID,
		CalculatedAt:          now,
		PeriodStart:           periodStart,
		PeriodEnd:             now,
		WearDepthMicrom:       wearResult.WearDepthMicrom,
		WearRateMicromPerHour: &wearResult.WearRateMicromPerHour,
		TotalWearMicrom:       wearResult.TotalWearMicrom,
		ArchardWearVolume:     &wearResult.ArchardWearVolume,
		EHLFilmParameter:      &wearResult.EHLFilmParameter,
		SlidingDistance:       &wearResult.SlidingDistance,
		WearCoefficient:       &wearResult.WearCoefficient,
		ContactPressure:       &wearResult.ContactPressure,
	}
	_ = database.Instance.InsertWearResult(ctx, wearDB)

	wearHistory, _ := database.Instance.GetWearHistory(ctx, body.BearingID, 100)
	runningHours := time.Since(targetBearing.InstalledAt).Hours()

	lifeReq := &messages.LifePredRequest{
		Bearing:      *targetBearing,
		WearHistory:  wearHistory,
		CurrentWear:  wearResult.TotalWearMicrom,
		RunningHours: runningHours,
		RequestID:    "api-" + strconv.Itoa(body.BearingID),
	}

	tempLifeChan := make(chan messages.LifePredResult, 1)
	syncLifePred := life_predictor.NewLifePredictor(make(chan messages.LifePredRequest, 1), tempLifeChan)
	lifeResult := syncLifePred.Predict(lifeReq)

	lifeDB := &models.LifePrediction{
		BearingID:              body.BearingID,
		PredictedAt:            now,
		WeibullShape:           lifeResult.WeibullShape,
		WeibullScale:           lifeResult.WeibullScale,
		RunningHours:           runningHours,
		PredictedRULHours:      lifeResult.PredictedRULHours,
		Reliability:            &lifeResult.Reliability,
		FailureProbability:     &lifeResult.FailureProbability,
		ConfidenceIntervalLow:  &lifeResult.ConfidenceIntervalLow,
		ConfidenceIntervalHigh: &lifeResult.ConfidenceIntervalHigh,
		WearRateTrend:          &lifeResult.WearRateTrend,
		FatigueDamage:          &lifeResult.FatigueDamage,
		PredictionMethod:       "weibull_mixed",
	}
	_ = database.Instance.InsertLifePrediction(ctx, lifeDB)

	c.JSON(http.StatusOK, gin.H{
		"wear": wearResult,
		"life": lifeResult,
	})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	dbOk := true
	if err := database.Instance.Ping(c.Request.Context()); err != nil {
		dbOk = false
	}
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"service":   "noria-bearing-system",
		"version":   Version,
		"database": map[string]interface{}{
			"connected": dbOk,
		},
	})
}

func (h *Handler) DebugWeibull(c *gin.Context) {
	rawData := c.Query("data")
	if rawData == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "需要提供data参数(JSON数组)"})
		return
	}

	var data []float64
	if err := json.Unmarshal([]byte(rawData), &data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "数据格式错误"})
		return
	}

	bearings, _ := database.Instance.GetAllBearings(context.Background())
	var bearing models.Bearing
	if len(bearings) > 0 {
		bearing = bearings[0]
	}

	lifeReq := &messages.LifePredRequest{
		Bearing:      bearing,
		CurrentWear:  50,
		RunningHours: 1000,
		RequestID:    "debug",
	}

	for _, v := range data {
		wr := models.WearResult{}
		wr.WearRateMicromPerHour = &v
		lifeReq.WearHistory = append(lifeReq.WearHistory, wr)
	}

	tempLifeChan := make(chan messages.LifePredResult, 1)
	syncLifePred := life_predictor.NewLifePredictor(make(chan messages.LifePredRequest, 1), tempLifeChan)
	result := syncLifePred.Predict(lifeReq)
	c.JSON(http.StatusOK, result)
}
