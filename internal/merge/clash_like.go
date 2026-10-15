package merge

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// MergeClashLike merges two subscription response bodies for Clash, Mihomo and Stash response types.
func MergeClashLike(primary, limited []byte, insertAfter string) ([]byte, error) {
	// Parse both configurations as yaml.Node
	var primaryNode yaml.Node
	if err := yaml.Unmarshal(primary, &primaryNode); err != nil {
		return nil, fmt.Errorf("parse primary config: %w", err)
	}
	var limitedNode yaml.Node
	if err := yaml.Unmarshal(limited, &limitedNode); err != nil {
		return nil, fmt.Errorf("parse limited config: %w", err)
	}

	// Unmarshaling into a Node always produces a DocumentNode with one MappingNode child.
	primaryRoot, err := unwrapDocument(&primaryNode)
	if err != nil {
		return nil, fmt.Errorf("primary config: %w", err)
	}
	limitedRoot, err := unwrapDocument(&limitedNode)
	if err != nil {
		return nil, fmt.Errorf("limited config: %w", err)
	}

	// Extract proxies from the limited config
	limitedProxiesNode := mappingGetValue(limitedRoot, "proxies")
	if limitedProxiesNode == nil {
		return nil, fmt.Errorf("limited config has no 'proxies' key")
	}
	if limitedProxiesNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("limited config 'proxies' is not a sequence")
	}

	// Collect proxy names from the limited config
	proxyNames, err := extractProxyNames(limitedProxiesNode)
	if err != nil {
		return nil, fmt.Errorf("extract proxy names: %w", err)
	}
	if len(proxyNames) == 0 {
		return nil, fmt.Errorf("limited config 'proxies' is empty")
	}
	for i, proxyNode := range limitedProxiesNode.Content {
		nameNode := mappingGetValue(proxyNode, "name")
		nameNode.Value = cleanLimitedHostName(nameNode.Value)
		proxyNames[i] = nameNode.Value
	}

	// Insert proxies from the limited config into the primary config.
	primaryProxiesNode := mappingGetValue(primaryRoot, "proxies")
	if primaryProxiesNode == nil {
		return nil, fmt.Errorf("primary config has no 'proxies' key")
	}
	if primaryProxiesNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("primary config 'proxies' is not a sequence")
	}
	primaryProxyNames := make(map[string]struct{}, len(primaryProxiesNode.Content))
	for _, proxyNode := range primaryProxiesNode.Content {
		primaryProxyNames[clashHostName(proxyNode)] = struct{}{}
	}
	limitedItems := make([]*yaml.Node, 0, len(limitedProxiesNode.Content))
	for _, proxyNode := range limitedProxiesNode.Content {
		limitedItems = append(limitedItems, cloneYAMLNode(proxyNode))
	}
	primaryProxiesNode.Content = insertAfterHost(primaryProxiesNode.Content, limitedItems, insertAfter, clashHostName)

	// Insert limited proxy names into the first proxy-groups[].proxies list.
	proxyGroupsNode := mappingGetValue(primaryRoot, "proxy-groups")
	if proxyGroupsNode == nil {
		return nil, fmt.Errorf("primary config has no 'proxy-groups' key")
	}
	if proxyGroupsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("primary config 'proxy-groups' is not a sequence")
	}
	if len(proxyGroupsNode.Content) == 0 {
		return nil, fmt.Errorf("primary config 'proxy-groups' is empty")
	}

	firstGroupProxies := findFirstProxyGroupProxies(proxyGroupsNode)
	if firstGroupProxies == nil {
		return nil, fmt.Errorf("primary config proxy-groups has no 'proxies' sequence")
	}
	insertStringsUnique(firstGroupProxies, proxyNames, insertAfter, primaryProxyNames)

	// Serialize the result
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&primaryNode); err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// extractProxyNames extracts the "name" field from every item in the sequence
// node, where each item is a mapping that represents a proxy.
func extractProxyNames(proxiesSeq *yaml.Node) ([]string, error) {
	names := make([]string, 0, len(proxiesSeq.Content))
	for i, proxyNode := range proxiesSeq.Content {
		if proxyNode.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("proxies[%d] is not a mapping", i)
		}
		nameNode := mappingGetValue(proxyNode, "name")
		if nameNode == nil {
			return nil, fmt.Errorf("proxies[%d] has no 'name' field", i)
		}
		names = append(names, nameNode.Value)
	}
	return names, nil
}

func clashHostName(proxy *yaml.Node) string {
	if proxy.Kind != yaml.MappingNode {
		return ""
	}
	name := mappingGetValue(proxy, "name")
	if name == nil || name.Kind != yaml.ScalarNode || name.Tag != "!!str" {
		return ""
	}
	return name.Value
}

// insertStringsUnique inserts new references as a block, preserving existing
// values and excluding duplicates. Only primary proxy names can be anchors.
func insertStringsUnique(seq *yaml.Node, values []string, insertAfter string, primaryProxyNames map[string]struct{}) {
	seen := make(map[string]struct{}, len(seq.Content)+len(values))
	for _, item := range seq.Content {
		if item.Kind == yaml.ScalarNode {
			seen[item.Value] = struct{}{}
		}
	}
	limited := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		limited = append(limited, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: v,
			Tag:   "!!str",
		})
		seen[v] = struct{}{}
	}
	seq.Content = insertAfterHost(seq.Content, limited, insertAfter, func(item *yaml.Node) string {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return ""
		}
		if _, ok := primaryProxyNames[item.Value]; !ok {
			return ""
		}
		return item.Value
	})
}

// findFirstProxyGroupProxies returns the first proxy-groups[].proxies list,
// regardless of the group name.
func findFirstProxyGroupProxies(proxyGroups *yaml.Node) *yaml.Node {
	for _, group := range proxyGroups.Content {
		if group.Kind != yaml.MappingNode {
			continue
		}
		proxies := mappingGetValue(group, "proxies")
		if proxies != nil && proxies.Kind == yaml.SequenceNode {
			return proxies
		}
	}
	return nil
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := *n
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, 0, len(n.Content))
		for _, child := range n.Content {
			clone.Content = append(clone.Content, cloneYAMLNode(child))
		}
	}
	return &clone
}

// unwrapDocument extracts the root MappingNode from a DocumentNode.
// yaml.Unmarshal(&yaml.Node{}) always returns a DocumentNode.
func unwrapDocument(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected DocumentNode, got kind=%d", doc.Kind)
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("DocumentNode has no children")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected root MappingNode, got kind=%d", root.Kind)
	}
	return root, nil
}

// mappingGetValue looks up a value by key in a MappingNode.
// A yaml.Node MappingNode stores Content as [key0, val0, key1, val1, ...].
// It returns nil if the key is not found.
func mappingGetValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
