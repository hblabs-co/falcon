package project

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/models"
)

// handleMatchPending normalizes the project referenced by a pre-confirmed match.
// Prioritized over project.created because a match.result push may land imminently
// and the user will expect to see the normalized project when they open it.
func (s *Service) handleMatchPending(data []byte) error {
	var event models.MatchPendingEvent
	if err := json.Unmarshal(data, &event); err != nil {
		logrus.Errorf("unmarshal match.pending: %v (dropping)", err)
		return nil
	}
	return s.normalizeByProjectID(context.Background(), event.ProjectID)
}
