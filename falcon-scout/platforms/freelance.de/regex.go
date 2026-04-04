package freelancede

import "regexp"

// rateRe matches German rate strings like "550 € Tagessatz" or "47,50 € Stundensatz".
// Group 1: amount, Group 2: currency symbol, Group 3: rate type.
var rateRe = regexp.MustCompile(`(?i)^([\d.,]+)\s*(€|\$|CHF|USD|EUR)\s+(Tagessatz|Stundensatz)$`)

var platformIDRe = regexp.MustCompile(`projekt-\d+`)
