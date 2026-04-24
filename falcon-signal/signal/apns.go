package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/models"
)

type apnsClient struct {
	client   *apns2.Client
	bundleID string
}

func newAPNSClient() (*apnsClient, error) {
	values, err := environment.ReadMany("APNS_KEY_PATH", "APNS_KEY_ID", "APNS_TEAM_ID", "APNS_BUNDLE_ID")
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

	production := environment.ReadOptional("APNS_PRODUCTION", "false") == "true"
	var client *apns2.Client
	if production {
		client = apns2.NewTokenClient(t).Production()
	} else {
		client = apns2.NewTokenClient(t).Development()
	}

	return &apnsClient{client: client, bundleID: bundleID}, nil
}

// Send delivers a push notification to the given device token, localized to
// lang. Summary and label title are picked from the multi-lang fields on the
// result; lang falls back to "de" (the authoritative source language) when a
// translation is missing. totalMatches is the user's current total match
// count for the Live Activity header.
func (a *apnsClient) Send(ctx context.Context, deviceToken string, result *models.MatchResultEvent, lang string, totalMatches int) error {
	summary := result.Summary[lang]
	if summary == "" {
		summary = result.Summary["de"]
	}

	p := payload.NewPayload().
		AlertTitle(result.ProjectTitle).
		AlertSubtitle(labelTitle(result.Label, lang)).
		AlertBody(summary).
		Sound("default").
		Category("MATCH_RESULT").
		Custom("project_id", result.ProjectID).
		Custom("cv_id", result.CVID).
		Custom("score", result.Score).
		Custom("label", string(result.Label)).
		Custom("summary", summary).
		Custom("company_name", result.CompanyName).
		Custom("company_logo_url", result.CompanyLogoURL).
		Custom("total_matches", totalMatches).
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

// SendMatchLiveActivityStart sends a push-to-start APNs payload that makes iOS
// spin up a FalconMatchAttributes Live Activity on Lock Screen / Dynamic Island
// without the app running. Requires iOS 17.2+ and a liveActivityToken obtained
// from Activity.pushToStartTokenUpdates.
//
// The payload includes an inline alert so the same push also shows the normal
// banner — one push, two effects. Devices on older iOS don't register a
// liveActivityToken so they never receive this payload and fall back to Send.
//
// Topic MUST be "<bundle>.push-type.liveactivity" with header
// apns-push-type: liveactivity.
func (a *apnsClient) SendMatchLiveActivityStart(ctx context.Context, liveActivityToken string, result *models.MatchResultEvent, lang string, totalMatches int) error {
	summary := result.Summary[lang]
	if summary == "" {
		summary = result.Summary["de"]
	}

	// Raw aps structure — apns2's payload builder doesn't expose the
	// liveactivity-specific keys (event, content-state, attributes-type), so
	// we construct the payload by hand.
	body := map[string]any{
		"aps": map[string]any{
			"timestamp":       time.Now().Unix(),
			"event":           "start",
			"attributes-type": "FalconMatchAttributes",
			"attributes":      map[string]any{},
			"content-state": map[string]any{
				"score":                result.Score,
				"label":                string(result.Label),
				"lang":                 lang,
				"projectTitle":         result.ProjectTitle,
				"companyName":          result.CompanyName,
				"companyLogoUrl":       result.CompanyLogoURL,
				"totalMatches":         totalMatches,
				"summary":              summary,
				"projectID":            result.ProjectID,
				"cvID":                 result.CVID,
				"skillsMatch":          result.Scores.SkillsMatch,
				"seniorityFit":         result.Scores.SeniorityFit,
				"domainExperience":     result.Scores.DomainExperience,
				"communicationClarity": result.Scores.CommunicationClarity,
				"projectRelevance":     result.Scores.ProjectRelevance,
				"techStackOverlap":     result.Scores.TechStackOverlap,
			},
			"alert": map[string]any{
				"title":    result.ProjectTitle,
				"subtitle": labelTitle(result.Label, lang),
				"body":     summary,
			},
			"sound": "default",
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal liveactivity payload: %w", err)
	}

	// apns2 v0.23 doesn't expose PushTypeLiveActivity as a constant, but the
	// underlying type is just a string alias — the lib sends it verbatim in
	// the apns-push-type header.
	notification := &apns2.Notification{
		DeviceToken: liveActivityToken,
		Topic:       a.bundleID + ".push-type.liveactivity",
		Payload:     raw,
		PushType:    apns2.EPushType("liveactivity"),
		Priority:    apns2.PriorityHigh,
	}

	resp, err := a.client.PushWithContext(ctx, notification)
	if err != nil {
		return fmt.Errorf("apns liveactivity push: %w", err)
	}
	if !resp.Sent() {
		return fmt.Errorf("apns liveactivity rejected: %s (%d)", resp.Reason, resp.StatusCode)
	}

	logrus.Infof("apns liveactivity sent — apns_id=%s device=%s…", resp.ApnsID, liveActivityToken[:8])
	return nil
}

// SendMatchLiveActivityUpdate refreshes an already-running Live Activity with
// new content-state. The device token here is the per-activity UPDATE token
// iOS handed out when the activity started (different from the pushToStart
// token in SendMatchLiveActivityStart).
//
// Returns an error if the activity has ended (410 Gone from APNs) — callers
// should clear the stored update token and fall back to SendMatchLiveActivityStart.
func (a *apnsClient) SendMatchLiveActivityUpdate(ctx context.Context, updateToken string, result *models.MatchResultEvent, lang string, totalMatches int) error {
	summary := result.Summary[lang]
	if summary == "" {
		summary = result.Summary["de"]
	}

	body := map[string]any{
		"aps": map[string]any{
			"timestamp": time.Now().Unix(),
			"event":     "update",
			"content-state": map[string]any{
				"score":                result.Score,
				"label":                string(result.Label),
				"lang":                 lang,
				"projectTitle":         result.ProjectTitle,
				"companyName":          result.CompanyName,
				"companyLogoUrl":       result.CompanyLogoURL,
				"totalMatches":         totalMatches,
				"summary":              summary,
				"projectID":            result.ProjectID,
				"cvID":                 result.CVID,
				"skillsMatch":          result.Scores.SkillsMatch,
				"seniorityFit":         result.Scores.SeniorityFit,
				"domainExperience":     result.Scores.DomainExperience,
				"communicationClarity": result.Scores.CommunicationClarity,
				"projectRelevance":     result.Scores.ProjectRelevance,
				"techStackOverlap":     result.Scores.TechStackOverlap,
			},
			"alert": map[string]any{
				"title":    result.ProjectTitle,
				"subtitle": labelTitle(result.Label, lang),
				"body":     summary,
			},
			"sound": "default",
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal liveactivity update payload: %w", err)
	}

	notification := &apns2.Notification{
		DeviceToken: updateToken,
		Topic:       a.bundleID + ".push-type.liveactivity",
		Payload:     raw,
		PushType:    apns2.EPushType("liveactivity"),
		Priority:    apns2.PriorityHigh,
	}

	resp, err := a.client.PushWithContext(ctx, notification)
	if err != nil {
		return fmt.Errorf("apns liveactivity update push: %w", err)
	}
	if !resp.Sent() {
		return fmt.Errorf("apns liveactivity update rejected: %s (%d)", resp.Reason, resp.StatusCode)
	}

	logrus.Infof("apns liveactivity UPDATE sent — apns_id=%s update_token=%s…", resp.ApnsID, updateToken[:8])
	return nil
}

// SendAdminAlert delivers a high-severity operational alert to an admin device.
// Used by AdminNotifier when a service publishes an AdminAlertEvent. The
// payload carries enough lightweight context (subject_id, subject_kind, source,
// platform, name, severity) for the iOS app to deep-link straight into the
// offending record from the alert. The HTML snapshot is intentionally NOT
// included — it stays in MongoDB and is fetched on demand from the iOS app or
// from a Studio 3T session via the subject_id.
func (a *apnsClient) SendAdminAlert(ctx context.Context, deviceToken string, subject adminAlertSubject) error {
	title := subject.Name
	if subject.Platform != "" {
		title = subject.Platform + " — " + subject.Name
	}

	p := payload.NewPayload().
		AlertTitle(title).
		AlertSubtitle(strings.ToUpper(subject.Priority)).
		AlertBody(subject.Message).
		Sound("default").
		Category("ADMIN_ALERT").
		Custom("subject_id", subject.ID).
		Custom("subject_kind", string(subject.Kind)).
		Custom("source", subject.Source).
		Custom("platform", subject.Platform).
		Custom("name", subject.Name).
		Custom("severity", subject.Priority)

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

// labelTitle returns the localized APNs subtitle string for the given match
// label. Used as the notification's second line above the body.
func labelTitle(label models.MatchLabel, lang string) string {
	switch lang {
	case "en":
		switch label {
		case models.MatchLabelApplyImmediately:
			return "Apply immediately!"
		case models.MatchLabelTopCandidate:
			return "Top candidate"
		case models.MatchLabelAcceptable:
			return "Acceptable match"
		default:
			return "New match"
		}
	case "es":
		switch label {
		case models.MatchLabelApplyImmediately:
			return "¡Aplica ya!"
		case models.MatchLabelTopCandidate:
			return "Top candidato"
		case models.MatchLabelAcceptable:
			return "Coincidencia aceptable"
		default:
			return "Nueva coincidencia"
		}
	default: // "de" and anything unknown → German (authoritative source)
		switch label {
		case models.MatchLabelApplyImmediately:
			return "Jetzt bewerben!"
		case models.MatchLabelTopCandidate:
			return "Starker Kandidat"
		case models.MatchLabelAcceptable:
			return "Akzeptabler Treffer"
		default:
			return "Neuer Treffer"
		}
	}
}
