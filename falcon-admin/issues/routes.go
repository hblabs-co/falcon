package issues

import "github.com/gin-gonic/gin"

// Mount wires the /issues admin routes onto the already-
// authenticated bearer group.
//
// Resource map:
//
//	GET    /issues                      list errors + warnings (filters)
//	POST   /issues/:type/:id/resolve    mark a single issue resolved
//	POST   /issues/resolve-all          bulk-resolve (filter via query)
func Mount(g *gin.RouterGroup) {
	g.GET("/issues", listIssues)
	g.POST("/issues/:type/:id/resolve", resolveOne)
	g.POST("/issues/resolve-all", resolveAll)
}
