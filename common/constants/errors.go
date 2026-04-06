package constants

// Error name constants — used in ServiceError.ErrorName for fast identification.
const (
	// falcon-normalizer
	ErrNameNormalizerLLMParse   = "normalizer_llm_parse_failed"
	ErrNameNormalizerLLMRequest = "normalizer_llm_request_failed"

	// falcon-storage / company_logo
	ErrNameLogoDownloadFailed = "logo_download_failed"
	ErrNameLogoUploadFailed   = "logo_upload_failed"

	// falcon-scout
	ErrNameScrapeInspectFailed = "scrape_inspect_failed"

	// falcon-match-engine
	ErrNameMatchLLMFailed = "match_llm_failed"

	// falcon-signal
	ErrNameAPNsDeliveryFailed = "apns_delivery_failed"
)
