package models

import (
	"time"

	"gorm.io/gorm"
)

type Team struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

type Roles struct {
	Name string `json:"name" yaml:"name"`
}

type Cluster struct {
	Name string `json:"name"`
}

type RequestData struct {
	gorm.Model
	ApprovingTeamID   string    `json:"approvingTeamID"`
	ApprovingTeamName string    `json:"approvingTeamName"`
	ApproverID        int       `json:"approverID"`
	ApproverName      string    `json:"approverName"`
	ClusterName       string    `json:"clusterName"`
	RoleName          string    `json:"roleName"`
	Status            string    `json:"status"`
	Notes             string    `json:"notes"`
	UserID            string    `json:"userID"`
	Username          string    `json:"username"`
	Users             []string  `gorm:"type:jsonb;serializer:json" json:"users"`
	Namespaces        []string  `gorm:"type:jsonb;serializer:json" json:"namespaces"`
	Justification     string    `json:"justification"`
	StartDate         time.Time `json:"startDate"`
	EndDate           time.Time `json:"endDate"`
}

type GitHubTokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
}
