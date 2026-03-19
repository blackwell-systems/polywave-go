package protocol

// ErrorCode is a typed, stable identifier for a validation rule violation.
// It serializes cleanly to JSON as a string (e.g. "E001_MISSING_TITLE").
type ErrorCode string

// All error code constants. Codes use a numeric E-prefix (E001–E019+)
// that is distinct from the human-readable rule names used in validation.go
// (I1_, SV01_, etc.). The old rule names are preserved as MigrateErrorCode
// aliases for backward compatibility.
const (
	// Field-level required-value errors (category: "field")
	ErrMissingTitle      ErrorCode = "E001_MISSING_TITLE"
	ErrMissingSlug       ErrorCode = "E002_MISSING_SLUG"
	ErrInvalidVerdict    ErrorCode = "E003_INVALID_VERDICT"

	// Agent / gate configuration errors (category: "field")
	ErrInvalidAgentID  ErrorCode = "E004_INVALID_AGENT_ID"
	ErrInvalidGateType ErrorCode = "E011_INVALID_GATE_TYPE"

	// Invariant errors (category: "invariant")
	ErrI1Violation   ErrorCode = "E005_I1_DISJOINT_OWNERSHIP"
	ErrI2MissingDep  ErrorCode = "E006_I2_MISSING_DEP"
	ErrI2WaveOrder   ErrorCode = "E006_I2_WAVE_ORDER"   // same numeric group as ErrI2MissingDep
	ErrI3WaveSequence ErrorCode = "E007_I3_WAVE_SEQUENCE"
	ErrI4MissingField ErrorCode = "E015_I4_MISSING_FIELD" // overlaps schema; kept in invariant
	ErrI4InvalidValue ErrorCode = "E016_I4_INVALID_VALUE"
	ErrI5OrphanFile   ErrorCode = "E008_I5_ORPHAN_FILE"
	ErrI5Uncommitted  ErrorCode = "E009_I5_UNCOMMITTED"
	ErrI6Cycle        ErrorCode = "E010_I6_CYCLE"

	// State-machine errors (category: "state")
	ErrInvalidMergeState ErrorCode = "E012_INVALID_MERGE_STATE"
	ErrInvalidState      ErrorCode = "E013_INVALID_STATE"

	// Multi-repo consistency (category: "invariant")
	ErrMultiRepoMixed ErrorCode = "E014_MULTI_REPO_MIXED"

	// Schema-validation errors (category: "schema")
	ErrSVRequiredField ErrorCode = "E015_SV_REQUIRED_FIELD"
	ErrSVInvalidEnum   ErrorCode = "E016_SV_INVALID_ENUM"
	ErrSVInvalidPath   ErrorCode = "E017_SV_INVALID_PATH"
	ErrSVUnknownKey    ErrorCode = "E018_SV_UNKNOWN_KEY"
	ErrSVCrossField    ErrorCode = "E019_SV_CROSS_FIELD"
)

// ErrorCodeInfo provides stable metadata for a given ErrorCode.
type ErrorCodeInfo struct {
	// Code is the canonical typed constant.
	Code ErrorCode

	// Category groups related codes:
	//   "invariant" – I1–I6 ownership / dependency rules
	//   "schema"    – SV01 structural / field-format rules
	//   "state"     – protocol state-machine rules
	//   "field"     – individual field value rules
	Category string

	// Severity indicates how the finding should be treated:
	//   "error"   – must be fixed before proceeding
	//   "warning" – advisory; does not block the workflow
	Severity string

	// Description is a short human-readable summary of what the rule checks.
	Description string
}

