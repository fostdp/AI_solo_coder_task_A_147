package simulation

import (
	"math"

	"github.com/montanaflynn/stats"
	"noria-bearing-system/internal/config"
	"noria-bearing-system/internal/models"
)

type LifePredictor struct{}

func NewLifePredictor() *LifePredictor {
	return &LifePredictor{}
}

type LifePredInput struct {
	Bearing         models.Bearing
	WearHistory     []models.WearResult
	CurrentWear     float64
	RunningHours    float64
}

type LifePredOutput struct {
	WeibullShape           float64
	WeibullScale           float64
	PredictedRULHours      float64
	Reliability            float64
	FailureProbability     float64
	ConfidenceIntervalLow  float64
	ConfidenceIntervalHigh float64
	WearRateTrend          float64
	FatigueDamage          float64
}

func (lp *LifePredictor) Predict(input *LifePredInput) *LifePredOutput {
	var wearRates []float64
	for _, wr := range input.WearHistory {
		if wr.WearRateMicromPerHour != nil {
			wearRates = append(wearRates, *wr.WearRateMicromPerHour)
		}
	}

	var wearRateTrend float64
	if len(wearRates) >= 2 {
		slope, _ := linearRegressionSlope(wearRates)
		wearRateTrend = slope
	} else if len(wearRates) >= 1 {
		wearRateTrend, _ = stats.Mean(wearRates)
	} else {
		wearRateTrend = input.Bearing.WearLimitMicrom / input.Bearing.RatedLifeHours * 0.5
	}

	if wearRateTrend <= 0 {
		wearRateTrend = input.Bearing.WearLimitMicrom / input.Bearing.RatedLifeHours * 0.3
	}

	weibullShape, weibullScale := lp.estimateWeibullParameters(
		input.Bearing,
		wearRates,
		input.CurrentWear,
		input.RunningHours,
	)

	wearBasedRUL := (input.Bearing.WearLimitMicrom - input.CurrentWear) / wearRateTrend
	if wearBasedRUL < 0 {
		wearBasedRUL = 0
	}

	fatigueDamage := lp.calculateFatigueDamage(
		input.RunningHours,
		weibullShape,
		weibullScale,
	)

	weibullBasedRUL := lp.calculateWeibullRUL(
		input.RunningHours,
		weibullShape,
		weibullScale,
	)

	weightWear := 0.6
	weightWeibull := 0.4
	predictedRUL := weightWear*wearBasedRUL + weightWeibull*weibullBasedRUL

	if predictedRUL < 0 {
		predictedRUL = 0
	}
	if predictedRUL > input.Bearing.RatedLifeHours*2 {
		predictedRUL = input.Bearing.RatedLifeHours * 2
	}

	reliability := math.Exp(-math.Pow(input.RunningHours/weibullScale, weibullShape))
	if reliability < 0 {
		reliability = 0
	}
	if reliability > 1 {
		reliability = 1
	}
	failureProbability := 1.0 - reliability

	confidenceLow := predictedRUL * 0.7
	confidenceHigh := predictedRUL * 1.3

	return &LifePredOutput{
		WeibullShape:           weibullShape,
		WeibullScale:           weibullScale,
		PredictedRULHours:      predictedRUL,
		Reliability:            reliability,
		FailureProbability:     failureProbability,
		ConfidenceIntervalLow:  confidenceLow,
		ConfidenceIntervalHigh: confidenceHigh,
		WearRateTrend:          wearRateTrend,
		FatigueDamage:          fatigueDamage,
	}
}

func (lp *LifePredictor) estimateWeibullParameters(
	bearing models.Bearing,
	wearRates []float64,
	currentWear float64,
	runningHours float64,
) (float64, float64) {
	defaultShape := config.AppConfig.LifePred.WeibullDefaultShape
	defaultScale := config.AppConfig.LifePred.WeibullDefaultScale

	if len(wearRates) >= config.AppConfig.LifePred.MinSamplesForFit {
		shape, scale, ok := fitWeibull(wearRates)
		if ok {
			if shape < 1.0 {
				shape = 1.0
			}
			if shape > 5.0 {
				shape = 5.0
			}
			if scale < 1000 {
				scale = bearing.RatedLifeHours * 0.5
			}
			return shape, scale
		}
	}

	shape := defaultShape
	scale := defaultScale

	if runningHours > 0 && currentWear > 0 {
		projectedLifeHours := runningHours * bearing.WearLimitMicrom / currentWear
		scale = projectedLifeHours / math.Pow(math.Log(2.0), 1.0/shape)
	}

	return shape, scale
}

func fitWeibull(data []float64) (float64, float64, bool) {
	if len(data) < 5 {
		return 0, 0, false
	}

	for _, v := range data {
		if v <= 0 {
			return 0, 0, false
		}
	}

	sorted := make([]float64, len(data))
	copy(sorted, data)
	_ = stats.Sort(sorted)
	n := len(sorted)

	bestShape := 2.0
	bestScale := 0.0
	bestLL := math.Inf(-1)

	for shape := 0.5; shape <= 5.0; shape += 0.1 {
		var sumPow float64
		for _, x := range sorted {
			sumPow += math.Pow(x, shape)
		}
		scale := math.Pow(sumPow/float64(n), 1.0/shape)

		ll := 0.0
		for _, x := range sorted {
			if scale > 0 && x > 0 {
				ll += math.Log(shape) - shape*math.Log(scale) +
					(shape-1)*math.Log(x) - math.Pow(x/scale, shape)
			}
		}

		if ll > bestLL {
			bestLL = ll
			bestShape = shape
			bestScale = scale
		}
	}

	if bestScale <= 0 || math.IsNaN(bestScale) || math.IsNaN(bestShape) {
		return 0, 0, false
	}

	return bestShape, bestScale, true
}

func (lp *LifePredictor) calculateFatigueDamage(
	runningHours, shape, scale float64,
) float64 {
	if scale <= 0 || runningHours <= 0 {
		return 0
	}
	return 1.0 - math.Exp(-math.Pow(runningHours/scale, shape))
}

func (lp *LifePredictor) calculateWeibullRUL(
	runningHours, shape, scale float64,
) float64 {
	if scale <= 0 || runningHours <= 0 {
		return scale
	}

	targetReliability := 0.1
	t := scale * math.Pow(-math.Log(targetReliability), 1.0/shape)

	rul := t - runningHours
	if rul < 0 {
		rul = 0
	}
	return rul
}

func linearRegressionSlope(y []float64) (float64, float64) {
	n := len(y)
	if n < 2 {
		if n == 1 {
			return y[0], 0
		}
		return 0, 0
	}

	x := make([]float64, n)
	for i := range x {
		x[i] = float64(i)
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / float64(n)

	return slope, intercept
}
