package queue

// Item represents a queued IMPL feature request.
type Item struct {
	Title              string   `yaml:"title" json:"title"`
	// Priority is the sort order (1 = highest). Must match the numeric prefix
	// of the on-disk filename ({priority:03d}-{slug}.yaml). Always use Manager
	// methods to mutate queue items — direct file writes must keep both in sync.
	Priority int `yaml:"priority" json:"priority"`
	DependsOn          []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	FeatureDescription string   `yaml:"feature_description" json:"feature_description"`
	Status             string   `yaml:"status" json:"status"` // queued | in_progress | complete | blocked
	AutonomyOverride   string   `yaml:"autonomy_override,omitempty" json:"autonomy_override,omitempty"`
	RequireReview      bool     `yaml:"require_review,omitempty" json:"require_review,omitempty"`
	Slug               string   `yaml:"slug,omitempty" json:"slug,omitempty"`
	FilePath           string   `yaml:"-" json:"file_path,omitempty"` // populated at load time
}
