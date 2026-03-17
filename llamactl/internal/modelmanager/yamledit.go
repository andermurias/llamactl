package modelmanager

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ── Public YAML operations ────────────────────────────────────────────────────

// AddModelToConfig inserts a model into llama-swap.yaml:
//   - adds the model entry under models:
//   - adds the model ID to groups.<group>.members (if group is non-empty)
//
// Comments in the file are preserved via yaml.v3 Node round-trip.
func AddModelToConfig(configFile, modelID, group string, config ModelConfig) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("empty config document")
	}
	root := doc.Content[0]

	// Add to models: section
	if err := nodeAddModel(root, modelID, config); err != nil {
		return fmt.Errorf("add model entry: %w", err)
	}

	// Add to groups.<group>.members
	if group != "" {
		if err := nodeAddGroupMember(root, group, modelID); err != nil {
			// Non-fatal: group may not exist yet; log but continue
			_ = err
		}
	}

	return writeYAML(configFile, &doc)
}

// RemoveModelFromConfig removes a model from llama-swap.yaml:
//   - removes the entry from models:
//   - removes the model ID from all groups
//
// Returns the removed ModelConfig and its group so it can be stored in the
// disabled store for later re-enabling. Returns nil if the model was not found.
func RemoveModelFromConfig(configFile, modelID string) (*ModelConfig, string, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, "", fmt.Errorf("read config: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, "", fmt.Errorf("parse config: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, "", fmt.Errorf("empty config document")
	}
	root := doc.Content[0]

	cfg, err := nodeExtractModel(root, modelID)
	if err != nil {
		return nil, "", err
	}

	group, _ := nodeRemoveGroupMember(root, modelID)

	return cfg, group, writeYAML(configFile, &doc)
}

// ModelExistsInConfig reports whether a model ID is present in models:.
func ModelExistsInConfig(configFile, modelID string) (bool, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	if len(doc.Content) == 0 {
		return false, nil
	}
	modelsNode := findMappingKey(doc.Content[0], "models")
	if modelsNode == nil {
		return false, nil
	}
	return findMappingKey(modelsNode, modelID) != nil, nil
}

// ── Disabled model store ──────────────────────────────────────────────────────

// LoadDisabledStore reads ~/AI/llamactl-disabled.yaml.
// Returns an empty store if the file does not exist yet.
func LoadDisabledStore(path string) (*DisabledStore, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &DisabledStore{Disabled: map[string]DisabledEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s DisabledStore
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse disabled store: %w", err)
	}
	if s.Disabled == nil {
		s.Disabled = map[string]DisabledEntry{}
	}
	return &s, nil
}

// SaveDisabledStore writes the store back to disk.
func SaveDisabledStore(path string, s *DisabledStore) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ── Node manipulation helpers ─────────────────────────────────────────────────

