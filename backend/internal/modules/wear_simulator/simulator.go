package wear_simulator

import (
	"context"
	"log"
	"math"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/database"
	"noria-bearing-system/internal/models"
	"noria-bearing-system/internal/modules/messages"
)

type WearSimulator struct {
	requestChan  <-chan messages.WearCalcRequest
	resultChan   chan<- messages.WearCalcResult
	wearParams   *config.WearParamsConfig
	lubrication  *config.LubricationConfig
	running      bool
}

func NewWearSimulator(
	requestChan <-chan messages.WearCalcRequest,
	resultChan chan<- messages.WearCalcResult,
) *WearSimulator {
	return &WearSimulator{
		requestChan: requestChan,
		resultChan:  resultChan,
		wearParams:  &config.AppConfig.WearParams,
		lubrication: &config.AppConfig.Lubrication,
		running:     false,
	}
}

func (ws *WearSimulator) Start(ctx context.Context) {
	ws.running = true
	log.Println("磨损仿真器已启动")

	go func() {
		for {
			select {
			case <-ctx.Done():
				ws.Stop()
				return
			case req, ok := <-ws.requestChan:
				if !ok {
					return
				}
				result := ws.Calculate(&req)
				ws.sendResult(result)
				ws.saveToDatabase(&req, result)
			}
		}
	}()
}

func (ws *WearSimulator) Stop() {
	ws.running = false
	close(ws.resultChan)
	log.Println("磨损仿真器已停止")
}

func (ws *WearSimulator) Calculate(input *messages.WearCalcRequest) *messages.WearCalcResult {
	result := &messages.WearCalcResult{
		BearingID:   input.Bearing.ID,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		RequestID:   input.RequestID,
		Success:     false,
	}

	if len(input.SensorData) == 0 {
		result.TotalWearMicrom = input.PreviousTotal
		result.Success = true
		result.Error = "无传感器数据"
		return result
	}

	avgTemp := averageFloat(getFieldValues(input.SensorData, "temperature"))
	avgLoad := averageFloat(getFieldValues(input.SensorData, "radial_load"))
	avgSpeed := averageFloat(getFieldValues(input.SensorData, "rotational_speed"))
	avgFilmThickness := averageFloat(getFieldValues(input.SensorData, "oil_film_thickness"))

	b := input.Bearing
	innerRadius := b.InnerDiameter / 2.0 / 1000.0
	outerRadius := b.OuterDiameter / 2.0 / 1000.0
	effectiveRadius := (innerRadius + outerRadius) / 2.0
	widthMeters := b.Width / 1000.0

	contactArea := math.Pi * (outerRadius*outerRadius - innerRadius*innerRadius)
	if contactArea <= 0 {
		contactArea = effectiveRadius * 2 * math.Pi * widthMeters
	}
	contactPressure := avgLoad / contactArea

	rpmToRadPerSec := 2.0 * math.Pi / 60.0
	angularVelocity := avgSpeed * rpmToRadPerSec
	slidingVelocity := effectiveRadius * angularVelocity

	periodHours := input.PeriodEnd.Sub(input.PeriodStart).Hours()
	periodSeconds := periodHours * 3600.0
	slidingDistance := slidingVelocity * periodSeconds

	hardnessPa := b.HardnessHV * ws.wearParams.HardnessConversionFactor

	ehlFilmParam := ws.calculateEHLFilmParameter(
		avgFilmThickness,
		effectiveRadius,
		avgSpeed,
		b.OilViscosityPaS,
		avgLoad,
		avgTemp,
	)

	wearCoefficient := ws.calculateWearCoefficient(ehlFilmParam)

	tempFactor := 1.0
	if avgTemp > ws.wearParams.EHLReferenceTempCelsius {
		tempFactor = 1.0 + ws.wearParams.TempCorrectionFactorPerDegree*(avgTemp-ws.wearParams.EHLReferenceTempCelsius)
	}

	archardWearVolume := wearCoefficient * avgLoad * slidingDistance / hardnessPa * tempFactor
	wearDepthMeters := archardWearVolume / contactArea
	wearDepthMicrom := wearDepthMeters * 1e6

	totalWear := input.PreviousTotal + wearDepthMicrom
	var wearRate float64
	if periodHours > 0 {
		wearRate = wearDepthMicrom / periodHours
	}

	result.WearDepthMicrom = wearDepthMicrom
	result.TotalWearMicrom = totalWear
	result.WearRateMicromPerHour = wearRate
	result.ArchardWearVolume = archardWearVolume
	result.EHLFilmParameter = ehlFilmParam
	result.SlidingDistance = slidingDistance
	result.WearCoefficient = wearCoefficient
	result.ContactPressure = contactPressure
	result.CalculatedAt = time.Now()
	result.Success = true

	return result
}

