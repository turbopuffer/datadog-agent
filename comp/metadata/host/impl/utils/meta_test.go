// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameimpl"
	"github.com/DataDog/datadog-agent/pkg/util/cache"
)

func TestGetMeta(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewMock(t)

	meta := getMeta(ctx, cfg, hostnameimpl.NewHostnameService())
	assert.NotEmpty(t, meta.SocketHostname)
	assert.NotEmpty(t, meta.Timezones)
	assert.NotEmpty(t, meta.SocketFqdn)
}

func TestGetMetaWithHostAliasesReplaceRules(t *testing.T) {
	ctx := context.Background()

	cfg := config.NewMockWithOverrides(t, map[string]interface{}{
		"host_aliases": []string{"ip-10-0-1-23.us-gov-west-1.compute.internal", "my-alias"},
	})
	meta := getMeta(ctx, cfg, hostnameimpl.NewHostnameService())
	assert.Contains(t, meta.HostAliases, "ip-10-0-1-23.us-gov-west-1.compute.internal")
	assert.Contains(t, meta.HostAliases, "my-alias")

	cfg = config.NewMockWithOverrides(t, map[string]interface{}{
		"host_aliases": []string{"ip-10-0-1-23.us-gov-west-1.compute.internal", "my-alias"},
		"host_aliases_replace_rules": []map[string]string{
			{"name": "*", "pattern": `^ip-\d+(-\d+){3}.*$`, "repl": ""},
		},
	})
	meta = getMeta(ctx, cfg, hostnameimpl.NewHostnameService())
	assert.NotContains(t, meta.HostAliases, "ip-10-0-1-23.us-gov-west-1.compute.internal")
	assert.Contains(t, meta.HostAliases, "my-alias")

	cfg = config.NewMockWithOverrides(t, map[string]interface{}{
		"host_aliases":               []string{"ip-10-0-1-23.us-gov-west-1.compute.internal", "my-alias"},
		"host_aliases_replace_rules": []map[string]string{{"name": "*", "pattern": "^.*$", "repl": ""}},
	})
	meta = getMeta(ctx, cfg, hostnameimpl.NewHostnameService())
	assert.Empty(t, meta.HostAliases)
}

func TestGetMetaFromCache(t *testing.T) {
	ctx := context.Background()
	cfg := config.NewMock(t)

	cache.Cache.Set(metaCacheKey, &Meta{
		SocketHostname: "socket_test",
		Timezones:      []string{"tz_test"},
	}, cache.NoExpiration)

	m := GetMetaFromCache(ctx, cfg, hostnameimpl.NewHostnameService())
	assert.Equal(t, "socket_test", m.SocketHostname)
	assert.Equal(t, []string{"tz_test"}, m.Timezones)
}
