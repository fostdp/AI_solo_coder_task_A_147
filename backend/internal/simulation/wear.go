package simulation

import (
	"math"
	"time"

	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

type WearCalculator struct{}

func NewWearCalculator() *WearCalculator {
	return &WearCalculator{}
}

type WearCalcInput struct {
	Bearing       models.Bearing
	SensorData    []models.SensorData
	PreviousTotal float64
	PeriodStart   time.Time
	PeriodEnd     time.Time
}

type WearCalcOutput struct {
	WearDepthMicrom       float64
	TotalWearMicrom       float64
	WearRateMicromPerHour float64
	ArchardWearVolume     float64
	EHLFilmParameter      float64
	SlidingDistance       float64
	WearCoefficient       float64
	ContactPressure       float64
}

func (wc *WearCalculator) Calculate(input *WearCalcInput) *WearCalcOutput {
	if len(input.SensorData) == 0 {
		return &WearCalcOutput{
			TotalWearMicrom: input.PreviousTotal,
		}
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

	hardnessPa := b.HardnessHV * 9.80665e6

	ehlFilmParam := calculateEHLFilmParameter(
		avgFilmThickness,
		effectiveRadius,
		avgSpeed,
		b.OilViscosityPaS,
		avgLoad,
		avgTemp,
	)

	var wearCoefficient float64
	baseK := config.AppConfig.WearCalc.ArchardK
	if ehlFilmParam >= 3.0 {
		wearCoefficient = baseK * 0.1
	} else if ehlFilmParam >= 1.0 {
		acRatio := math.Pow(1.0-ehlFilmParam/3.0, 2)
		wearCoefficient = baseK * (0.1 + 1.9*acRatio)
	} else {
		acRatio := 1.0 - 0.33*ehlFilmParam
		if acRatio > 0.95 {
			acRatio = 0.95
		}
		wearCoefficient = baseK * (2.0 + 3.0*acRatio)
	}

	tempFactor := 1.0
	if avgTemp > config.AppConfig.WearCalc.EHLReferenceTemp {
		tempFactor = 1.0 + 0.02*(avgTemp-config.AppConfig.WearCalc.EHLReferenceTemp)
	}

	archardWearVolume := wearCoefficient * avgLoad * slidingDistance / hardnessPa * tempFactor
	wearDepthMeters := archardWearVolume / contactArea
	wearDepthMicrom := wearDepthMeters * 1e6

	totalWear := input.PreviousTotal + wearDepthMicrom
	var wearRate float64
	if periodHours > 0 {
		wearRate = wearDepthMicrom / periodHours
	}

	return &WearCalcOutput{
		WearDepthMicrom:       wearDepthMicrom,
		TotalWearMicrom:       totalWear,
		WearRateMicromPerHour: wearRate,
		ArchardWearVolume:     archardWearVolume,
		EHLFilmParameter:      ehlFilmParam,
		SlidingDistance:       slidingDistance,
		WearCoefficient:       wearCoefficient,
		ContactPressure:       contactPressure,
	}
}

func calculateEHLFilmParameter(
	filmThickness, effectiveRadius, speedRPM, viscosity, load, temperature float64,
) float64 {
	if filmThickness <= 0 || effectiveRadius <= 0 || viscosity <= 0 {
		return 0.1
	}

	alphaPV := 2.2e-8
	reducedModulus := 2.0e11
	roughnessRMS := 0.4e-6

	u := effectiveRadius * speedRPM * 2.0 * math.Pi / 60.0
	if u < 1e-6 {
		u = 1e-6
	}

	R := effectiveRadius
	G := alphaPV * reducedModulus
	U := (viscosity * u) / (reducedModulus * R)
	W := load / (reducedModulus * R)

	hMin := 3.63 * R * math.Pow(U, 0.68) * math.Pow(G, 0.49) * math.Pow(W, -0.073)

	if U > 1e-12 {
		hMinAlt := 2.65 * R * math.Pow(G, 0.54) * math.Pow(U, 0.7) * math.Pow(W, -0.13)
		if hMinAlt < hMin {
			hMin = hMinAlt
		}
	}

	if temperature > 40.0 {
		tempViscRatio := math.Pow(40.0/temperature, 1.5)
		hMin *= tempViscRatio
	}

	lambda := hMin / roughnessRMS

	if lambda < 3.0 {
		lambda = applyMixedLubricationCorrection(lambda, load, u, viscosity, effectiveRadius)
	}

	if math.IsNaN(lambda) || lambda < 0 {
		lambda = 0.1
	}
	if lambda > 10 {
		lambda = 10
	}

	return lambda
}

func applyMixedLubricationCorrection(lambda, load, velocity, viscosity, radius float64) float64 {
	sommerfeldNumber := viscosity * velocity * radius * radius / load

	var asperityContactRatio float64
	if lambda >= 3.0 {
		asperityContactRatio = 0.0
	} else if lambda >= 1.0 {
		asperityContactRatio = 1.0 - lambda/3.0
		asperityContactRatio *= asperityContactRatio
	} else {
		asperityContactRatio = 1.0 - 0.33*lambda
		if asperityContactRatio > 0.95 {
			asperityContactRatio = 0.95
		}
	}

	if sommerfeldNumber < 1e-4 {
		lambda *= 0.6
	} else if sommerfeldNumber < 1e-2 {
		lambda *= (0.6 + 0.4*math.Log10(sommerfeldNumber/1e-4)/2.0)
	}

	loadSeverity := math.Sqrt(load / 10000.0)
	if loadSeverity > 1.0 {
		lambda /= (1.0 + 0.15*(loadSeverity-1.0))
	}

	effectiveLambda := lambda * (1.0 - 0.3*asperityContactRatio)
	if effectiveLambda < 0.05 {
		effectiveLambda = 0.05
	}

	return effectiveLambda
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

type FilmThicknessGrid struct {
	GridSizeX int
	GridSizeY int
	Data      [][]float64
}

func GenerateOilFilmMap(
	bearing models.Bearing,
	avgLoad, avgSpeed, avgTemp, avgFilmThickness float64,
) *FilmThicknessGrid {
	gridSizeX := 64
	gridSizeY := 32
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

			pressureReduction := 0.3 * loadAngleEffect * (avgLoad / 10000.0)
			if pressureReduction > 0.5 {
				pressureReduction = 0.5
			}

			tempReduction := 0.0
			if avgTemp > config.AppConfig.WearCalc.EHLReferenceTemp {
				tempReduction = 0.01 * (avgTemp - config.AppConfig.WearCalc.EHLReferenceTemp)
			}

			film := baseThickness * (1.0 - pressureReduction - tempReduction)

			effectiveRadiusM := (innerR + (outerR-innerR)*radiusRatio) / 1000.0
			u := effectiveRadiusM * avgSpeed * 2.0 * math.Pi / 60.0
			sommerfeld := bearing.OilViscosityPaS * u * effectiveRadiusM * effectiveRadiusM / avgLoad

			if sommerfeld < 1e-4 {
				mixedCorrection := 0.5 + 0.5*math.Log10(sommerfeld/1e-6)/2.0
				if mixedCorrection < 0.3 {
					mixedCorrection = 0.3
				}
				film *= mixedCorrection
			}

			loadSeverity := math.Sqrt(avgLoad / 10000.0)
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