func (ws *WearSimulator) calculateEHLFilmParameter(
	filmThickness, effectiveRadius, speedRPM, viscosity, load, temperature float64,
) float64 {
	if filmThickness <= 0 || effectiveRadius <= 0 || viscosity <= 0 {
		return ws.lubrication.EHLCorrection.LambdaMinClamp
	}

	alphaPV := ws.wearParams.PressureViscosityCoefficient
	reducedModulus := ws.wearParams.ReducedElasticModulusPa
	roughnessRMS := ws.wearParams.SurfaceRoughnessRMSMeters

	u := effectiveRadius * speedRPM * 2.0 * math.Pi / 60.0
	if u < 1e-6 {
		u = 1e-6
	}

	R := effectiveRadius
	G := alphaPV * reducedModulus
	U := (viscosity * u) / (reducedModulus * R)
	W := load / (reducedModulus * R)

	hMin := ws.lubrication.EHLCorrection.DowsonHigginsonCoefficient * R *
		math.Pow(U, 0.68) * math.Pow(G, 0.49) * math.Pow(W, -0.073)

	if U > ws.lubrication.EHLCorrection.AlternativeFormulaThresholdU {
		hMinAlt := ws.lubrication.EHLCorrection.AlternativeFormulaCoefficient * R *
			math.Pow(G, 0.54) * math.Pow(U, 0.7) * math.Pow(W, -0.13)
		if hMinAlt < hMin {
			hMin = hMinAlt
		}
	}

	if temperature > ws.wearParams.EHLReferenceTempCelsius {
		tempViscRatio := math.Pow(ws.wearParams.EHLReferenceTempCelsius/temperature,
			ws.lubrication.EHLCorrection.TempViscosityExponent)
		hMin *= tempViscRatio
	}

	lambda := hMin / roughnessRMS

	if lambda < ws.wearParams.FullFilmLambdaThreshold {
		lambda = ws.applyMixedLubricationCorrection(lambda, load, u, viscosity, effectiveRadius)
	}

	if math.IsNaN(lambda) || lambda < 0 {
		lambda = ws.lubrication.EHLCorrection.LambdaMinClamp
	}
	if lambda > ws.lubrication.EHLCorrection.LambdaMaxClamp {
		lambda = ws.lubrication.EHLCorrection.LambdaMaxClamp
	}

	return lambda
}

