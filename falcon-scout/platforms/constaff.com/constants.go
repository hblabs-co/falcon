package constaffcom

const Source = "constaff.com"

// CompanyID is the hblabs.co identifier for ConStaff GmbH.
const CompanyID = "4552"

const baseURL = "https://www.constaff.com"

// apiURL is the WordPress admin-ajax endpoint that proxies to the
// staffITpro backend. POST as multipart/form-data with fields:
//   - action             = "searchProjects"
//   - contractsAsString[]= "Contracting"
//   - selectedContracts[]= "5", "3", "1"  (the 3 freelance-type IDs)
//   - searchQuery        = ""
//   - page               = "1" (1-indexed)
const apiURL = baseURL + "/wp-admin/admin-ajax.php"

// contactsURL is the WP REST API endpoint for the "Ansprechpartner"
// (contact person) custom post type. Returns all entries in one page
// (per_page=100 covers the full team). Fetched once on Init, refreshed
// daily by the worker.
const contactsURL = baseURL + "/wp-json/wp/v2/ansprechpartner?per_page=100"
