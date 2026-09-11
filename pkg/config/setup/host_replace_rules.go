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
)

const (
	hostTagsReplaceRulesKey    = "host_tags_replace_rules"
	hostAliasesReplaceRulesKey = "host_aliases_replace_rules"
)

// hostReplaceRule has the shape of an apm_config.replace_tags rule.
type hostReplaceRule struct {
	Name    string `mapstructure:"name"`
	Pattern string `mapstructure:"pattern"`
	Repl    string `mapstructure:"repl"`
}

// validateHostReplaceRules rejects a malformed rule at config load, so the
// Agent stops before it sends its first host metadata payload.
func validateHostReplaceRules(config pkgconfigmodel.Reader) error {
	if err := validateHostReplaceRulesKey(config, hostTagsReplaceRulesKey, false); err != nil {
		return err
	}
	return validateHostReplaceRulesKey(config, hostAliasesReplaceRulesKey, true)
}

func validateHostReplaceRulesKey(config pkgconfigmodel.Reader, key string, wildcardOnly bool) error {
	rules := []hostReplaceRule{}
	if err := structure.UnmarshalKey(config, key, &rules, structure.EnableStringUnmarshal); err != nil {
		return fmt.Errorf("%s: bad format, expected [{\"name\": \"<tag>\", \"pattern\": \"<regexp>\", \"repl\": \"<text>\"}]: %w", key, err)
	}
	for i, rule := range rules {
		if rule.Name == "" {
			return fmt.Errorf("%s: rule %d has no \"name\" (use \"*\" to target every entry)", key, i)
		}
		if wildcardOnly && rule.Name != "*" {
			return fmt.Errorf("%s: rule %d: \"name\" must be \"*\", got %q", key, i, rule.Name)
		}
		if rule.Pattern == "" {
			return fmt.Errorf("%s: rule %d has no \"pattern\"", key, i)
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("%s: rule %d: %w", key, i, err)
		}
	}
	return nil
}
