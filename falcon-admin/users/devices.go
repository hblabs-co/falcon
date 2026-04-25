package users

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

func listUserDevices(c *gin.Context) {
	id := c.Param("id")
	devices, err := listDevicesFor(c.Request.Context(), id)
	if err != nil {
		respondInternal(c, "list devices", err)
		return
	}
	out := make([]deviceView, 0, len(devices))
	for _, d := range devices {
		out = append(out, deviceView{
			ID:       d.ID,
			DeviceID: d.DeviceID,
			// Hardcoded — the source collection is ios_device_tokens.
			// Once android/web devices are persisted, switch this to
			// read from a unified document field.
			Platform:          "ios",
			TokenMasked:       maskAPNs(d.Token),
			HasLiveActivity:   d.LiveActivityToken != "",
			HasUpdateActivity: d.LiveActivityUpdateToken != "",
			CreatedAt:         d.CreatedAt,
			UpdatedAt:         d.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "devices": out})
}

func listDevicesFor(ctx context.Context, userID string) ([]models.IOSDeviceToken, error) {
	var devices []models.IOSDeviceToken
	err := system.GetStorage().GetAllByField(ctx,
		constants.MongoIOSDeviceTokensCollection, "user_id", userID, &devices)
	return devices, err
}

// maskAPNs returns the first 6 + last 4 hex chars of an APNs token,
// enough to differentiate devices in the UI without leaking a full
// token (which is push-capable PII).
func maskAPNs(t string) string {
	if len(t) <= 12 {
		return t
	}
	return t[:6] + "…" + t[len(t)-4:]
}
