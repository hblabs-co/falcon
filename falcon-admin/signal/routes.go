package signal

import "github.com/gin-gonic/gin"

// Mount wires every signal-test trigger onto the already-
// authenticated admin group. Caller (`falcon-admin/admin/service.go`)
// is responsible for the bearer-token middleware.
func Mount(parent *gin.RouterGroup) {

	signalGroup := parent.Group("/signal")

	signalGroup.GET("/test-alert", TestAlert)
	signalGroup.GET("/test-last-match", TestLastMatch)
	signalGroup.GET("/test-push", TestPush)
	signalGroup.GET("/test-email", TestEmail)
}
