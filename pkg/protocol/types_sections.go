package protocol

// PostMergeChecklist represents the structured post-merge verification checklist.
// Scout outputs this as a typed YAML block; parser unmarshals into this struct.
type PostMergeChecklist struct {
	Groups []ChecklistGroup `yaml:"groups" json:"groups"`
}

// ChecklistGroup is a logical grouping of related checklist items.
type ChecklistGroup struct {
	Title string          `yaml:"title" json:"title"`
	Items []ChecklistItem `yaml:"items" json:"items"`
}

// ChecklistItem is a single verification step in the post-merge checklist.
type ChecklistItem struct {
	Description string `yaml:"description" json:"description"`
	Command     string `yaml:"command,omitempty" json:"command,omitempty"`
}
