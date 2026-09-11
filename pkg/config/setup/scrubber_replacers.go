// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package setup

import (
	"fmt"
	"regexp"

	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/config/structure"
	"github.com/DataDog/datadog-agent/pkg/util/scrubber"
)

const scrubberAdditionalReplacersKey = "scrubber.additional_replacers"

// addScrubberAdditionalReplacers registers scrubber.additional_replacers on the
// default scrubber. A malformed rule is an error, so the Agent stops at config
// load rather than run with a scrubber that is missing a rule.
func addScrubberAdditionalReplacers(config pkgconfigmodel.Reader) error {
	rules := []hostReplaceRule{}
	if err := structure.UnmarshalKey(config, scrubberAdditionalReplacersKey, &rules, structure.EnableStringUnmarshal); err != nil {
		return fmt.Errorf("%s: bad format, expected [{\"name\": \"<label>\", \"pattern\": \"<regexp>\", \"repl\": \"<text>\"}]: %w", scrubberAdditionalReplacersKey, err)
	}
	for i, rule := range rules {
		if rule.Pattern == "" {
			return fmt.Errorf("%s: rule %d has no \"pattern\"", scrubberAdditionalReplacersKey, i)
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("%s: rule %d: %w", scrubberAdditionalReplacersKey, i, err)
		}
		scrubber.DefaultScrubber.AddReplacer(scrubber.SingleLine, scrubber.Replacer{
			Regex: re,
			Repl:  []byte(rule.Repl),
		})
	}
	return nil
}
