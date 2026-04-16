package interfaces

// Project is the standard interface every scraped project must implement,
// regardless of the source that produced it.
type Project interface {
	GetId() string
	GetURL() string
	GetPlatform() string
	GetPlatformId() string
	GetReferenceId() string
	GetPlatformUpdatedAt() string

	GetTitle() string
	GetCompany() string
	GetDescription() string
	GetStartDate() string
	GetEndDate() string
	GetDuration() string
	GetLocation() string

	GetSkills() []string
	GetRequiredSkills() []string

	GetRateRaw() string
	GetRateAmount() *float64
	GetRateCurrency() string
	GetRateType() string

	GetContactName() string
	GetContactCompany() string
	GetContactEmail() string
	GetContactPhone() string
	GetContactRole() string
	GetContactAddress() string
	GetContactImage() string

	IsDirectClient() bool
	IsRemote() bool
	IsANUE() bool
}
