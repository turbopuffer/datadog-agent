// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package setup

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	delegatedauthmock "github.com/DataDog/datadog-agent/comp/core/delegatedauth/mock"
	secretsmock "github.com/DataDog/datadog-agent/comp/core/secrets/mock"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
)

func TestLoadDatadogValidatesHostReplaceRulesWithoutConfigFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "datadog.yaml")

	t.Setenv("DD_HOST_TAGS_REPLACE_RULES", `[{"name":"kube_node","pattern":"^.*$","repl":""}]`)
	conf := newTestConf(t)
	conf.SetConfigFile(missing)
	err := LoadDatadog(conf, secretsmock.New(t), delegatedauthmock.New(t), nil)
	require.ErrorIs(t, err, pkgconfigmodel.ErrConfigFileNotFound)

	t.Setenv("DD_HOST_TAGS_REPLACE_RULES", `[{"name":"kube_node","pattern":"("}]`)
	conf = newTestConf(t)
	conf.SetConfigFile(missing)
	err = LoadDatadog(conf, secretsmock.New(t), delegatedauthmock.New(t), nil)
	require.Error(t, err)
	assert.False(t, errors.Is(err, pkgconfigmodel.ErrConfigFileNotFound))
	assert.Contains(t, err.Error(), "host_tags_replace_rules")
}

func TestValidateHostReplaceRulesDefaults(t *testing.T) {
	conf := confFromYAML(t, "")
	require.NoError(t, validateHostReplaceRules(conf))
}

func TestValidateHostReplaceRulesValid(t *testing.T) {
	conf := confFromYAML(t, `
host_tags_replace_rules:
  - name: kube_node
    pattern: "^.*$"
    repl: ""
  - name: "*"
    pattern: '\b\d{1,3}(\.\d{1,3}){3}\b'
    repl: "[IP]"
host_aliases_replace_rules:
  - name: "*"
    pattern: "^.*$"
    repl: ""
`)
	require.NoError(t, validateHostReplaceRules(conf))
}

func TestValidateHostReplaceRulesRejectsBadRules(t *testing.T) {
	for name, yaml := range map[string]string{
		"pattern does not compile": `
host_tags_replace_rules:
  - name: kube_node
    pattern: "("
`,
		"missing name": `
host_tags_replace_rules:
  - pattern: "^.*$"
    repl: ""
`,
		"missing pattern": `
host_tags_replace_rules:
  - name: kube_node
    repl: ""
`,
		"alias rule name is not a wildcard": `
host_aliases_replace_rules:
  - name: kube_node
    pattern: "^.*$"
    repl: ""
`,
		"not a list of rules": `
host_tags_replace_rules: "kube_node"
`,
	} {
		t.Run(name, func(t *testing.T) {
			conf := confFromYAML(t, yaml)
			assert.Error(t, validateHostReplaceRules(conf))
		})
	}
}

func TestValidateHostReplaceRulesFromEnvJSON(t *testing.T) {
	t.Setenv("DD_HOST_TAGS_REPLACE_RULES", `[{"name":"kube_node","pattern":"^.*$","repl":""}]`)
	t.Setenv("DD_HOST_ALIASES_REPLACE_RULES", `[{"name":"*","pattern":"^.*$","repl":""}]`)
	conf := confFromYAML(t, "")
	require.NoError(t, validateHostReplaceRules(conf))

	t.Setenv("DD_HOST_TAGS_REPLACE_RULES", `[{"name":"kube_node","pattern":"(","repl":""}]`)
	conf = confFromYAML(t, "")
	assert.Error(t, validateHostReplaceRules(conf))
}
