package sapient

import (
	"encoding/json"
	"strings"
)

// CardMeta is the generic, modality-agnostic projection of an Agent Card's
// capability extension and commerce posture, parsed straight from the card
// JSON (the EIP-8004 agentURI). It exists for UI/verifier layers that must
// render ANY seller's card — neuron.rid/1, neuron.adsb/1, or a future
// neuron.<x>/<n> — without baking in one modality (the display and the
// explorer both consume it).
type CardMeta struct {
	// ExtensionID is the capability-extension key found in the stdOut topic
	// service config (e.g. "neuron.adsb/1"). Empty when the card carries none.
	ExtensionID string

	// Modality is the segment between "neuron." and "/" of ExtensionID
	// ("rid", "adsb", …). Empty when ExtensionID is empty.
	Modality string

	// Capabilities is the extension's optional closed capability vocabulary.
	Capabilities []string

	// SensorModels is the extension's advertised sensor family.
	SensorModels []string

	// Wire is the extension's on-the-wire format label.
	Wire string

	// Schema / SchemaSha256 reference the extension's wire schema.
	Schema       string
	SchemaSha256 string

	// NodeID is the extension's identity-bound SAPIENT node_id.
	NodeID string

	// CommerceService and CommerceBinding describe the first neuron-commerce
	// service: its name and settlement binding ("none" = advertisement-only).
	CommerceService string
	CommerceBinding string
}

// ParseCardMeta extracts CardMeta from a card's raw JSON. It is deliberately
// tolerant: a malformed or empty card yields a zero CardMeta, never an error —
// UI layers render "—" for absent facts rather than failing.
func ParseCardMeta(agentURI json.RawMessage) CardMeta {
	var meta CardMeta
	var card struct {
		Services []map[string]any `json:"services"`
	}
	if err := json.Unmarshal(agentURI, &card); err != nil {
		return meta
	}
	for _, svc := range card.Services {
		typ, _ := svc["type"].(string)
		switch typ {
		case "neuron-topic":
			// The stdOut topic is identified by channel; older fixtures and
			// renders may carry only the canonical name — accept either.
			ch, _ := svc["channel"].(string)
			name, _ := svc["name"].(string)
			if ch != "stdOut" && name != TopicNameStdOut {
				continue
			}
			cfg, _ := svc["config"].(map[string]any)
			for key, val := range cfg {
				if !strings.HasPrefix(key, "neuron.") {
					continue
				}
				ext, _ := val.(map[string]any)
				meta.ExtensionID = key
				meta.Modality = modalityOfExtension(key)
				meta.Capabilities = stringSlice(ext["capabilities"])
				meta.SensorModels = stringSlice(ext["sensorModels"])
				meta.Wire, _ = ext["wire"].(string)
				meta.Schema, _ = ext["schema"].(string)
				meta.SchemaSha256, _ = ext["schemaSha256"].(string)
				meta.NodeID, _ = ext["nodeId"].(string)
				break
			}
		case "neuron-commerce":
			if meta.CommerceService != "" {
				continue // first commerce service wins
			}
			meta.CommerceService, _ = svc["name"].(string)
			if settlement, ok := svc["settlement"].(map[string]any); ok {
				meta.CommerceBinding, _ = settlement["binding"].(string)
			}
		}
	}
	return meta
}

// modalityOfExtension returns the segment between "neuron." and "/" of an
// extension id ("neuron.adsb/1" → "adsb"). Unknown shapes return the raw id.
func modalityOfExtension(extensionID string) string {
	rest := strings.TrimPrefix(extensionID, "neuron.")
	if rest == extensionID {
		return extensionID
	}
	modality, _, _ := strings.Cut(rest, "/")
	return modality
}

// stringSlice coerces a decoded JSON array into []string, skipping non-strings.
func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
