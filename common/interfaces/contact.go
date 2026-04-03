package interfaces

// Contact holds normalized contact/person data scraped from a project page.
type Contact interface {
	GetCompany() string
	GetName() string
	GetRole() string
	GetEmail() string
	GetPhone() string
	GetAddress() string
}
