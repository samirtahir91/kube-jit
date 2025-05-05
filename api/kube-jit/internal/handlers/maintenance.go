package handlers

import (
	"sync"
)

var (
	maintenanceMode     bool
	maintenanceModeLock sync.RWMutex
)

func SetMaintenanceMode(on bool) {
	maintenanceModeLock.Lock()
	defer maintenanceModeLock.Unlock()
	maintenanceMode = on
}

func IsMaintenanceMode() bool {
	maintenanceModeLock.RLock()
	defer maintenanceModeLock.RUnlock()
	return maintenanceMode
}
