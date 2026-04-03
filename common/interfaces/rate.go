package interfaces

import "hblabs.co/falcon/common/constants"

// Rate is a parsed representation of a project's payment rate.
// Raw always holds the original scraped string; Amount, Currency and Type are
// populated only when the string can be parsed (e.g. "550 € Tagessatz").
type Rate interface {
	GetRaw() string
	GetAmount() *float64
	GetCurrency() string
	GetType() constants.RateType
}
