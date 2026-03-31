package protocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FreezeViolation represents a detected modification to frozen sections of the manifest.
type FreezeViolation struct {
	Section string `json:"section"` // "interface_contracts" | "scaffolds"
	Message string `json:"message"`
}

// FreezeData holds the data returned by a successful SetFreezeTimestamp call.
type FreezeData struct {
	FreezeTimestamp time.Time
	ContractsFrozen bool
}

// SetFreezeTimestamp sets the worktrees_created_at timestamp on a manifest
// and computes hash checksums for interface contracts and scaffolds to detect
// future modifications.
func SetFreezeTimestamp(ctx context.Context, m *IMPLManifest, t time.Time) result.Result[FreezeData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[FreezeData]([]result.SAWError{
			result.NewFatal("FREEZE_CANCELLED", err.Error()),
		})
	}
	m.WorktreesCreatedAt = &t

	// Compute and store hash of interface contracts
	contractsHash, err := computeHash(m.InterfaceContracts)
	if err != nil {
		return result.NewFailure[FreezeData]([]result.SAWError{
			result.NewFatal("FREEZE_FAILED", fmt.Sprintf("failed to compute contracts hash: %v", err)),
		})
	}
	m.FrozenContractsHash = contractsHash

	// Compute and store hash of scaffolds
	scaffoldsHash, err := computeHash(m.Scaffolds)
	if err != nil {
		return result.NewFailure[FreezeData]([]result.SAWError{
			result.NewFatal("FREEZE_FAILED", fmt.Sprintf("failed to compute scaffolds hash: %v", err)),
		})
	}
	m.FrozenScaffoldsHash = scaffoldsHash

	return result.NewSuccess(FreezeData{
		FreezeTimestamp: t,
		ContractsFrozen: true,
	})
}

// CheckFreeze checks if the manifest has been modified after worktree creation.
// Returns violations if interface contracts or scaffolds were edited post-freeze.
// Returns empty slice if no freeze timestamp is set (backward compatible).
func CheckFreeze(ctx context.Context, manifest *IMPLManifest) ([]FreezeViolation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// No freeze timestamp means no freeze enforcement (backward compatible)
	if manifest.WorktreesCreatedAt == nil {
		return nil, nil
	}

	var violations []FreezeViolation

	// Check interface contracts
	if manifest.FrozenContractsHash != "" {
		currentHash, err := computeHash(manifest.InterfaceContracts)
		if err != nil {
			return nil, fmt.Errorf("failed to compute current contracts hash: %w", err)
		}
		if currentHash != manifest.FrozenContractsHash {
			violations = append(violations, FreezeViolation{
				Section: "interface_contracts",
				Message: fmt.Sprintf("Interface contracts have been modified after freeze at %s. Expected hash %s, got %s",
					manifest.WorktreesCreatedAt.Format(time.RFC3339),
					manifest.FrozenContractsHash[:8],
					currentHash[:8]),
			})
		}
	}

	// Check scaffolds
	if manifest.FrozenScaffoldsHash != "" {
		currentHash, err := computeHash(manifest.Scaffolds)
		if err != nil {
			return nil, fmt.Errorf("failed to compute current scaffolds hash: %w", err)
		}
		if currentHash != manifest.FrozenScaffoldsHash {
			violations = append(violations, FreezeViolation{
				Section: "scaffolds",
				Message: fmt.Sprintf("Scaffolds have been modified after freeze at %s. Expected hash %s, got %s",
					manifest.WorktreesCreatedAt.Format(time.RFC3339),
					manifest.FrozenScaffoldsHash[:8],
					currentHash[:8]),
			})
		}
	}

	return violations, nil
}

// computeHash returns the SHA256 hash of the JSON serialization of the given data.
// This provides a deterministic fingerprint for detecting changes.
func computeHash(data interface{}) (string, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:]), nil
}
