package redglobalde

const Source = "redglobal.de"

// CompanyID is the platform-assigned ID stored in the companies collection,
// originally created by the freelance.de scraper when it processed a RED Global
// job posting. The metadata loop uses it to locate the company doc and refresh
// its robots.txt / security.txt / humans.txt / sitemap fields.
const CompanyID = "6070"

// Hostnames accepted by colly's AllowedDomains and used to scope rate limits.
const (
	hostBare = "redglobal.de"
	hostWWW  = "www.redglobal.de"
	// hostGlob matches both bare and www variants for colly.LimitRule.
	hostGlob = "*redglobal.*"
)

const baseURL = "https://" + hostWWW

// searchURL is the listing page for contract/freelance jobs.
// types[0]=0 filters for "Freiberuflich" (contract) positions.
const searchURL = baseURL + "/jobs/search?keywords=&role=&workplace=&types%5B0%5D=0"
