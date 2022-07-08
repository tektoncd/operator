package config

import (
	"errors"

	"gopkg.in/yaml.v3"
)

// This type implements a low-level get/set config that is backed by an in-memory tree of yaml
// nodes. It allows us to interact with a yaml-based config programmatically, preserving any
// comments that were present when the yaml was parsed.
type configMap struct {
	Root *yaml.Node
}

type configEntry struct {
	KeyNode   *yaml.Node
	ValueNode *yaml.Node
	Index     int
}

type NotFoundError struct {
	error
}

func (cm *configMap) empty() bool {
	return cm.Root == nil || len(cm.Root.Content) == 0
}

func (cm *configMap) getStringValue(key string) (string, error) {
	entry, err := cm.findEntry(key)
	if err != nil {
		return "", err
	}
	return entry.ValueNode.Value, nil
}

func (cm *configMap) setStringValue(key, value string) error {
	entry, err := cm.findEntry(key)
	if err == nil {
		entry.ValueNode.Value = value
		return nil
	}

	var notFound *NotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return err
	}

	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: key,
	}
	valueNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: value,
	}

	cm.Root.Content = append(cm.Root.Content, keyNode, valueNode)
	return nil
}

func (cm *configMap) findEntry(key string) (*configEntry, error) {
	if cm.empty() {
		return nil, &NotFoundError{errors.New("not found")}
	}

	ce := &configEntry{}

	// Content slice goes [key1, value1, key2, value2, ...].
	topLevelPairs := cm.Root.Content
	for i, v := range topLevelPairs {
		// Skip every other slice item since we only want to check against keys.
		if i%2 != 0 {
			continue
		}
		if v.Value == key {
			ce.KeyNode = v
			ce.Index = i
			if i+1 < len(topLevelPairs) {
				ce.ValueNode = topLevelPairs[i+1]
			}
			return ce, nil
		}
	}

	return nil, &NotFoundError{errors.New("not found")}
}

func (cm *configMap) removeEntry(key string) {
	if cm.empty() {
		return
	}

	newContent := []*yaml.Node{}

	var skipNext bool
	for i, v := range cm.Root.Content {
		if skipNext {
			skipNext = false
			continue
		}
		if i%2 != 0 || v.Value != key {
			newContent = append(newContent, v)
		} else {
			// Don't append current node and skip the next which is this key's value.
			skipNext = true
		}
	}

	cm.Root.Content = newContent
}

func (cm *configMap) keys() []string {
	keys := []string{}
	if cm.empty() {
		return keys
	}

	// Content slice goes [key1, value1, key2, value2, ...].
	for i, v := range cm.Root.Content {
		// Skip every other slice item since we only want keys.
		if i%2 != 0 {
			continue
		}
		keys = append(keys, v.Value)
	}

	return keys
}