// nodeAddModel appends a model key+value under the `models:` mapping node.
func nodeAddModel(root *yaml.Node, modelID string, config ModelConfig) error {
	modelsNode := findMappingKey(root, "models")
	if modelsNode == nil {
		return fmt.Errorf("no 'models:' section in config")
	}
	if modelsNode.Kind != yaml.MappingNode {
		return fmt.Errorf("'models:' is not a mapping")
	}

	// Check if key already exists
	for i := 0; i < len(modelsNode.Content)-1; i += 2 {
		if modelsNode.Content[i].Value == modelID {
			return fmt.Errorf("model %q already exists in config", modelID)
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: modelID, Tag: "!!str"}
	valNode := buildModelValueNode(config)

	modelsNode.Content = append(modelsNode.Content, keyNode, valNode)
	return nil
}

// nodeExtractModel finds and removes a model from models:, returning its config.
func nodeExtractModel(root *yaml.Node, modelID string) (*ModelConfig, error) {
	modelsNode := findMappingKey(root, "models")
	if modelsNode == nil {
		return nil, fmt.Errorf("no 'models:' section in config")
	}

	for i := 0; i < len(modelsNode.Content)-1; i += 2 {
		if modelsNode.Content[i].Value == modelID {
			valNode := modelsNode.Content[i+1]

			// Decode the value node into a ModelConfig
			var cfg ModelConfig
			if err := valNode.Decode(&cfg); err != nil {
				return nil, fmt.Errorf("decode model config: %w", err)
			}

			// Remove the key+value pair
			modelsNode.Content = append(
				modelsNode.Content[:i],
				modelsNode.Content[i+2:]...,
			)
			return &cfg, nil
		}
	}
	return nil, fmt.Errorf("model %q not found in config", modelID)
}

// nodeAddGroupMember appends modelID to groups.<group>.members sequence.
func nodeAddGroupMember(root *yaml.Node, group, modelID string) error {
	groupsNode := findMappingKey(root, "groups")
	if groupsNode == nil {
		return fmt.Errorf("no 'groups:' section")
	}
	groupNode := findMappingKey(groupsNode, group)
	if groupNode == nil {
		return fmt.Errorf("group %q not found", group)
	}
	membersNode := findMappingKey(groupNode, "members")
	if membersNode == nil {
		return fmt.Errorf("no members in group %q", group)
	}
	if membersNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("members of group %q is not a sequence", group)
	}

	// Check for duplicates
	for _, n := range membersNode.Content {
		if n.Value == modelID {
			return nil // already there
		}
	}

	membersNode.Content = append(membersNode.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: modelID,
		Tag:   "!!str",
	})
	return nil
}

// nodeRemoveGroupMember removes modelID from all groups, returning the first group found.
func nodeRemoveGroupMember(root *yaml.Node, modelID string) (string, bool) {
	groupsNode := findMappingKey(root, "groups")
	if groupsNode == nil {
		return "", false
	}

	// Iterate over all groups
	for i := 0; i < len(groupsNode.Content)-1; i += 2 {
		groupName := groupsNode.Content[i].Value
		groupNode := groupsNode.Content[i+1]
		membersNode := findMappingKey(groupNode, "members")
		if membersNode == nil || membersNode.Kind != yaml.SequenceNode {
			continue
		}
		for j, m := range membersNode.Content {
			if m.Value == modelID {
				membersNode.Content = append(
					membersNode.Content[:j],
					membersNode.Content[j+1:]...,
				)
				return groupName, true
			}
		}
	}
	return "", false
}

// findMappingKey finds the value node for a key in a YAML mapping node.
// Returns nil if the key is not found.
func findMappingKey(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// buildModelValueNode creates a yaml.Node tree for a ModelConfig.
func buildModelValueNode(config ModelConfig) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// cmd — always present, use literal block scalar (|) for multiline
	cmdNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: config.Cmd}
	if strings.Contains(config.Cmd, "\n") {
		cmdNode.Style = yaml.LiteralStyle
	}
	node.Content = append(node.Content,
		scalarNode("cmd"), cmdNode,
	)

	// useModelName
	if config.UseModelName != "" {
		node.Content = append(node.Content,
			scalarNode("useModelName"),
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str",
				Value: config.UseModelName, Style: yaml.DoubleQuotedStyle},
		)
	}

	// checkEndpoint
	if config.CheckEndpoint != "" {
		node.Content = append(node.Content,
			scalarNode("checkEndpoint"),
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str",
				Value: config.CheckEndpoint, Style: yaml.DoubleQuotedStyle},
		)
	}

	// ttl
	node.Content = append(node.Content,
		scalarNode("ttl"),
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int",
			Value: fmt.Sprintf("%d", config.TTL)},
	)

	return node
}

// scalarNode creates a plain string scalar node (for map keys).
func scalarNode(val string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val}
}

// writeYAML marshals a yaml.Node document back to a file.
func writeYAML(path string, doc *yaml.Node) error {
	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	// yaml.Marshal of a DocumentNode adds "---\n" prefix; keep it if original had it
	return os.WriteFile(path, out, 0o644)
}
