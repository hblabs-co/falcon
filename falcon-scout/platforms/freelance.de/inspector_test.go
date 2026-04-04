package freelancede

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gocolly/colly/v2"
	"hblabs.co/falcon/common/helpers"
)

// scrapeFile serves the given HTML file via a local HTTP server and runs the
// freelance.de HTML handlers against it, returning the populated project.
func scrapeFile(t *testing.T, subpath, filename string) Project {
	t.Helper()
	from := fmt.Sprintf("%v%v/%v", examplesDir, subpath, filename)
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := colly.NewCollector()
	var project Project
	project.URL = srv.URL
	registerHandlers(c, &project)
	if err := c.Visit(srv.URL); err != nil {
		t.Fatalf("visit: %v", err)
	}
	return project
}

// -----------------------------------------------------------------------------
// projekt-1258632 – Angular Entwickler
// Old contact layout (col-md-6): no .list-item-main → Contact is nil.
// -----------------------------------------------------------------------------

func TestAngularDeveloper_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1258632-Angular-Entwickler-m-w-d-3-Tage-Woche-vor-Ort.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Angular Entwickler (m/w/d) 3 Tage/Woche vor Ort in Frankfurt, (ID: 30220)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Contractor Consulting GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Oktober 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Frankfurt am Main", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 17:01", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "30220", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden aus dem Finanzumfeld") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description)[:60])
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Softwareentwicklung / -programmierung",
		"Typescript",
		"Web",
		"Angular",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Contractor Consulting GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Martin Neusser", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Email, "martin.neusser@contractor.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "34", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Adams-Lehmann-Straße 56 80797 München Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1262909 – Azure DevOps Architect
// Old contact layout: no .list-item-main → Contact is nil. No skills section.
// -----------------------------------------------------------------------------

func TestAzureDevOpsArchitect_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1262909-Azure-DevOps-m-w-d-Architect.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Azure DevOps (m/w/d) Architect", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Akkodis Germany Tech Freelance GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "September 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Frankfurt am Main", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:39", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "JN -032026-75659_1774968410", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "For our client, we are looking for an Azure DevOps Architect") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description)[:70])
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Akkodis Germany Tech Freelance GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Valerie Dziwok", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Email, "valerie.dziwok.39619.1200@akkodis.aplitrak.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 7113516630", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Friedrichstr. 6 70174 Stuttgart Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263124 – AFRA Senior Frontend Developer
// New contact layout (.list-item-main) with role, phone (+49 only), address.
// -----------------------------------------------------------------------------

func TestAFRASeniorFrontendDeveloper_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263124-AFRA-10225-Senior-Entwickler-w-m-d-Frontend.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "AFRA-10225-Senior Entwickler (w/m/d) Frontend", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Randstad Digital Germany AG", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Berlin", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "550 € Tagessatz", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 16:14", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden suche ich einen Senior Entwickler") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description)[:60])
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{"Web", "React (JavaScript library)"})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Randstad Digital Germany AG", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Jeannine Keith", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "External Recruiting Expert", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "jeannine.keith@randstaddigital.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 (0) 173 896 57 95", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Christoph-Rapparini-Bogen 29 80639 München Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263164 – Luware Specialist
// New contact layout with role and full phone number on a single token.
// -----------------------------------------------------------------------------

func TestLuwareSpecialist_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263164-Luware-Spezialist-m-w-d.html.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Luware-Spezialist (m/w/d)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "ambass GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Mai 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Lüneburg", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 17:52", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für einen unserer Bestandskunden bin ich derzeit") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description)[:60])
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{"Weitere IT-Qualifikationen", "Microsoft Office 365"})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "ambass GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Steffen Immink", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "Personalberater IT", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "s.immink@ambass-group.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "01604558724", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Cecilienallee 10 40474 Düsseldorf Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263059 – Senior DevOps Engineer (Banking)
// New layout. Long skills list (many categories). Swiss phone (+41).
// -----------------------------------------------------------------------------

func TestSeniorDevOpsEngineerBanking_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263059-Senior-DevOps-Engineer-m-w-d-Digital-Client-Onboarding-Banking.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Senior DevOps Engineer (m/w/d) - Digital Client Onboarding (Banking)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Wavestone AG", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Oktober 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "September 2029", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "CH-Zürich", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 12:10", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "46467", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für ein langfristiges Projekt bei unserem Kunden in Zürich") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Systementwickler und -analytiker", "Solution Architekt",
		"DevOps", "DevOps",
		"Mitarbeiter Bankfiliale", "Mitarbeiter Investmentbank",
		"Manager Versicherungen und Finanzen", "Leiter Finanzierung",
		"Sicherheitstechnik", "Gesundheit und Sicherheit am Arbeitsplatz",
		"Management (Technik)", "Lifecycle Management",
		"Wissenschaft", "Informatik",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Wavestone AG", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Vivek Kandiah", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "STS Senior Consultant (Recruitment Consultant)", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "vivek.kandiah@wavestone.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+41762238515", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Leopoldstraße 28a 80802 München Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263068 – DevOps Experte
