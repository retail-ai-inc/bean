package dbdrivers

import (
	"fmt"
	"math"
	"strconv"
)

// tenantString returns the string field key of a tenant connection config (the `TenantConnections.Connections`
// JSON), "" if it is missing. A JSON integer is accepted too (e.g. `"port": 3306`). Any other type is an error
// instead of a panic, so that a wrong tenant row fails the initialization with a message.
func tenantString(cfg map[string]interface{}, key string) (string, error) {
	switch v := cfg[key].(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case float64:
		if v == math.Trunc(v) && !math.IsInf(v, 0) {
			return strconv.FormatInt(int64(v), 10), nil
		}
	}
	return "", fmt.Errorf("%q must be a string, got %T", key, cfg[key])
}

// requiredTenantString is `tenantString` for a field that must be present (it may be empty, as before: only a
// missing field, which used to panic, is an error).
func requiredTenantString(cfg map[string]interface{}, key string) (string, error) {
	if _, ok := cfg[key]; !ok {
		return "", fmt.Errorf("%q is required", key)
	}
	return tenantString(cfg, key)
}

// tenantHost returns the host of a tenant connection config: the field named tenantAlterDbHostParam if it is set
// (see `bean.TenantAlterDbHostParam`), otherwise `host`.
func tenantHost(cfg map[string]interface{}, tenantAlterDbHostParam string) (string, error) {
	if tenantAlterDbHostParam != "" && cfg[tenantAlterDbHostParam] != nil {
		return requiredTenantString(cfg, tenantAlterDbHostParam)
	}
	return requiredTenantString(cfg, "host")
}
