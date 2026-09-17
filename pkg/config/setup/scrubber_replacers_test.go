// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package setup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DataDog/datadog-agent/pkg/util/scrubber"
)

func TestAddScrubberAdditionalReplacersDefaults(t *testing.T) {
	conf := confFromYAML(t, "")
	require.NoError(t, addScrubberAdditionalReplacers(conf))
}

func TestAddScrubberAdditionalReplacers(t *testing.T) {
	conf := confFromYAML(t, `
scrubber:
  additional_replacers:
    - name: test_secret
      pattern: 'tpuf-test-secret-[0-9]+'
      repl: "[SECRET]"
`)
	require.NoError(t, addScrubberAdditionalReplacers(conf))

	assert.Equal(t, "token [SECRET] rotated", scrubber.ScrubLine("token tpuf-test-secret-42 rotated"))
}

func TestAddScrubberAdditionalReplacersRejectsBadRules(t *testing.T) {
	for name, yaml := range map[string]string{
		"pattern does not compile": `
scrubber:
  additional_replacers:
    - name: broken
      pattern: "("
`,
		"missing pattern": `
scrubber:
  additional_replacers:
    - name: broken
      repl: "x"
`,
		"not a list of rules": `
scrubber:
  additional_replacers: "broken"
`,
	} {
		t.Run(name, func(t *testing.T) {
			conf := confFromYAML(t, yaml)
			assert.Error(t, addScrubberAdditionalReplacers(conf))
		})
	}
}
