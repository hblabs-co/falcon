package cv

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/llm"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Service normalizes CVs via LLM.
type Service struct {
	llm             *llm.Client
	normalizePrompt string
}

// NewService creates the CV normalizer.
func NewService(llmClient *llm.Client, normalizePrompt string) *Service {
	return &Service{llm: llmClient, normalizePrompt: normalizePrompt}
}

// Register subscribes to cv.indexed events.
func (s *Service) Register(ctx context.Context) error {
	if err := system.Subscribe(ctx, constants.StreamStorage, "normalizer-cv-indexed", constants.SubjectCVIndexed, s.handleCVIndexed); err != nil {
		return fmt.Errorf("subscribe cv.indexed: %w", err)
	}
	logrus.Info("[cv] subscribed to cv.indexed")
	return nil
}

func (s *Service) handleCVIndexed(data []byte) error {
	var evt models.CVIndexedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		logrus.Errorf("unmarshal cv.indexed event: %v (dropping)", err)
		return nil
	}

	log := logrus.WithField("cv_id", evt.CVID)
	ctx := context.Background()

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, evt.CVID, &cv); err != nil {
		return fmt.Errorf("fetch cv %s: %w", evt.CVID, err)
	}

	if cv.ExtractedText == "" {
		log.Warn("cv has no extracted text, skipping normalization")
		return nil
	}

	if cv.Status == models.CVStatusNormalized {
		log.Info("cv already normalized, skipping")
		return nil
	}

	log.Info("normalizing cv")

	normalized, rawContent, llmErr := s.normalizeCV(ctx, cv.ExtractedText, map[string]any{
		"cv_id":   cv.ID,
		"user_id": cv.UserID,
	})
	if llmErr != nil {
		log.Errorf("cv normalization failed: %v (raw: %.500s)", llmErr, rawContent)
		system.RecordError(ctx, models.ServiceError{
			ServiceName:   constants.ServiceNormalizer,
			ErrorName:     "cv_normalize_failed",
			Error:         llmErr.Error(),
			CVID:          cv.ID,
			UserID:        cv.UserID,
			RawLLMContent: rawContent,
		})
		if dbErr := system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
			"status":     models.CVStatusFailed,
			"error":      llmErr.Error(),
			"updated_at": time.Now(),
		}); dbErr != nil {
			log.Errorf("failed to persist cv failed status: %v", dbErr)
		}
		return nil
	}

	if err := system.GetStorage().SetById(ctx, constants.MongoCVsCollection, cv.ID, bson.M{
		"normalized": normalized,
		"status":     models.CVStatusNormalized,
		"updated_at": time.Now(),
	}); err != nil {
		return fmt.Errorf("save normalized cv: %w", err)
	}

	if err := system.Publish(ctx, constants.SubjectCVNormalized, models.CVIndexedEvent{
		CVID: cv.ID, UserID: cv.UserID, QdrantID: cv.QdrantID,
	}); err != nil {
		log.Warnf("publish cv.normalized: %v", err)
	}

	log.Infof("cv normalized — de_exp=%d en_exp=%d es_exp=%d",
		len(normalized.De.Experience), len(normalized.En.Experience), len(normalized.Es.Experience))
	return nil
}

func (s *Service) normalizeCV(ctx context.Context, cvText string, logFields map[string]any) (*models.NormalizedCV, string, error) {
	userPrompt := strings.ReplaceAll(
		"Extract and normalize the following CV text according to your instructions.\nRespond ONLY with the JSON object (no language wrapper keys). No markdown, no explanation.\n\n{{cv_text}}",
		"{{cv_text}}", cvText)

	deMap, rawDE, err := s.llm.NormalizeDE(ctx, s.normalizePrompt, userPrompt, "cv")
	if err != nil {
		return nil, rawDE, err
	}

	enMap, esMap, rawTranslate, transErr := s.llm.TranslateToEnEs(ctx, deMap, logFields)
	if transErr != nil {
		return nil, rawTranslate, transErr
	}

	de, err := mapToLang(deMap)
	if err != nil {
		return nil, rawDE, fmt.Errorf("decode de cv: %w", err)
	}
	en, err := mapToLang(enMap)
	if err != nil {
		return nil, rawDE, fmt.Errorf("decode en cv: %w", err)
	}
	es, err := mapToLang(esMap)
	if err != nil {
		return nil, rawDE, fmt.Errorf("decode es cv: %w", err)
	}

	return &models.NormalizedCV{De: *de, En: *en, Es: *es}, rawDE, nil
}

func mapToLang(m map[string]any) (*models.NormalizedCVLang, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var lang models.NormalizedCVLang
	if err := json.Unmarshal(b, &lang); err != nil {
		return nil, err
	}
	return &lang, nil
}
