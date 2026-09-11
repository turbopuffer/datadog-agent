// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package hosttags

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rule(name, pattern, repl string) compiledReplaceRule {
	return compiledReplaceRule{name: name, re: regexp.MustCompile(pattern), repl: repl}
}

func TestApplyTagReplaceRules(t *testing.T) {
	ipRule := rule("*", `\b\d{1,3}(\.\d{1,3}){3}\b`, "[IP]")
	dropKubeNode := rule("kube_node", "^.*$", "")

	for name, tc := range map[string]struct {
		tags     []string
		rules    []compiledReplaceRule
		expected []string
	}{
		"no rules keeps tags": {
			tags:     []string{"kube_node:ip-10-0-1-23", "bare"},
			rules:    nil,
			expected: []string{"kube_node:ip-10-0-1-23", "bare"},
		},
		"named rule drops the tag when the value becomes empty": {
			tags:     []string{"kube_node:ip-10-0-1-23.us-gov-west-1.compute.internal", "role:index"},
			rules:    []compiledReplaceRule{dropKubeNode},
			expected: []string{"role:index"},
		},
		"wildcard rule rewrites every value": {
			tags:     []string{"node_ip:10.0.1.23", "endpoint:http://10.0.1.23:10250/metrics", "role:index"},
			rules:    []compiledReplaceRule{ipRule},
			expected: []string{"node_ip:[IP]", "endpoint:http://[IP]:10250/metrics", "role:index"},
		},
		"rules apply in order": {
			tags:     []string{"kube_node:10.0.1.23"},
			rules:    []compiledReplaceRule{ipRule, dropKubeNode},
			expected: []string{},
		},
		"bare tags are never rewritten or dropped": {
			tags:     []string{"bare", "kube_node"},
			rules:    []compiledReplaceRule{rule("*", "^.*$", "")},
			expected: []string{"bare", "kube_node"},
		},
		"a tag with an empty value survives when no rule targets it": {
			tags:     []string{"empty:", "role:index"},
			rules:    []compiledReplaceRule{dropKubeNode},
			expected: []string{"empty:", "role:index"},
		},
		"the value may contain colons": {
			tags:     []string{"url:https://10.0.1.23:10250"},
			rules:    []compiledReplaceRule{ipRule},
			expected: []string{"url:https://[IP]:10250"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, applyTagReplaceRules(tc.tags, tc.rules))
		})
	}
}

func TestApplyAliasReplaceRules(t *testing.T) {
	for name, tc := range map[string]struct {
		aliases  []string
		rules    []compiledReplaceRule
		expected []string
	}{
		"no rules keeps aliases": {
			aliases:  []string{"ip-10-0-1-23.us-gov-west-1.compute.internal"},
			rules:    nil,
			expected: []string{"ip-10-0-1-23.us-gov-west-1.compute.internal"},
		},
		"catch-all empties the list": {
			aliases:  []string{"ip-10-0-1-23.us-gov-west-1.compute.internal", "i-0123456789abcdef0"},
			rules:    []compiledReplaceRule{rule("*", "^.*$", "")},
			expected: []string{},
		},
		"aliases that no rule changes are kept": {
			aliases:  []string{"my-alias"},
			rules:    []compiledReplaceRule{rule("*", `^ip-\d+(-\d+){3}.*$`, "")},
			expected: []string{"my-alias"},
		},
		"a rewrite that is not a hostname is dropped": {
			aliases:  []string{"ip-10-0-1-23.us-gov-west-1.compute.internal"},
			rules:    []compiledReplaceRule{rule("*", `^ip-\d+(-\d+){3}.*$`, "[HOST]")},
			expected: []string{},
		},
		"a rewrite that is a hostname is kept": {
			aliases:  []string{"ip-10-0-1-23.us-gov-west-1.compute.internal"},
			rules:    []compiledReplaceRule{rule("*", `^ip-\d+(-\d+){3}`, "node")},
			expected: []string{"node.us-gov-west-1.compute.internal"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, applyAliasReplaceRules(tc.aliases, tc.rules))
		})
	}
}

