package constants

// RateType indicates whether a rate is charged daily or hourly.
type RateType string

const (
	RateTypeDaily  RateType = "daily"
	RateTypeHourly RateType = "hourly"
)
