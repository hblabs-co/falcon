package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listUserConfigs returns every UserConfig row tied to a given
// user_id. The collection is small (per-user, per-platform,
// per-device, per-name compound key) so we don't paginate; the UI
// renders them grouped by platform/device.
func listUserConfigs(c *gin.Context) {
	id := c.Param("id")
	if _, ok := loadUserOr404(c, id); !ok {
		return
	}
	var configs []models.UserConfig
	if err := system.GetStorage().GetMany(c.Request.Context(),
		constants.MongoUsersConfigurationsCollection,
		bson.M{"user_id": id}, &configs); err != nil {
		respondInternal(c, "list user configs", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(configs), "configs": configs})
}