func TestLoadReplaceRules(t *testing.T) {
	mockConfig, _ := setupTest(t)

	rules, err := loadReplaceRules(mockConfig, hostTagsReplaceRulesKey)
	require.NoError(t, err)
	assert.Empty(t, rules)

	mockConfig.SetInTest(hostTagsReplaceRulesKey, []map[string]string{
		{"name": "kube_node", "pattern": "^.*$", "repl": ""},
		{"name": "*", "pattern": `\d+`, "repl": "N"},
	})
	rules, err = loadReplaceRules(mockConfig, hostTagsReplaceRulesKey)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "kube_node", rules[0].name)
	assert.Equal(t, "", rules[0].repl)
	assert.Equal(t, "*", rules[1].name)
	assert.Equal(t, "N", rules[1].repl)

	mockConfig.SetInTest(hostTagsReplaceRulesKey, `[{"name":"kube_node","pattern":"^.*$","repl":""}]`)
	rules, err = loadReplaceRules(mockConfig, hostTagsReplaceRulesKey)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "kube_node", rules[0].name)

	mockConfig.SetInTest(hostTagsReplaceRulesKey, []map[string]string{{"name": "kube_node", "pattern": "("}})
	_, err = loadReplaceRules(mockConfig, hostTagsReplaceRulesKey)
	assert.Error(t, err)

	mockConfig.SetInTest(hostTagsReplaceRulesKey, []map[string]string{{"pattern": "^.*$"}})
	_, err = loadReplaceRules(mockConfig, hostTagsReplaceRulesKey)
	assert.Error(t, err)
}

func TestGetWithHostTagsReplaceRules(t *testing.T) {
	mockConfig, ctx := setupTest(t)
	mockConfig.SetInTest("tags", []string{
		"kube_node:ip-10-0-1-23.us-gov-west-1.compute.internal",
		"node_ip:10.0.1.23",
		"role:index",
		"bare",
	})
	mockConfig.SetInTest(hostTagsReplaceRulesKey, []map[string]string{
		{"name": "kube_node", "pattern": "^.*$", "repl": ""},
		{"name": "*", "pattern": `\b\d{1,3}(\.\d{1,3}){3}\b`, "repl": "[IP]"},
	})

	hostTags := Get(ctx, false, mockConfig)
	assert.Equal(t, []string{"bare", "node_ip:[IP]", "role:index"}, hostTags.System)
}

func TestGetWithHostTagsReplaceRulesFromJSONString(t *testing.T) {
	mockConfig, ctx := setupTest(t)
	mockConfig.SetInTest("tags", []string{"kube_node:ip-10-0-1-23", "role:index"})
	mockConfig.SetInTest(hostTagsReplaceRulesKey, `[{"name":"kube_node","pattern":"^.*$","repl":""}]`)

	hostTags := Get(ctx, false, mockConfig)
	assert.Equal(t, []string{"role:index"}, hostTags.System)
}

func TestGetWithMalformedHostTagsReplaceRulesKeepsTags(t *testing.T) {
	mockConfig, ctx := setupTest(t)
	mockConfig.SetInTest("tags", []string{"kube_node:ip-10-0-1-23", "role:index"})
	mockConfig.SetInTest(hostTagsReplaceRulesKey, []map[string]string{{"name": "kube_node", "pattern": "("}})

	hostTags := Get(ctx, false, mockConfig)
	assert.Equal(t, []string{"kube_node:ip-10-0-1-23", "role:index"}, hostTags.System)
}

func TestApplyHostAliasReplaceRules(t *testing.T) {
	mockConfig, _ := setupTest(t)
	aliases := []string{"ip-10-0-1-23.us-gov-west-1.compute.internal", "my-alias"}

	assert.Equal(t, aliases, ApplyHostAliasReplaceRules(mockConfig, aliases))

	mockConfig.SetInTest(hostAliasesReplaceRulesKey, []map[string]string{{"name": "*", "pattern": "^.*$", "repl": ""}})
	assert.Equal(t, []string{}, ApplyHostAliasReplaceRules(mockConfig, aliases))

	mockConfig.SetInTest(hostAliasesReplaceRulesKey, []map[string]string{{"name": "*", "pattern": "(", "repl": ""}})
	assert.Equal(t, aliases, ApplyHostAliasReplaceRules(mockConfig, aliases))
}
