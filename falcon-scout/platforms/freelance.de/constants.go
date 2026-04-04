package freelancede

const Source = "freelance.de"
const examplesDir = "./examples/"
const baseUrl = "https://www.freelance.de"
const projectCandidatesURL = baseUrl + "/projekte?pageSize=100"

// tokenEndpoint exchanges session cookies for a JWT access token.
const tokenEndpoint = "/api/ui/users/access-token"

// projectsSearchURL is the API endpoint for searching projects.
const projectsSearchURL = baseUrl + "/api/ui/projects/search"
