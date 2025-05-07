package k8s

import (
	"kube-jit/internal/models"
)

const jitgroupcacheName = "jitgroupcache" // Static name for the JitGroupCache object

var (
	ApiConfig             Config
	AllowedRoles          []models.Roles
	PlatformApproverTeams []models.Team
	AdminTeams            []models.Team
	ClusterNames          []string
	ClusterConfigs        = make(map[string]ClusterConfig)
	callbackHostOverride  string // from utils.MustGetEnv("CALLBACK_HOST_OVERRIDE") to be used in CreateK8sObject
)
