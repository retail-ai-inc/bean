/**#bean*/ /*#bean.replace({{ .Copyright }})**/

package dbdrivers

import (
	"sync"
)

var tenantIdMutexMap struct {
	sync.RWMutex
	M map[string]uint64
}

func InitTenantIdMutexMap() {
	tenantIdMutexMap.M = make(map[string]uint64)
}

func GetTenantId(key string) (uint64, bool) {
	tenantIdMutexMap.RLock()
	defer tenantIdMutexMap.RUnlock()

	if tenantIdMutexMap.M != nil {

		tenantId, ok := tenantIdMutexMap.M[key]
		return tenantId, ok
	}

	return 0, false
}

func SetTenantId(key string, tenantId uint64) {
	tenantIdMutexMap.RLock()
	defer tenantIdMutexMap.RUnlock()

	if tenantIdMutexMap.M != nil {
		tenantIdMutexMap.M[key] = tenantId
	}
}