// New layout. Duplicate "DevOps" skills (two separate category entries).
// -----------------------------------------------------------------------------

func TestDevOpsExperte_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263068-DevOps-Experte.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "DevOps Experte", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "LUDO Solutions GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Wolfsburg", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 12:32", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "DevOps Experte | ab sofort 6+ Monate | Großraum Wolfsburg") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{"DevOps", "DevOps"})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "LUDO Solutions GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Christof Schäfer", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "Geschäftsführer", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "christof.schaefer@ludo-solutions.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+491748197982", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Hauptstraße 19a 74838 Limbach Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263082 – Fullstack Entwickler Backend
// New layout.
// -----------------------------------------------------------------------------

func TestFullstackEntwicklerBackend_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263082-Fullstack-Entwickler-mit-Schwerpunkt-Backend-m-w-d.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Fullstack Entwickler mit Schwerpunkt Backend (m/w/d)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "HR Solutions Services GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Berlin", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 13:36", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden im Öffentlichen Sektor sind wir momentan auf der") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{"Web", "Angular"})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "HR Solutions Services GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Olga Krauter", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "Geschäftsführerin / Recruiting", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "recruiting@hr-solutions.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "071136560230", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Hellmuth-Hirth-Str. 5 73760 Ostfildern Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263104 – Android App Entwickler
// New layout. EndDate is "nicht angegeben". Role contains pipe character.
// -----------------------------------------------------------------------------

func TestAndroidAppEntwickler_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263104-Android-App-Entwickler-w-m-d.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Android App Entwickler (w/m/d)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Etengo AG", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "nicht angegeben", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Osnabrück", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:00", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "CA-101424", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden suchen wir einen Android App Entwickler (w/m/d)") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Programmierer", "JavaScript-Entwickler",
		"Systementwickler und -analytiker", "Solution Architekt",
		"Web", "Laravel", "AngularJS",
		"Ingenieure, Konstrukteure und Techniker Chemie", "Ingenieur Verfahrenstechnik",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Etengo AG", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Estelle Kasimatis", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "Department Manager | Client & Partner Services", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "estelle.kasimatis@etengo.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "0621150212827", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Konrad-Zuse-Ring 27 68163 Mannheim Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263107 – remote DevOps Engineer
// New layout. Multi-city location. Short description.
// -----------------------------------------------------------------------------

func TestRemoteDevOpsEngineer_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263107-remote-DevOps-Engineer-w-m-d-POS72320-Python-Angular-Terraform.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "remote: DevOps Engineer w/m/d POS72320 Python Angular Terraform GitLab Spring Boot", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "CAES GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Frankfurt am Main | D-Großraum Berlin | D-Großraum Hamburg | D-Großraum Köln | D-Großraum München", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:06", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Start: 20.04.2026") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"DevOps", "DevOps (allg.)",
		"Softwareentwicklung / -programmierung", "Python",
		"Web", "Angular", "Elasticsearch",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "CAES GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Rafael Gallus", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "WFM", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "gallus@caes.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "08232 906546", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Wiesenstr 32 86836 Augsburg- Untermeitingen Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263115 – Native Android Entwickler
// New layout. Contact has no role.
// -----------------------------------------------------------------------------

func TestNativeAndroidEntwickler_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263115-Native-Android-Entwickler-m-w-x.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Native Android Entwickler (m/w/x)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Computer Futures, part of SThree", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Oktober 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Dortmund", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:40", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "CR/4049660_1775050400", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für meinen Top-Kunden suche ich aktuell zwei Native Android Entwickler") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Systementwickler und -analytiker", "Linux-Systemadministrator",
		"System Architektur / Analyse", "Cloud Computing",
		"Softwareentwicklung / -programmierung", "Gradle", "Kotlin",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Computer Futures, part of SThree", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Shanon Ann Maslin", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "slin.38073.1200@sthreede2.aplitrak.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 89 5519 78223", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Arnulfstraße 31 80636 München Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263135 – Experte Apache Solr
// New layout. Contact has no role.
// -----------------------------------------------------------------------------

