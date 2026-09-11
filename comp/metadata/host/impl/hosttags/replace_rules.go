// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package hosttags

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/DataDog/datadog-agent/pkg/config/structure"
	"github.com/DataDog/datadog-agent/pkg/util/hostname/validate"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

const (
	hostTagsReplaceRulesKey    = "host_tags_replace_rules"
	hostAliasesReplaceRulesKey = "host_aliases_replace_rules"
)

// ReplaceRule rewrites a host tag value or a host alias with a regular
// expression. It has the shape of an apm_config.replace_tags rule.
type ReplaceRule struct {
	Name    string `mapstructure:"name"`
	Pattern string `mapstructure:"pattern"`
	Repl    string `mapstructure:"repl"`
}

type compiledReplaceRule struct {
	name string
	re   *regexp.Regexp
	repl string
}

var logLoadedRulesOnce sync.Map

func loadReplaceRules(conf model.Reader, key string) ([]compiledReplaceRule, error) {
	rules := []ReplaceRule{}
	if err := structure.UnmarshalKey(conf, key, &rules, structure.EnableStringUnmarshal); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	compiled := make([]compiledReplaceRule, 0, len(rules))
	for i, rule := range rules {
		if rule.Name == "" || rule.Pattern == "" {
			return nil, fmt.Errorf("%s: rule %d needs a \"name\" and a \"pattern\"", key, i)
		}
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s: rule %d: %w", key, i, err)
		}
		compiled = append(compiled, compiledReplaceRule{name: rule.Name, re: re, repl: rule.Repl})
	}
	if _, logged := logLoadedRulesOnce.LoadOrStore(key, true); !logged && len(compiled) > 0 {
		log.Infof("%d %s loaded", len(compiled), key)
	}
	return compiled, nil
}

// applyTagReplaceRules rewrites the value of each "key:value" tag that a rule
// targets. A rule targets a tag when its name is "*" or equals the tag key.
// A tag whose value becomes empty is dropped. Tags without a value are kept.
func applyTagReplaceRules(tags []string, rules []compiledReplaceRule) []string {
	if len(rules) == 0 {
		return tags
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		key, value, found := strings.Cut(tag, ":")
		if !found {
			out = append(out, tag)
			continue
		}
		matched := false
		for _, rule := range rules {
			if rule.name != "*" && rule.name != key {
				continue
			}
			matched = true
			value = rule.re.ReplaceAllString(value, rule.repl)
		}
		if !matched {
			out = append(out, tag)
			continue
		}
		if value == "" {
			continue
		}
		out = append(out, key+":"+value)
	}
	return out
}

// applyAliasReplaceRules rewrites each alias with every rule. An alias that
// becomes empty or is not a valid hostname is dropped.
func applyAliasReplaceRules(aliases []string, rules []compiledReplaceRule) []string {
	if len(rules) == 0 {
		return aliases
	}
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		rewritten := alias
		for _, rule := range rules {
			rewritten = rule.re.ReplaceAllString(rewritten, rule.repl)
		}
		if rewritten == "" {
			continue
		}
		if rewritten != alias {
			if err := validate.ValidHostname(rewritten); err != nil {
				log.Debugf("Dropping host alias after %s: %s", hostAliasesReplaceRulesKey, err)
				continue
			}
		}
		out = append(out, rewritten)
	}
	return out
}

// ApplyHostAliasReplaceRules applies host_aliases_replace_rules to the aliases
// the host metadata payload reports. Config load already rejected malformed
// rules, so a load error here keeps the aliases unchanged and is logged.
func ApplyHostAliasReplaceRules(conf model.Reader, aliases []string) []string {
	rules, err := loadReplaceRules(conf, hostAliasesReplaceRulesKey)
	if err != nil {
		log.Errorf("Ignoring %s: %v", hostAliasesReplaceRulesKey, err)
		return aliases
	}
	return applyAliasReplaceRules(aliases, rules)
}
