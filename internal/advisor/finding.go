package advisor

// Finding is one problem the model reports through the report_finding tool. It is advice:
// G3 executes an action only after the user confirms it, and quarantine actions still go
// through the executor's checks.
type Finding struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Cause      string   `json:"cause"`
	Paths      []string `json:"paths"`
	Bytes      uint64   `json:"bytes"`
	Risk       string   `json:"risk"`
	Confidence string   `json:"confidence"`
	Regrowth   string   `json:"regrowth"`
	Evidence   []string `json:"evidence"`
	Actions    []Action `json:"actions"`
}

// Action kinds. Only quarantine is reversible by Pare.
const (
	ActionQuarantine = "quarantine"
	ActionTemplate   = "template"
	ActionCommand    = "command"
	ActionConfig     = "config"
	ActionManual     = "manual"
)

type Action struct {
	Kind     string            `json:"kind"`
	Label    string            `json:"label"`
	Explain  string            `json:"explain"`
	Bytes    uint64            `json:"bytes"`
	Paths    []string          `json:"paths,omitempty"`
	Template string            `json:"template,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
	Command  string            `json:"command,omitempty"`
}

var riskLevels = map[string]bool{"safe": true, "rebuildable": true, "careful": true, "system": true}
var confidenceLevels = map[string]bool{"high": true, "medium": true, "low": true}

// normalize fills defaults so the UI never has to guess; it does not judge the advice.
func (f *Finding) normalize() {
	if !riskLevels[f.Risk] {
		f.Risk = "careful"
	}
	if !confidenceLevels[f.Confidence] {
		f.Confidence = "low"
	}
	valid := f.Actions[:0]
	for _, action := range f.Actions {
		switch action.Kind {
		case ActionQuarantine, ActionTemplate, ActionCommand, ActionConfig, ActionManual:
			valid = append(valid, action)
		}
	}
	f.Actions = valid
	if f.Paths == nil {
		f.Paths = []string{}
	}
	if f.Evidence == nil {
		f.Evidence = []string{}
	}
}
