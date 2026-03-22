package orchestrator

// setters.go previously contained SetParseIMPLDocFunc which injected the IMPL
// doc parser. Now that orchestrator.New() calls protocol.Load() directly, this
// setter is no longer needed. The file is retained for backward compatibility
// of the package structure.
