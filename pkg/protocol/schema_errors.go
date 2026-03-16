package protocol

// Schema validation error code constants (SV01 prefix).
// These are used by the schema validation layer to report structural issues
// in IMPL manifests (required fields, enum values, path format, unknown keys,
// and cross-field consistency).
const (
	SV01RequiredField   = "SV01_REQUIRED_FIELD"
	SV01InvalidEnum     = "SV01_INVALID_ENUM"
	SV01InvalidPath     = "SV01_INVALID_PATH"
	SV01UnknownKey      = "SV01_UNKNOWN_KEY"
	SV01CrossFieldError = "SV01_CROSS_FIELD"
)
