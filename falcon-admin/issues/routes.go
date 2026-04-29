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
func Mount(parent *gin.RouterGroup) {

	issuesGroup := parent.Group("/issues")

	issuesGroup.GET("", listIssues)
	issuesGroup.POST("/:type/:id/resolve", resolveOne)
	issuesGroup.POST("/resolve-all", resolveAll)
}
