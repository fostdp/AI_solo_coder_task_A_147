package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/mqtt"
	"noria-bearing-system/internal/simulation"
)

type Scheduler struct {
	wearCalc     *simulation.WearCalculator
	lifePred     *simulation.LifePredictor
	alertMgr     *mqtt.AlertManager
	wearTicker   *time.Ticker
	predTicker   *time.Ticker
	alertTicker  *time.Ticker
	stopCh       chan struct{}
}

func NewScheduler(alertMgr *mqtt.AlertManager) *Scheduler {
	return &Scheduler{
		wearCalc: simulation.NewWearCalculator(),
		lifePred: simulation.NewLifePredictor(),
		alertMgr: alertMgr,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.wearTicker = time.NewTicker(time.Duration(config.AppConfig.WearCalc.IntervalMinutes) * time.Minute)
	s.predTicker = time.NewTicker(time.Duration(config.AppConfig.LifePred.IntervalMinutes) * time.Minute)
	s.alertTicker = time.NewTicker(5 * time.Minute)

	log.Println("调度服务已启动")

	go s.wearLoop()
	go s.predictionLoop()
	go s.alertLoop()

	go func() {
		time.Sleep(5 * time.Second)
		s.runWearCalculation()
		s.runLifePrediction()
		s.runAlertCheck()
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
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

func (s *Scheduler) runWearCalculation() {
	ctx := context.Background()
	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
		return
	}

	for _, bearing := range bearings {
		s.calculateBearingWear(ctx, bearing)
	}
}

func (s *Scheduler) calculateBearingWear(ctx context.Context, bearing models.Bearing) {
	now := time.Now()
	periodStart := now.Add(-time.Duration(config.AppConfig.WearCalc.IntervalMinutes) * time.Minute)

	sensorData, err := database.Instance.GetSensorDataByTimeRange(ctx, bearing.ID, periodStart, now)
	if err != nil {
		log.Printf("获取轴承 %d 传感器数据失败: %v", bearing.ID, err)
		return
	}

	if len(sensorData) == 0 {
		log.Printf("轴承 %d 无传感器数据，跳过磨损计算", bearing.ID)
		return
	}

	previousTotal := bearing.InitialWearMicrom
	lastWear, err := database.Instance.GetLatestWearResult(ctx, bearing.ID)
	if err == nil && lastWear != nil {
		previousTotal = lastWear.TotalWearMicrom
	}

	input := &simulation.WearCalcInput{
		Bearing:       bearing,
		SensorData:    sensorData,
		PreviousTotal: previousTotal,
		PeriodStart:   periodStart,
		PeriodEnd:     now,
	}

	result := s.wearCalc.Calculate(input)

	note := "自动计算"
	wearResult := &models.WearResult{
		BearingID:             bearing.ID,
		CalculatedAt:          now,
		PeriodStart:           periodStart,
		PeriodEnd:             now,
		WearDepthMicrom:       result.WearDepthMicrom,
		WearRateMicromPerHour: &result.WearRateMicromPerHour,
		TotalWearMicrom:       result.TotalWearMicrom,
		ArchardWearVolume:     &result.ArchardWearVolume,
		EHLFilmParameter:      &result.EHLFilmParameter,
		SlidingDistance:       &result.SlidingDistance,
		WearCoefficient:       &result.WearCoefficient,
		ContactPressure:       &result.ContactPressure,
		CalculationNote:       &note,
	}

	if err := database.Instance.InsertWearResult(ctx, wearResult); err != nil {
		log.Printf("保存磨损结果失败 (轴承 %d): %v", bearing.ID, err)
		return
	}

	log.Printf("轴承 %s 磨损计算完成: 阶段磨损=%.4fμm, 累计磨损=%.4fμm, 磨损率=%.6fμm/h, EHL参数=%.3f",
		bearing.BearingCode, result.WearDepthMicrom, result.TotalWearMicrom,
		result.WearRateMicromPerHour, result.EHLFilmParameter)
}

func (s *Scheduler) runLifePrediction() {
	ctx := context.Background()
	bearings, err := database.Instance.GetAllBearings(ctx)
	if err != nil {
		log.Printf("获取轴承列表失败: %v", err)
		return
	}

	for _, bearing := range bearings {
		s.predictBearingLife(ctx, bearing)
	}
}

func (s *Scheduler) predictBearingLife(ctx context.Context, bearing models.Bearing) {
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

	input := &simulation.LifePredInput{
		Bearing:      bearing,
		WearHistory:  wearHistory,
		CurrentWear:  currentWear,
		RunningHours: runningHours,
	}

	result := s.lifePred.Predict(input)

	prediction := &models.LifePrediction{
		BearingID:              bearing.ID,
		PredictedAt:            time.Now(),
		WeibullShape:           result.WeibullShape,
		WeibullScale:           result.WeibullScale,
		RunningHours:           runningHours,
		PredictedRULHours:      result.PredictedRULHours,
		Reliability:            &result.Reliability,
		FailureProbability:     &result.FailureProbability,
		ConfidenceIntervalLow:  &result.ConfidenceIntervalLow,
		ConfidenceIntervalHigh: &result.ConfidenceIntervalHigh,
		WearRateTrend:          &result.WearRateTrend,
		FatigueDamage:          &result.FatigueDamage,
		PredictionMethod:       "weibull_mixed",
	}

	if err := database.Instance.InsertLifePrediction(ctx, prediction); err != nil {
		log.Printf("保存寿命预测失败 (轴承 %d): %v", bearing.ID, err)
		return
	}

	log.Printf("轴承 %s 寿命预测完成: 预测RUL=%.2f小时, 可靠度=%.4f, Weibull形状=%.3f, 疲劳损伤=%.4f",
		bearing.BearingCode, result.PredictedRULHours, result.Reliability,
		result.WeibullShape, result.FatigueDamage)
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

		s.checkWearAlert(ctx, bearing, status)
		s.checkOilFilmAlert(ctx, bearing, status)
	}
}

func (s *Scheduler) checkWearAlert(ctx context.Context, bearing models.Bearing, status models.BearingLatestStatus) {
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

	s.sendAlert(ctx, bearing, alertType, alertLevel, message, &threshold, &totalWear)
}

func (s *Scheduler) checkOilFilmAlert(ctx context.Context, bearing models.Bearing, status models.BearingLatestStatus) {
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

	s.sendAlert(ctx, bearing, alertType, alertLevel, message, &minFilm, &filmThickness)
}

func (s *Scheduler) sendAlert(
	ctx context.Context,
	bearing models.Bearing,
	alertType, alertLevel, message string,
	threshold, actualValue *float64,
) {
	if !s.alertMgr.ShouldAlert(bearing.ID, alertType) {
		return
	}

	alert := &models.AlertEvent{
		BearingID:      bearing.ID,
		AlertTime:      time.Now(),
		AlertType:      alertType,
		AlertLevel:     alertLevel,
		AlertMessage:   message,
		ThresholdValue: threshold,
		ActualValue:    actualValue,
	}

	topic, err := s.alertMgr.PublishAlert(&bearing, alert)
	if err != nil {
		log.Printf("MQTT告警推送失败: %v", err)
	}
	if topic != "" {
		alert.MQTTTopic = &topic
	}

	if err := database.Instance.InsertAlertEvent(ctx, alert); err != nil {
		log.Printf("保存告警事件失败: %v", err)
		return
	}

	s.alertMgr.MarkAlerted(bearing.ID, alertType)
	log.Printf("告警已触发: [%s] %s - %s", alertLevel, bearing.BearingCode, message)
}
