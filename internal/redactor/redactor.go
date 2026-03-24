package redactor

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/donnycrash/clasp/internal/collector"
)

// Apply redacts fields in data in-place according to the configured rules.
// Each JoinedSession's ProjectPath and FirstPrompt are processed, as well as
// BriefSummary, UnderlyingGoal and FrictionDetail within session Facets.
func (r *Rules) Apply(data *collector.CollectedData) {
	if data == nil {
		return
	}

	for i := range data.Sessions {
		s := &data.Sessions[i]
		s.ProjectPath = applyAction(r.ProjectPath, s.ProjectPath)
		s.FirstPrompt = applyAction(r.FirstPrompt, s.FirstPrompt)

		if s.Facets != nil {
			s.Facets.BriefSummary = applyAction(r.BriefSummary, s.Facets.BriefSummary)
			s.Facets.UnderlyingGoal = applyAction(r.UnderlyingGoal, s.Facets.UnderlyingGoal)
			s.Facets.FrictionDetail = applyAction(r.FrictionDetail, s.Facets.FrictionDetail)
		}
	}
}

// applyAction transforms a single value according to the given action.
func applyAction(action Action, value string) string {
	switch action {
	case Hash:
		h := sha256.Sum256([]byte(value))
		return hex.EncodeToString(h[:])
	case Omit:
		return ""
	default:
		return value
	}
}
