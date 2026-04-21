package realtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// Clients sign (timestamp|device_id|platform) with the shared secret baked
// into the app binary. This is not a real authentication system — it's
// enough to keep random internet traffic off the WebSocket while being
// cheap to implement on every client. For strong assurance (binding the
// connection to an actual Apple/Google device) we'd add App Attest and
// Play Integrity on top; see followup-work.md.
//
// The timestamp window keeps replays bounded. A leaked app binary exposes
// the shared key — treat this as a speedbump, not a security boundary.
const (
	clockSkew = 5 * time.Minute
)

// ComputeSignature is exposed so tests and CLI tools can produce the same
// signature the client does. The signing string is stable: we never change
// its composition without bumping the key.
func ComputeSignature(secret, timestamp, deviceID, platform string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("|"))
	mac.Write([]byte(deviceID))
	mac.Write([]byte("|"))
	mac.Write([]byte(platform))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHandshake returns nil when the signature is valid and the
// timestamp is within ±clockSkew of the server clock. The caller must
// ensure the fields came from trusted transport headers — we don't
// validate format beyond what the check requires.
func VerifyHandshake(secret, timestamp, deviceID, platform, signature string) error {
	if secret == "" {
		return fmt.Errorf("REALTIME_SHARED_SECRET not configured")
	}
	if timestamp == "" || deviceID == "" || platform == "" || signature == "" {
		return fmt.Errorf("missing handshake headers")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	drift := time.Since(time.Unix(ts, 0))
	if drift < -clockSkew || drift > clockSkew {
		return fmt.Errorf("timestamp outside ±%s window (drift=%s)", clockSkew, drift)
	}

	expected := ComputeSignature(secret, timestamp, deviceID, platform)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
