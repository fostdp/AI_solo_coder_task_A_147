package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/messages"
)

type Scheduler struct {
	channels    *messages.ModuleChannels
	wearTicker  *time.Ticker
	predTicker  *time.Ticker
	alertTicker *time.Ticker
	stopCh      chan struct{}
	running     bool
}

func NewScheduler(channels *messages.ModuleChannels) *Scheduler {
	return &Scheduler{
		channels: channels,
		stopCh:   make(chan struct{}),
		running:  false,
	}
}

func (s *Scheduler) Start() {
	s.running = true
	s.wearTicker = time.NewTicker(time.Duration(config.AppConfig.WearCalc.IntervalMinutes) * time.Minute)
	s.predTicker = time.NewTicker(time.Duration(config.AppConfig.LifePred.IntervalMinutes) * time.Minute)
	s.alertTicker = time.NewTicker(5 * time.Minute)

	log.Println("调度服务已启动")

	go s.wearLoop()
	go s.predictionLoop()
	go s.alertLoop()
	go s.resultListenerLoop()

	go func() {
		time.Sleep(5 * time.Second)
		s.runWearCalculation()
		time.Sleep(2 * time.Second)
		s.runLifePrediction()
		time.Sleep(2 * time.Second)
		s.runAlertCheck()
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.running = false
	if s.wearTicker != nil {
		s.wearTicker.Stop()
	}
	if s.predTicker != nil {
		s.predTicker.Stop()
	}
	if s.alertTicker != nil {
		s.alertTicker.Stop()
	}
	log.Println("调度服务已停止")
}

func (s *Scheduler) wearLoop() {
	for {
		select {
		case <-s.wearTicker.C:
			s.runWearCalculation()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) predictionLoop() {
	for {
		select {
		case <-s.predTicker.C:
			s.runLifePrediction()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) alertLoop() {
	for {
		select {
		case <-s.alertTicker.C:
			s.runAlertCheck()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) resultListenerLoop() {
	for {
		select {
		case result, ok := <-s.channels.WearResultChan:
			if !ok {
				return
			}
			s.handleWearResult(&result)
		case result, ok := <-s.channels.LifeResultChan:
			if !ok {
				return
			}
			s.handleLifeResult(&result)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) runWearCalculation() {
	ctx := context.Background()
	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
		return
	}

	for _, bearing := range bearings {
		s.sendWearRequest(ctx, bearing)
	}
}

func (s *Scheduler) sendWearRequest(ctx context.Context, bearing models.Bearing) {
	now := time.Now()
	periodStart := now.Add(-time.Duration(config.AppConfig.WearCalc.IntervalMinutes) * time.Minute)

	sensorData, err := database.Instance.GetSensorDataByTimeRange(ctx, bearing.ID, periodStart, now)
	if err != nil {
		log.Printf("获取轴承 %d 传感器数据失败: %v", bearing.ID, err)
		return
	}

	previousTotal := bearing.InitialWearMicrom
	lastWear, err := database.Instance.GetLatestWearResult(ctx, bearing.ID)
	if err == nil && lastWear != nil {
		previousTotal = lastWear.TotalWearMicrom
	}

	reqID := uuid.New().String()
	request := messages.WearCalcRequest{
		Bearing:       bearing,
		SensorData:    sensorData,
		PreviousTotal: previousTotal,
		PeriodStart:   periodStart,
		PeriodEnd:     now,
		RequestID:     reqID,
	}

	select {
	case s.channels.WearRequestChan <- request:
		log.Printf("已发送磨损计算请求 (轴承 %s, 请求ID: %s)", bearing.BearingCode, reqID)
	default:
		log.Printf("磨损计算请求通道已满，丢弃请求 (轴承 %s)", bearing.BearingCode)
	}
}

func (s *Scheduler) handleWearResult(result *messages.WearCalcResult) {
	if !result.Success {
		log.Printf("磨损计算失败 (轴承 %d, 请求ID: %s): %s", result.BearingID, result.RequestID, result.Error)
		return
	}

	log.Printf("磨损计算完成 (轴承 %d, 请求ID: %s): 阶段磨损=%.4fμm, 累计磨损=%.4fμm, EHL=%.3f",
		result.BearingID, result.RequestID, result.WearDepthMicrom, result.TotalWearMicrom, result.EHLFilmParameter)
}

func (s *Scheduler) runLifePrediction() {
	ctx := context.Background()
	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
		return
	}

	for _, bearing := range bearings {
		s.sendLifeRequest(ctx, bearing)
	}
}

func (s *Scheduler) sendLifeRequest(ctx context.Context, bearing models.Bearing) {
	wearHistory, err := database.Instance.GetWearHistory(ctx, bearing.ID, 100)
	if err != nil {
		log.Printf("获取轴承 %d 磨损历史失败: %v", bearing.ID, err)
		return
	}

	currentWear := bearing.InitialWearMicrom
	if len(wearHistory) > 0 {
		currentWear = wearHistory[0].TotalWearMicrom
	}

	runningHours := time.Since(bearing.InstalledAt).Hours()

	reqID := uuid.New().String()
	request := messages.LifePredRequest{
		Bearing:      bearing,
		WearHistory:  wearHistory,
		CurrentWear:  currentWear,
		RunningHours: runningHours,
		RequestID:    reqID,
	}

	select {
	case s.channels.LifeRequestChan <- request:
		log.Printf("已发送寿命预测请求 (轴承 %s, 请求ID: %s)", bearing.BearingCode, reqID)
	default:
		log.Printf("寿命预测请求通道已满，丢弃请求 (轴承 %s)", bearing.BearingCode)
	}
}

func (s *Scheduler) handleLifeResult(result *messages.LifePredResult) {
	if !result.Success {
		log.Printf("寿命预测失败 (轴承 %d, 请求ID: %s): %s", result.BearingID, result.RequestID, result.Error)
		return
	}

	log.Printf("寿命预测完成 (轴承 %d, 请求ID: %s): RUL=%.2f小时, 可靠度=%.4f, β=%.3f",
		result.BearingID, result.RequestID, result.PredictedRULHours, result.Reliability, result.WeibullShape)
}

func (s *Scheduler) runAlertCheck() {
	ctx := context.Background()
	statuses, err := database.Instance.GetBearingLatestStatus(ctx)
	if err != nil {
		log.Printf("获取轴承状态失败: %v", err)
		return
	}

	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
		return
	}

	bearingMap := make(map[int]models.Bearing)
	for _, b := range bearings {
		bearingMap[b.ID] = b
	}

	for _, status := range statuses {
		bearing, ok := bearingMap[status.BearingID]
		if !ok {
			continue
		}

		s.checkWearAlert(bearing, status)
		s.checkOilFilmAlert(bearing, status)
	}
}

func (s *Scheduler) checkWearAlert(bearing models.Bearing, status models.BearingLatestStatus) {
	if status.TotalWearMicrom == nil {
		return
	}

	totalWear := *status.TotalWearMicrom
	warnThreshold := bearing.WearLimitMicrom * config.AppConfig.Alert.WearWarningRatio
	critThreshold := bearing.WearLimitMicrom * config.AppConfig.Alert.WearCriticalRatio

	var alertType, alertLevel, message string
	var threshold float64

	if totalWear >= critThreshold {
		alertType = "wear_exceeded"
		alertLevel = "critical"
		threshold = critThreshold
		message = fmt.Sprintf("轴承 %s 磨损深度严重超限！累计磨损%.4fμm，已达阈值%.4fμm的%.1f%%",
			bearing.BearingCode, totalWear, bearing.WearLimitMicrom, totalWear/bearing.WearLimitMicrom*100)
	} else if totalWear >= warnThreshold {
		alertType = "wear_warning"
		alertLevel = "warning"
		threshold = warnThreshold
		message = fmt.Sprintf("轴承 %s 磨损深度接近阈值。累计磨损%.4fμm，阈值%.4fμm",
			bearing.BearingCode, totalWear, bearing.WearLimitMicrom)
	} else {
		return
	}

	s.sendAlert(&bearing, alertType, alertLevel, message, &threshold, &totalWear)
}

func (s *Scheduler) checkOilFilmAlert(bearing models.Bearing, status models.BearingLatestStatus) {
	if status.OilFilmThickness == nil {
		return
	}

	filmThickness := *status.OilFilmThickness
	minFilm := config.AppConfig.Alert.OilFilmMinimum

	if filmThickness >= minFilm {
		return
	}

	alertType := "oil_film_rupture"
	alertLevel := "critical"
	message := fmt.Sprintf("轴承 %s 润滑油膜破裂！油膜厚度%.4fμm低于安全阈值%.4fμm，存在干摩擦风险",
		bearing.BearingCode, filmThickness, minFilm)

	s.sendAlert(&bearing, alertType, alertLevel, message, &minFilm, &filmThickness)
}

func (s *Scheduler) sendAlert(
	bearing *models.Bearing,
	alertType, alertLevel, message string,
	threshold, actualValue *float64,
) {
	alert := messages.AlertMessage{
		Bearing:      bearing,
		AlertType:    alertType,
		AlertLevel:   alertLevel,
		AlertMessage: message,
		Threshold:    threshold,
		ActualValue:  actualValue,
		Timestamp:    time.Now(),
	}

	select {
	case s.channels.AlertChan <- alert:
		log.Printf("告警已发送到通道: [%s] %s - %s", alertLevel, bearing.BearingCode, message)
	default:
		log.Printf("告警通道已满，丢弃告警 (轴承 %d)", bearing.ID)
	}
}