// errorCodeRegistry is the authoritative source of truth for all registered codes.
var errorCodeRegistry = map[ErrorCode]ErrorCodeInfo{
	ErrMissingTitle: {
		Code:        ErrMissingTitle,
		Category:    "field",
		Severity:    "error",
		Description: "The manifest title field is missing or empty.",
	},
	ErrMissingSlug: {
		Code:        ErrMissingSlug,
		Category:    "field",
		Severity:    "error",
		Description: "The manifest feature_slug field is missing or empty.",
	},
	ErrInvalidVerdict: {
		Code:        ErrInvalidVerdict,
		Category:    "field",
		Severity:    "error",
		Description: "The verdict field contains an unrecognised value (must be SUITABLE, NOT_SUITABLE, or SUITABLE_WITH_CAVEATS).",
	},
	ErrInvalidAgentID: {
		Code:        ErrInvalidAgentID,
		Category:    "field",
		Severity:    "error",
		Description: "An agent ID does not match the protocol pattern ^[A-Z][2-9]?$.",
	},
	ErrI1Violation: {
		Code:        ErrI1Violation,
		Category:    "invariant",
		Severity:    "error",
		Description: "I1: Two or more agents claim ownership of the same file in the same wave.",
	},
	ErrI2MissingDep: {
		Code:        ErrI2MissingDep,
		Category:    "invariant",
		Severity:    "error",
		Description: "I2: An agent dependency references an agent that does not exist in the manifest.",
	},
	ErrI2WaveOrder: {
		Code:        ErrI2WaveOrder,
		Category:    "invariant",
		Severity:    "error",
		Description: "I2: An agent depends on another agent in the same wave or a later wave.",
	},
	ErrI3WaveSequence: {
		Code:        ErrI3WaveSequence,
		Category:    "invariant",
		Severity:    "error",
		Description: "I3: Wave numbers are not sequential starting from 1.",
	},
	ErrI4MissingField: {
		Code:        ErrI4MissingField,
		Category:    "invariant",
		Severity:    "error",
		Description: "I4: A required manifest field is absent or empty.",
	},
	ErrI4InvalidValue: {
		Code:        ErrI4InvalidValue,
		Category:    "invariant",
		Severity:    "error",
		Description: "I4: A manifest field contains an invalid value.",
	},
	ErrI5OrphanFile: {
		Code:        ErrI5OrphanFile,
		Category:    "invariant",
		Severity:    "error",
		Description: "I5: An agent references a file that has no entry in the file_ownership table.",
	},
	ErrI5Uncommitted: {
		Code:        ErrI5Uncommitted,
		Category:    "invariant",
		Severity:    "error",
		Description: "I5: A completion report lacks a valid commit hash — agents must commit before reporting.",
	},
	ErrI6Cycle: {
		Code:        ErrI6Cycle,
		Category:    "invariant",
		Severity:    "error",
		Description: "I6: A cycle was detected in the agent dependency graph.",
	},
	ErrInvalidGateType: {
		Code:        ErrInvalidGateType,
		Category:    "field",
		Severity:    "error",
		Description: "A quality gate type is not one of: build, lint, test, typecheck, custom.",
	},
	ErrInvalidMergeState: {
		Code:        ErrInvalidMergeState,
		Category:    "state",
		Severity:    "error",
		Description: "The merge_state field contains an unrecognised value.",
	},
	ErrInvalidState: {
		Code:        ErrInvalidState,
		Category:    "state",
		Severity:    "error",
		Description: "The state field contains an unrecognised ProtocolState value.",
	},
	ErrMultiRepoMixed: {
		Code:        ErrMultiRepoMixed,
		Category:    "invariant",
		Severity:    "error",
		Description: "Some file_ownership entries have an explicit repo: field and others do not; all entries must be consistent.",
	},
	ErrSVRequiredField: {
		Code:        ErrSVRequiredField,
		Category:    "schema",
		Severity:    "error",
		Description: "A required field is missing according to schema validation (SV01_REQUIRED_FIELD).",
	},
	ErrSVInvalidEnum: {
		Code:        ErrSVInvalidEnum,
		Category:    "schema",
		Severity:    "error",
		Description: "A field value is not one of the allowed enumeration values (SV01_INVALID_ENUM).",
	},
	ErrSVInvalidPath: {
		Code:        ErrSVInvalidPath,
		Category:    "schema",
		Severity:    "error",
		Description: "A path field does not pass format validation (SV01_INVALID_PATH).",
	},
	ErrSVUnknownKey: {
		Code:        ErrSVUnknownKey,
		Category:    "schema",
		Severity:    "warning",
		Description: "An unrecognised key is present in the manifest (SV01_UNKNOWN_KEY).",
	},
	ErrSVCrossField: {
		Code:        ErrSVCrossField,
		Category:    "schema",
		Severity:    "error",
		Description: "Two or more fields are mutually inconsistent (SV01_CROSS_FIELD).",
	},
}

