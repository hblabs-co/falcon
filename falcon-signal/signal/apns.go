package signal

import (
	"context"
	"fmt"
	"strings"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
)

type apnsClient struct {
	client   *apns2.Client
	bundleID string
}

func newAPNSClient() (*apnsClient, error) {
	values, err := helpers.ReadEnvs("APNS_KEY_PATH", "APNS_KEY_ID", "APNS_TEAM_ID", "APNS_BUNDLE_ID")
	if err != nil {
		return nil, err
	}
	keyPath, keyID, teamID, bundleID := values[0], values[1], values[2], values[3]

	authKey, err := token.AuthKeyFromFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load apns key %s: %w", keyPath, err)
	}

	t := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}

	production := helpers.ReadEnvOptional("APNS_PRODUCTION", "false") == "true"
	var client *apns2.Client
	if production {
		client = apns2.NewTokenClient(t).Production()
	} else {
		client = apns2.NewTokenClient(t).Development()
	}

	return &apnsClient{client: client, bundleID: bundleID}, nil
}

// Send delivers a push notification to the given device token.
func (a *apnsClient) Send(ctx context.Context, deviceToken string, result *models.MatchResultEvent) error {
	p := payload.NewPayload().
		AlertTitle(result.ProjectTitle).
		AlertSubtitle(labelTitle(result.Label)).
		AlertBody(result.Summary).
		Sound("default").
		Category("MATCH_RESULT").
		Custom("project_id", result.ProjectID).
		Custom("cv_id", result.CVID).
		Custom("score", result.Score).
		Custom("label", string(result.Label)).
		Custom("summary", result.Summary).
		Custom("matched_skills", result.MatchedSkills).
		Custom("missing_skills", result.MissingSkills).
		Custom("scores", map[string]float32{
			"skills_match":          result.Scores.SkillsMatch,
			"seniority_fit":         result.Scores.SeniorityFit,
			"domain_experience":     result.Scores.DomainExperience,
			"communication_clarity": result.Scores.CommunicationClarity,
			"project_relevance":     result.Scores.ProjectRelevance,
			"tech_stack_overlap":    result.Scores.TechStackOverlap,
		})

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.bundleID,
		Payload:     p,
	}

	resp, err := a.client.PushWithContext(ctx, notification)
	if err != nil {
		return fmt.Errorf("apns push: %w", err)
	}
	if !resp.Sent() {
		return fmt.Errorf("apns rejected: %s (%d)", resp.Reason, resp.StatusCode)
	}

	logrus.Infof("apns sent — apns_id=%s device=%s…", resp.ApnsID, deviceToken[:8])
	return nil
}

// SendAdminAlert delivers a high-severity operational alert to an admin device.
// Used by AdminNotifier when a service publishes an AdminAlertEvent. The
// payload carries enough context (error_id, source, platform, severity) for
// the iOS app to deep-link straight into the offending error from the alert.
func (a *apnsClient) SendAdminAlert(ctx context.Context, deviceToken string, errDoc *models.ServiceError) error {
	title := errDoc.ErrorName
	if errDoc.Platform != "" {
		title = errDoc.Platform + " — " + errDoc.ErrorName
	}

	p := payload.NewPayload().
		AlertTitle(title).
		AlertSubtitle(strings.ToUpper(string(errDoc.Priority))).
		AlertBody(errDoc.Error).
		Sound("default").
		Category("ADMIN_ALERT").
		Custom("error_id", errDoc.ID).
		Custom("source", errDoc.ServiceName).
		Custom("platform", errDoc.Platform).
		Custom("error_name", errDoc.ErrorName).
		Custom("severity", string(errDoc.Priority))

	notification := &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       a.bundleID,
		Payload:     p,
	}

	resp, err := a.client.PushWithContext(ctx, notification)
	if err != nil {
		return fmt.Errorf("apns push: %w", err)
	}
	if !resp.Sent() {
		return fmt.Errorf("apns rejected: %s (%d)", resp.Reason, resp.StatusCode)
	}

	logrus.Infof("admin apns sent — apns_id=%s device=%s…", resp.ApnsID, deviceToken[:8])
	return nil
}

// IsStaleToken returns true when the APNs error indicates the device token is
// no longer valid (device unregistered or token expired). The caller should
// remove the token from the database.
func (a *apnsClient) IsStaleToken(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "BadDeviceToken") || strings.Contains(msg, "Unregistered")
}

func labelTitle(label models.MatchLabel) string {
	switch label {
	case models.MatchLabelApplyImmediately:
		return "Jetzt bewerben!"
	case models.MatchLabelTopCandidate:
		return "Starker Kandidat"
	default:
		return "Neue Projektempfehlung"
	}
}