func (ws *WearSimulator) applyMixedLubricationCorrection(
	lambda, load, velocity, viscosity, radius float64,
) float64 {
	sommerfeldNumber := viscosity * velocity * radius * radius / load

	var asperityContactRatio float64
	if lambda >= ws.wearParams.FullFilmLambdaThreshold {
		asperityContactRatio = 0.0
	} else if lambda >= ws.wearParams.MixedLubricationLambdaThreshold {
		asperityContactRatio = 1.0 - lambda/ws.wearParams.FullFilmLambdaThreshold
		asperityContactRatio = math.Pow(asperityContactRatio,
			ws.lubrication.MixedLubrication.AsperityContactExponent)
	} else {
		asperityContactRatio = 1.0 - (lambda/ws.wearParams.MixedLubricationLambdaThreshold)/3.0
		if asperityContactRatio > ws.lubrication.MixedLubrication.BoundaryMaxContactRatio {
			asperityContactRatio = ws.lubrication.MixedLubrication.BoundaryMaxContactRatio
		}
	}

	if sommerfeldNumber < ws.lubrication.MixedLubrication.SommerfeldThresholdLow {
		lambda *= ws.lubrication.MixedLubrication.CorrectionFactorLowSommerfeld
	} else if sommerfeldNumber < ws.lubrication.MixedLubrication.SommerfeldThresholdHigh {
		logFactor := math.Log10(sommerfeldNumber / ws.lubrication.MixedLubrication.SommerfeldThresholdLow)
		rangeWidth := math.Log10(ws.lubrication.MixedLubrication.SommerfeldThresholdHigh /
			ws.lubrication.MixedLubrication.SommerfeldThresholdLow)
		lambda *= (ws.lubrication.MixedLubrication.CorrectionFactorLowSommerfeld +
			(1.0-ws.lubrication.MixedLubrication.CorrectionFactorLowSommerfeld)*logFactor/rangeWidth)
	}

	loadSeverity := math.Pow(load/ws.lubrication.MixedLubrication.LoadSeverityReferenceN,
		ws.lubrication.MixedLubrication.LoadSeverityExponent)
	if loadSeverity > 1.0 {
		lambda /= (1.0 + 0.15*(loadSeverity-1.0))
	}

	effectiveLambda := lambda * (1.0 - ws.lubrication.MixedLubrication.AsperityEffectFactor*asperityContactRatio)
	if effectiveLambda < 0.05 {
		effectiveLambda = 0.05
	}

	return effectiveLambda
}

func (ws *WearSimulator) calculateWearCoefficient(ehlFilmParam float64) float64 {
	baseK := ws.wearParams.ArchardKBase
	factors := ws.wearParams.WearCoefficientFactors

	if ehlFilmParam >= ws.wearParams.FullFilmLambdaThreshold {
		return baseK * factors["full_film"]
	} else if ehlFilmParam >= ws.wearParams.MixedLubricationLambdaThreshold {
		acRatio := math.Pow(1.0-ehlFilmParam/ws.wearParams.FullFilmLambdaThreshold, 2)
		return baseK * (factors["mixed_min"] + (factors["mixed_max"]-factors["mixed_min"])*acRatio)
	} else {
		acRatio := 1.0 - 0.33*ehlFilmParam
		if acRatio > ws.lubrication.MixedLubrication.BoundaryMaxContactRatio {
			acRatio = ws.lubrication.MixedLubrication.BoundaryMaxContactRatio
		}
		return baseK * (factors["boundary_min"] + (factors["boundary_max"]-factors["boundary_min"])*acRatio)
	}
}

type FilmThicknessGrid struct {
	GridSizeX int
	GridSizeY int
	Data      [][]float64
}