// LookupErrorCode returns the ErrorCodeInfo for the given code.
// If the code is not registered, it returns a zero-value ErrorCodeInfo
// (all fields empty, including Code).
func LookupErrorCode(code ErrorCode) ErrorCodeInfo {
	return errorCodeRegistry[code]
}

// AllErrorCodes returns a slice of ErrorCodeInfo for every registered code.
// The order is deterministic (sorted by Code string) to support documentation
// and analytics pipelines.
func AllErrorCodes() []ErrorCodeInfo {
	// Build ordered list. We enumerate the canonical constant set so the
	// order stays consistent even if the map iteration order changes.
	ordered := []ErrorCode{
		ErrMissingTitle,
		ErrMissingSlug,
		ErrInvalidVerdict,
		ErrInvalidAgentID,
		ErrI1Violation,
		ErrI2MissingDep,
		ErrI2WaveOrder,
		ErrI3WaveSequence,
		ErrI4MissingField,
		ErrI4InvalidValue,
		ErrI5OrphanFile,
		ErrI5Uncommitted,
		ErrI6Cycle,
		ErrInvalidGateType,
		ErrInvalidMergeState,
		ErrInvalidState,
		ErrMultiRepoMixed,
		ErrSVRequiredField,
		ErrSVInvalidEnum,
		ErrSVInvalidPath,
		ErrSVUnknownKey,
		ErrSVCrossField,
	}

	out := make([]ErrorCodeInfo, 0, len(ordered))
	for _, code := range ordered {
		if info, ok := errorCodeRegistry[code]; ok {
			out = append(out, info)
		}
	}
	return out
}

// migrateTable maps legacy string codes (as used in validation.go and
// schema_errors.go) to their canonical ErrorCode counterparts.
var migrateTable = map[string]ErrorCode{
	// validation.go codes
	"I1_VIOLATION":        ErrI1Violation,
	"I2_MISSING_DEP":      ErrI2MissingDep,
	"I2_WAVE_ORDER":       ErrI2WaveOrder,
	"I3_WAVE_ORDER":       ErrI3WaveSequence,
	"I4_MISSING_FIELD":    ErrI4MissingField,
	"I4_INVALID_VALUE":    ErrI4InvalidValue,
	"I5_ORPHAN_FILE":      ErrI5OrphanFile,
	"I5_UNCOMMITTED":      ErrI5Uncommitted,
	"I6_CYCLE":            ErrI6Cycle,
	"DC04_INVALID_AGENT_ID": ErrInvalidAgentID,
	"DC07_INVALID_GATE_TYPE": ErrInvalidGateType,
	"E9_INVALID_MERGE_STATE": ErrInvalidMergeState,
	"SM01_INVALID_STATE":  ErrInvalidState,
	"MR01_INCONSISTENT_REPO": ErrMultiRepoMixed,

	// schema_errors.go / SV01 codes
	SV01RequiredField:   ErrSVRequiredField,
	SV01InvalidEnum:     ErrSVInvalidEnum,
	SV01InvalidPath:     ErrSVInvalidPath,
	SV01UnknownKey:      ErrSVUnknownKey,
	SV01CrossFieldError: ErrSVCrossField,
}

// MigrateErrorCode translates a legacy string-based error code to the
// canonical typed ErrorCode. Returns an empty ErrorCode ("") if the old
// code has no known mapping.
func MigrateErrorCode(oldCode string) ErrorCode {
	return migrateTable[oldCode]
}
