// Schema overlay loader for the enterprise overlay
// (transports/config.schema.enterprise.json).
//
// Constitution Principle XI rule 3 — schema overlay, not patch.
// Upstream's transports/config.schema.json is touched only by a single
// `$ref` anchor pointing here. The merge happens at config-load time.
//
// This is a sibling file: nothing in upstream's lib/*.go is modified.

package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnterpriseSchemaFilename is the basename of the overlay file. The full
// path is resolved relative to the upstream schema's directory.
const EnterpriseSchemaFilename = "config.schema.enterprise.json"

// LoadEnterpriseOverlay reads the enterprise schema overlay from disk.
// Returns (nil, nil) when the file does not exist — this is a normal OSS
// deployment where no enterprise features have been enabled.
//
// The returned map can be merged into a parsed upstream schema's `$defs`
// or referenced via `$ref` from the upstream schema (which is the pattern
// chosen — see config.schema.json's `enterprise` property).
func LoadEnterpriseOverlay(upstreamSchemaPath string) (map[string]any, error) {
	overlayPath := filepath.Join(filepath.Dir(upstreamSchemaPath), EnterpriseSchemaFilename)

	data, err := os.ReadFile(overlayPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read enterprise overlay %s: %w", overlayPath, err)
	}

	var overlay map[string]any
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("parse enterprise overlay %s: %w", overlayPath, err)
	}
	return overlay, nil
}

// MergeEnterpriseIntoSchema composes the enterprise overlay into the
// parsed upstream schema by attaching it under `$defs.enterprise_overlay`
// and updating the `enterprise` property's `$ref` to point inside the
// document. After this call, validators receiving the merged schema
// will resolve the enterprise overlay without needing the file system.
//
// The upstream schema's `enterprise` field already has:
//
//	"enterprise": { "$ref": "config.schema.enterprise.json" }
//
// MergeEnterpriseIntoSchema rewrites that ref to a JSON-pointer:
//
//	"enterprise": { "$ref": "#/$defs/enterprise_overlay" }
//
// and inserts the overlay body at the new location. If the upstream
// schema has no `enterprise` property (e.g., during the brief window
// before T007 lands), this function is a no-op.
func MergeEnterpriseIntoSchema(upstream map[string]any, overlay map[string]any) {
	if overlay == nil {
		return
	}
	defs, ok := upstream["$defs"].(map[string]any)
	if !ok {
		defs = make(map[string]any)
		upstream["$defs"] = defs
	}
	defs["enterprise_overlay"] = overlay

	props, ok := upstream["properties"].(map[string]any)
	if !ok {
		return
	}
	ent, ok := props["enterprise"].(map[string]any)
	if !ok {
		return
	}
	ent["$ref"] = "#/$defs/enterprise_overlay"
}
