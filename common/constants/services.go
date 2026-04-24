package constants

// Service-name constants — kebab-case, one const per Go binary that
// ships under falcon-*. Used as the stable identifier when services
// self-register in the `system` collection, as the title of the
// startup banner, and anywhere else a human-readable name is needed.
// Keep in sync with directory names.
const (
	ServiceNormalizer           = "falcon-normalizer"
	ServiceScout                = "falcon-scout"
	ServiceAPI                  = "falcon-api"
	ServiceAuth                 = "falcon-auth"
	ServiceAuthorizer           = "falcon-authorizer"
	ServiceDispatch             = "falcon-dispatch"
	ServiceMatchEngine          = "falcon-match-engine"
	ServiceSignal               = "falcon-signal"
	ServiceStorage              = "falcon-storage"
	ServiceRealtime             = "falcon-realtime"
	ServiceLanding              = "falcon-landing"
	ServiceImport               = "falcon-import"
	ServiceDesigner             = "falcon-designer"
)