func TestExperteApacheSolr_OK(t *testing.T) {
	p := scrapeFile(t, "ok", "projekt-1263135-Experte-Apache-Solr-m-w-d.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Experte Apache Solr (m/w/d)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Krongaard GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Juli 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "Niedersachsen", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 16:15", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "38755", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden suchen wir einen Experten für Apache Solr und") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Systementwickler und -analytiker", "Linux-Systemadministrator",
		"Berater und Spezialisten", "IT-Infrastrukturspezialist",
		"Web", "Elasticsearch", "Apache Solr",
		"Bildungswesen und Training (sonstige)", "Leiter wissenschaftliche Untersuchungen",
		"Rechnungswesen", "Berechnungen",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Krongaard GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Annika Ehrenberg", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "Annika.Ehrenberg@krongaard.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "040 30 38 44 258", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Fuhlentwiete 10 20355 Hamburg Deutschland", "Contact.Address")
}

// =============================================================================
// Direct client projects (Direktkunden-Projekt badge, no Kontaktdaten section)
// =============================================================================

// -----------------------------------------------------------------------------
// projekt-1263127 – Senior Scala Developer
// Direct client. Contact is nil.
// -----------------------------------------------------------------------------

func TestSeniorScalaDeveloper_Direct(t *testing.T) {
	p := scrapeFile(t, "direct", "projekt-1263127-Senior-Scala-Developer-Kubernetes-AWS-CI-CD-m-w-d.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Senior Scala Developer (Kubernetes, AWS, CI/CD) (m,w,d)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Operations Team", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-60306 Frankfurt am Main", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:56", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "2026_04_01_01_52_13", "RefNr")

	helpers.CheckBool(t, p.IsDirectClient(), true, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Web", "Kubernetes",
		"Amazon Web Services (AWS)",
		"Softwareentwicklung / -programmierung", "Scala", "Git",
	})
	helpers.CheckSkills(t, p.RequiredSkills, []string{"Kubernetes", "Amazon Web Services (AWS)", "Scala", "Git", "Protobuf"})

	if p.Contact != nil {
		t.Errorf("Contact: expected nil for direct-client page, got %+v", p.Contact)
	}
}

// -----------------------------------------------------------------------------
// projekt-1262922 – Senior Performance Marketing Expert
// Direct client. No RefNr. Contact is nil.
// -----------------------------------------------------------------------------

func TestSeniorPerformanceMarketing_Direct(t *testing.T) {
	p := scrapeFile(t, "direct", "projekt-1262922-Senior-Performance-Marketing-Expert-Freelance-Skalierung-Paid-Social-Lead.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Senior Performance Marketing Expert (Freelance) – Skalierung Paid Social & Lead Gen", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "anstoss24", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Oktober 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum München", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "47 € Stundensatz", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 10:40", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	helpers.CheckBool(t, p.IsDirectClient(), true, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), false, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{"Marketing", "Social Media Marketing"})
	helpers.CheckSkills(t, p.RequiredSkills, []string{"Social Media Marketing"})

	if p.Contact != nil {
		t.Errorf("Contact: expected nil for direct-client page, got %+v", p.Contact)
	}
}

// =============================================================================
// ANUE projects (Arbeitnehmerüberlassung badge in company-name)
// =============================================================================

// -----------------------------------------------------------------------------
// projekt-1262388 – Experte Arbeitssicherheit
// ANUE badge. Contact new layout. No RefNr.
// -----------------------------------------------------------------------------

func TestExperteArbeitssicherheit_ANUE(t *testing.T) {
	p := scrapeFile(t, "anue", "projekt-1262388-Experte-m-w-d-Arbeitssicherheit-in-ANUe.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Experte (m/w/d) Arbeitssicherheit in ANÜ", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "MERSOL GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "April 2027", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Hamburg", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 15:50", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für einen Einsatz in Arbeitnehmerüberlassung suchen wir einen erfahr") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), true, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Sicherheitstechnik",
		"Arbeitsschutzmanagement / Arbeitssicherheitsmanagement",
		"Fachkraft für Arbeitssicherheit (FASi)",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "MERSOL GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Julian Schotten", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "Managing Director", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "experten@mersol.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 69 2009 1447", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Hanauer Landstraße 114 60314 Frankfurt am Main Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263006 – Java Entwickler
// ANUE badge. Contact new layout. RefNr present.
// -----------------------------------------------------------------------------

func TestJavaEntwickler_ANUE(t *testing.T) {
	p := scrapeFile(t, "anue", "projekt-1263006-Java-Entwickler-m-w-d-Remote-GR-Frankfurt-Mai-2026.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Java Entwickler (m/w/d) // Remote/GR Frankfurt // Mai 2026 // 8 Monate +", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Windhoff Staffing Services", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "Dezember 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Großraum Frankfurt am Main", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 10:45", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "2181", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für ein Kundenprojekt im Mobilitätsumfeld suchen wir zu Mai 2026") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), true, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Programmierer", "Java-Entwickler",
		"Softwareentwicklung / -programmierung", "Spring Framework",
		"Web", "Amazon Web Services (AWS)",
	})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Windhoff Staffing Services", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Marlin Schumacher", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "IT-Recruiter", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "m.schumacher@windhoff-group.de", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "0254295590", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Am Campus 17 48712 Gescher Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263064 – Spezialist Projektcontrolling
