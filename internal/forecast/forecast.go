package forecast

import (
	"math"
	"sort"
	"time"
)

type CapacityPoint struct {
	Timestamp  time.Time
	UsedBytes  int64
	TotalBytes int64
}

type Forecast struct {
	Timestamp     time.Time
	PredictedUsed int64
	LowerBound    int64
	UpperBound    int64
	Confidence    float64
}

type LinearRegression struct {
	Slope      float64
	Intercept  float64
	RSquared   float64
	StdError   float64
	DataPoints int
	TimeRange  time.Duration
	Origin     time.Time
}

func FitLinear(points []CapacityPoint) *LinearRegression {
	if len(points) < 3 {
		return nil
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	tMin := points[0].Timestamp
	tMax := points[len(points)-1].Timestamp
	if tMax.Sub(tMin) < 24*time.Hour {
		return nil
	}
	n := float64(len(points))
	var sumT, sumU, sumTT, sumTU float64
	for _, p := range points {
		t := float64(p.Timestamp.Sub(tMin).Seconds())
		u := float64(p.UsedBytes)
		sumT += t
		sumU += u
		sumTT += t * t
		sumTU += t * u
	}
	denom := n*sumTT - sumT*sumT
	if denom == 0 {
		return nil
	}
	slope := (n*sumTU - sumT*sumU) / denom
	intercept := (sumU - slope*sumT) / n
	meanU := sumU / n
	var ssRes, ssTot float64
	for _, p := range points {
		t := float64(p.Timestamp.Sub(tMin).Seconds())
		u := float64(p.UsedBytes)
		ssRes += math.Pow(u-(slope*t+intercept), 2)
		ssTot += math.Pow(u-meanU, 2)
	}
	r2 := 1.0
	if ssTot > 0 {
		r2 = 1.0 - ssRes/ssTot
	}
	stdErr := 0.0
	if len(points) > 2 {
		stdErr = math.Sqrt(ssRes / float64(len(points)-2))
	}
	return &LinearRegression{
		Slope: slope, Intercept: intercept, RSquared: r2,
		StdError: stdErr, DataPoints: len(points),
		TimeRange: tMax.Sub(tMin), Origin: tMin,
	}
}

func (lr *LinearRegression) Project(historyEnd time.Time, interval time.Duration, count int, totalBytes int64) []Forecast {
	if lr == nil || lr.Slope <= 0 || count <= 0 {
		return nil
	}
	ci := 1.96
	if lr.RSquared < 0.5 {
		ci = 2.5
	}
	out := make([]Forecast, 0, count)
	for i := 1; i <= count; i++ {
		t := float64(historyEnd.Sub(lr.Origin).Seconds()) + float64(i)*interval.Seconds()
		pred := lr.Slope*t + lr.Intercept
		if pred < 0 {
			pred = 0
		}
		margin := ci * lr.StdError * math.Sqrt(1.0+float64(i)*0.05)
		lo := int64(pred - margin)
		if lo < 0 {
			lo = 0
		}
		hi := int64(pred + margin)
		if hi > totalBytes {
			hi = totalBytes
		}
		out = append(out, Forecast{
			Timestamp:     historyEnd.Add(interval * time.Duration(i)),
			PredictedUsed: int64(pred),
			LowerBound:    lo,
			UpperBound:    hi,
			Confidence:    math.Max(0, math.Min(1.0, lr.RSquared)),
		})
	}
	return out
}

func (lr *LinearRegression) DaysUntilFull(historyEnd time.Time, totalBytes int64, pct float64) *float64 {
	if lr == nil || lr.Slope <= 0 {
		return nil
	}
	target := float64(totalBytes) * pct / 100.0
	tNow := float64(historyEnd.Sub(lr.Origin).Seconds())
	currentPred := lr.Slope*tNow + lr.Intercept
	if currentPred >= target {
		z := 0.0
		return &z
	}
	secsNeeded := (target - currentPred) / lr.Slope
	days := secsNeeded / 86400.0
	return &days
}
