package interfaces

// Project is the standard interface every scraped project must implement,
// regardless of the source that produced it.
type Project interface {
	GetId() string
	GetURL() string
	GetPlatform() string
	GetPlatformId() string
	GetPlatformUpdatedAt() string

	GetTitle() string
	GetCompany() string
	GetDescription() string
	GetStartDate() string
	GetEndDate() string
	GetLocation() string

	GetSkills() []string
	GetRequiredSkills() []string

	GetRate() Rate
	GetContact() Contact

	IsDirectClient() bool
	IsRemote() bool
	IsANUE() bool
}
