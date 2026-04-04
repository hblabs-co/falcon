package freelancede

import (
	"testing"

	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
)

// =============================================================================
// parseRate unit tests
// =============================================================================

func TestParseRate(t *testing.T) {
	cases := []struct {
		raw      string
		amount   *float64
		currency string
		rateType constants.RateType
	}{
		// Daily rate, integer amount
		{"550 € Tagessatz", helpers.Ptr(550), "EUR", constants.RateTypeDaily},
		// Hourly rate, integer amount
		{"47 € Stundensatz", helpers.Ptr(47), "EUR", constants.RateTypeHourly},
		// Daily rate with thousands separator (German dot)
		{"1.200 € Tagessatz", helpers.Ptr(1200), "EUR", constants.RateTypeDaily},
		// Hourly rate with decimal comma
		{"47,50 € Stundensatz", helpers.Ptr(47.50), "EUR", constants.RateTypeHourly},
		// CHF currency
		{"850 CHF Tagessatz", helpers.Ptr(850), "CHF", constants.RateTypeDaily},
		// USD via $ symbol
		{"120 $ Stundensatz", helpers.Ptr(120), "USD", constants.RateTypeHourly},
		// "auf Anfrage" — no amount, no currency, no type
		{"auf Anfrage", nil, "", ""},
		// Empty string
		{"", nil, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			r := parseRate(tc.raw)
			helpers.CheckStrings(t, r.Raw, tc.raw, "Raw")
			if tc.amount == nil {
				if r.Amount != nil {
					t.Errorf("Amount: want nil, got %v", *r.Amount)
				}
			} else {
				if r.Amount == nil {
					t.Fatalf("Amount: want %v, got nil", *tc.amount)
				}
				if *r.Amount != *tc.amount {
					t.Errorf("Amount: want %v, got %v", *tc.amount, *r.Amount)
				}
			}
			helpers.CheckStrings(t, r.Currency, tc.currency, "Currency")
			helpers.CheckStrings(t, string(r.Type), string(tc.rateType), "Type")
		})
	}
}