// ANUE badge. No skills. Contact has no role. ATS email.
// -----------------------------------------------------------------------------

func TestSpezialistProjektcontrolling_ANUE(t *testing.T) {
	p := scrapeFile(t, "anue", "projekt-1263064-Spezialist-in-controlling-m-w-d-IT-Dienstleistungen-ANUe.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Spezialist*in Projektcontrolling (m/w/d) – IT & Dienstleistungen (ANÜ)", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "eightbit experts GmbH", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "April 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "September 2026", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Oldenburg", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "Remote", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 13:04", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "JO-2604-14141", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Für unseren Kunden suchen wir asap nach einem:einer Spezialist*in Pro") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), true, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "eightbit experts GmbH", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Janica Ewert", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "c114u807j312453b175@ats-eu.yourecruit.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 40 42925700", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Borselstr. 26 22765 Hamburg Deutschland", "Contact.Address")
}

// -----------------------------------------------------------------------------
// projekt-1263084 – Senior Development Engineer Embedded Vision
// ANUE badge. Contact has no role. Large skills list.
// -----------------------------------------------------------------------------

func TestSeniorDevelopmentEngineer_ANUE(t *testing.T) {
	p := scrapeFile(t, "anue", "projekt-1263084-Senior-Development-Engineer-w-m-d-Software-Embedded-Vision.html")

	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Title), "Senior Development Engineer (w/m/d) Software Embedded Vision", "Title")
	helpers.CheckStrings(t, helpers.NormalizeText(p.Overview.Company), "Computer Futures, part of SThree", "Company")
	helpers.CheckStrings(t, p.Overview.StartDate, "Mai 2026", "StartDate")
	helpers.CheckStrings(t, p.Overview.EndDate, "April 2027", "EndDate")
	helpers.CheckStrings(t, p.Overview.Location, "D-Augsburg", "Location")
	helpers.CheckStrings(t, p.Rate.Raw, "auf Anfrage", "HourlyRate")
	helpers.CheckStrings(t, p.Overview.Remote, "", "Remote")
	helpers.CheckStrings(t, p.Overview.LastUpdate, "01.04.2026 14:27", "LastUpdate")
	helpers.CheckStrings(t, p.Overview.RefNr, "TR/4049636_1775043967", "RefNr")

	if !strings.HasPrefix(helpers.NormalizeText(p.Description), "Senior Development Engineer (w/m/d) Software für Embedded Vision") {
		t.Errorf("Description prefix mismatch: %q", helpers.NormalizeText(p.Description))
	}

	helpers.CheckBool(t, p.IsDirectClient(), false, "IsDirectClient")
	helpers.CheckBool(t, p.IsANUE(), true, "IsANUE")

	helpers.CheckSkills(t, p.Skills, []string{
		"Weitere IT-Qualifikationen", "Rechnerarchitektur",
		"IT-Koordinatoren", "Helpdesk-Koordinator im Bereich Automatisierung",
		"Leitende Angestelle IT", "Projektleiter im Bereich Information",
		"Programmierer", "Python-Programmierer", "Programmierer C, C++",
		"Techniker Installation und Wartung elektrischer Geräte", "Telekommunikationstechniker",
		"Mess- / Regelungstechnik", "Sensorik", "Embedded Software",
		"Mitarbeiter Marketing und Werbung", "Marketingberater",
		"Regisseure und Produzenten", "Multimedia Producer",
		"Wissenschaft", "Informatik", "Physik",
	})
	helpers.CheckSkills(t, p.RequiredSkills, []string{})

	if p.Contact == nil {
		t.Fatal("Contact: expected non-nil")
	}
	helpers.CheckStrings(t, p.Contact.Company, "Computer Futures, part of SThree", "Contact.Company")
	helpers.CheckStrings(t, p.Contact.Name, "Simone Schenk", "Contact.Name")
	helpers.CheckStrings(t, p.Contact.Role, "", "Contact.Role")
	helpers.CheckStrings(t, p.Contact.Email, "sosk.37725.1200@sthreede2.aplitrak.com", "Contact.Email")
	helpers.CheckStrings(t, p.Contact.Phone, "+49 89 5519 78223", "Contact.Phone")
	helpers.CheckStrings(t, p.Contact.Address, "Arnulfstraße 31 80636 München Deutschland", "Contact.Address")
}
