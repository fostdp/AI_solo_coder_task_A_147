package messages

import (
	"time"

	"noria-bearing-system/internal/models"
)

type SensorDataMessage struct {
	Data      *models.SensorData
	Timestamp time.Time
	Valid     bool
	Error     string
}

type WearCalcRequest struct {
	Bearing       models.Bearing
	SensorData    []models.SensorData
	PreviousTotal float64
	PeriodStart   time.Time
	PeriodEnd     time.Time
	RequestID     string
}

type WearCalcResult struct {
	BearingID             int
	WearDepthMicrom       float64
	TotalWearMicrom       float64
	WearRateMicromPerHour float64
	ArchardWearVolume     float64
	EHLFilmParameter      float64
	SlidingDistance       float64
	WearCoefficient       float64
	ContactPressure       float64
	CalculatedAt          time.Time
	PeriodStart           time.Time
	PeriodEnd             time.Time
	RequestID             string
	Success               bool
	Error                 string
}

type LifePredRequest struct {
	Bearing      models.Bearing
	WearHistory  []models.WearResult
	CurrentWear  float64
	RunningHours float64
	RequestID    string
}

type LifePredResult struct {
	BearingID              int
	WeibullShape           float64
	WeibullScale           float64
	PredictedRULHours      float64
	Reliability            float64
	FailureProbability     float64
	ConfidenceIntervalLow  float64
	ConfidenceIntervalHigh float64
	WearRateTrend          float64
	FatigueDamage          float64
	PredictedAt            time.Time
	RequestID              string
	Success                bool
	Error                  string
}

type AlertMessage struct {
	Bearing      *models.Bearing
	AlertType    string
	AlertLevel   string
	AlertMessage string
	Threshold    *float64
	ActualValue  *float64
	Timestamp    time.Time
}

type ModuleChannels struct {
	SensorDataChan   chan SensorDataMessage
	WearRequestChan  chan WearCalcRequest
	WearResultChan   chan WearCalcResult
	LifeRequestChan  chan LifePredRequest
	LifeResultChan   chan LifePredResult
	AlertChan        chan AlertMessage
}

func NewModuleChannels(bufferSize int) *ModuleChannels {
	return &ModuleChannels{
		SensorDataChan:   make(chan SensorDataMessage, bufferSize),
		WearRequestChan:  make(chan WearCalcRequest, bufferSize),
		WearResultChan:   make(chan WearCalcResult, bufferSize),
		LifeRequestChan:  make(chan LifePredRequest, bufferSize),
		LifeResultChan:   make(chan LifePredResult, bufferSize),
		AlertChan:        make(chan AlertMessage, bufferSize),
	}
}
