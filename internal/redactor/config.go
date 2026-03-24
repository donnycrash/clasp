package redactor

// Action defines how a field should be redacted.
type Action string

const (
	// Keep leaves the field value unchanged.
	Keep Action = "keep"
	// Hash replaces the field value with its SHA-256 hex digest.
	Hash Action = "hash"
	// Omit replaces the field value with an empty string.
	Omit Action = "omit"
)

// RedactionConfig holds string-based redaction settings typically read from
// a user configuration file. Each field value should be "keep", "hash", or
// "omit". Empty strings are treated as the default action.
type RedactionConfig struct {
	ProjectPath    string
	FirstPrompt    string
	BriefSummary   string
	UnderlyingGoal string
	FrictionDetail string
}

// Rules holds the parsed redaction actions for each sensitive field.
type Rules struct {
	ProjectPath    Action
	FirstPrompt    Action
	BriefSummary   Action
	UnderlyingGoal Action
	FrictionDetail Action
}

// DefaultRules returns the default redaction rules. By default, all sensitive
// fields are either hashed or omitted to minimise data exposure.
func DefaultRules() Rules {
	return Rules{
		ProjectPath:    Hash,
		FirstPrompt:    Omit,
		BriefSummary:   Omit,
		UnderlyingGoal: Omit,
		FrictionDetail: Omit,
	}
}

// RulesFromConfig builds Rules from a RedactionConfig. Any field left empty
// in the config falls back to the corresponding default action.
func RulesFromConfig(cfg RedactionConfig) Rules {
	defaults := DefaultRules()
	return Rules{
		ProjectPath:    parseAction(cfg.ProjectPath, defaults.ProjectPath),
		FirstPrompt:    parseAction(cfg.FirstPrompt, defaults.FirstPrompt),
		BriefSummary:   parseAction(cfg.BriefSummary, defaults.BriefSummary),
		UnderlyingGoal: parseAction(cfg.UnderlyingGoal, defaults.UnderlyingGoal),
		FrictionDetail: parseAction(cfg.FrictionDetail, defaults.FrictionDetail),
	}
}

// parseAction converts a string to an Action. If the string is empty or
// unrecognised, the provided fallback is returned.
func parseAction(s string, fallback Action) Action {
	switch Action(s) {
	case Keep, Hash, Omit:
		return Action(s)
	default:
		return fallback
	}
}