func GenerateOilFilmMap(
	bearing models.Bearing,
	avgLoad, avgSpeed, avgTemp, avgFilmThickness float64,
) *FilmThicknessGrid {
	params := &config.AppConfig.WearParams
	lub := &config.AppConfig.Lubrication

	gridSizeX := params.OilFilmGrid.SizeX
	gridSizeY := params.OilFilmGrid.SizeY
	data := make([][]float64, gridSizeY)

	innerR := bearing.InnerDiameter / 2.0
	outerR := bearing.OuterDiameter / 2.0

	for i := 0; i < gridSizeY; i++ {
		data[i] = make([]float64, gridSizeX)
		for j := 0; j < gridSizeX; j++ {
			theta := 2.0 * math.Pi * float64(j) / float64(gridSizeX)
			radiusRatio := float64(i) / float64(gridSizeY-1)
			radius := innerR + (outerR-innerR)*radiusRatio

			loadAngleEffect := math.Cos(theta)
			if loadAngleEffect < 0 {
				loadAngleEffect = 0
			}

			speedFactor := 1.0 + 0.15*math.Sin(theta+math.Pi/4)
			radialFactor := 1.0 + 0.1*(radiusRatio-0.5)
			baseThickness := avgFilmThickness * speedFactor * radialFactor

			pressureReduction := 0.3 * loadAngleEffect * (avgLoad / lub.MixedLubrication.LoadSeverityReferenceN)
			if pressureReduction > 0.5 {
				pressureReduction = 0.5
			}

			tempReduction := 0.0
			if avgTemp > params.EHLReferenceTempCelsius {
				tempReduction = params.TempCorrectionFactorPerDegree * (avgTemp - params.EHLReferenceTempCelsius)
			}

			film := baseThickness * (1.0 - pressureReduction - tempReduction)

			effectiveRadiusM := (innerR + (outerR-innerR)*radiusRatio) / 1000.0
			u := effectiveRadiusM * avgSpeed * 2.0 * math.Pi / 60.0
			sommerfeld := bearing.OilViscosityPaS * u * effectiveRadiusM * effectiveRadiusM / avgLoad

			if sommerfeld < lub.MixedLubrication.SommerfeldThresholdLow {
				logFactor := math.Log10(sommerfeld / 1e-6)
				mixedCorrection := 0.5 + 0.5*logFactor/2.0
				if mixedCorrection < 0.3 {
					mixedCorrection = 0.3
				}
				film *= mixedCorrection
			}

			loadSeverity := math.Pow(avgLoad/lub.MixedLubrication.LoadSeverityReferenceN,
				lub.MixedLubrication.LoadSeverityExponent)
			if loadSeverity > 1.0 {
				film /= (1.0 + 0.1*(loadSeverity-1.0))
			}

			noise := (math.Sin(float64(i*7+j*11)) * 0.05)
			film = film * (1.0 + noise)

			if film < 0.01 {
				film = 0.01
			}
			if film > avgFilmThickness*2.0 {
				film = avgFilmThickness * 2.0
			}

			data[i][j] = film
		}
	}

	return &FilmThicknessGrid{
		GridSizeX: gridSizeX,
		GridSizeY: gridSizeY,
		Data:      data,
	}
}

func (ws *WearSimulator) sendResult(result *messages.WearCalcResult) {
	select {
	case ws.resultChan <- *result:
	default:
		log.Printf("磨损结果通道已满，丢弃结果 (轴承ID=%d)", result.BearingID)
	}
}

func (ws *WearSimulator) saveToDatabase(req *messages.WearCalcRequest, result *messages.WearCalcResult) {
	if !result.Success || database.Instance == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	note := "仿真器计算"
	wearResult := &models.WearResult{
		BearingID:             result.BearingID,
		CalculatedAt:          result.CalculatedAt,
		PeriodStart:           result.PeriodStart,
		PeriodEnd:             result.PeriodEnd,
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
		log.Printf("保存磨损结果失败 (轴承 %d): %v", result.BearingID, err)
	} else {
		log.Printf("轴承 %s 磨损计算完成: 阶段磨损=%.4fμm, 累计磨损=%.4fμm, 磨损率=%.6fμm/h, EHL参数=%.3f",
			req.Bearing.BearingCode, result.WearDepthMicrom, result.TotalWearMicrom,
			result.WearRateMicromPerHour, result.EHLFilmParameter)
	}
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func getFieldValues(data []models.SensorData, field string) []float64 {
	var values []float64
	for _, d := range data {
		switch field {
		case "temperature":
			values = append(values, d.Temperature)
		case "radial_load":
			values = append(values, d.RadialLoad)
		case "rotational_speed":
			values = append(values, d.RotationalSpeed)
		case "oil_film_thickness":
			values = append(values, d.OilFilmThickness)
		}
	}
	return values
}
