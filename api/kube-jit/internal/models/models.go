package models

import (
	"time"

	"gorm.io/gorm"
)

// Team represents a team structure for both GitHub and Google
type Team struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// Roles represents a role structure
type Roles struct {
	Name string `json:"name" yaml:"name"`
}

// Cluster represents a cluster structure
type Cluster struct {
	Name string `json:"name"`
}

// RequestData represents a JIT request
type RequestData struct {
	gorm.Model
	ApproverIDs   []string  `gorm:"type:jsonb;serializer:json" json:"approverIDs"`
	ApproverNames []string  `gorm:"type:jsonb;serializer:json" json:"approverNames"`
	ClusterName   string    `json:"clusterName"`
	RoleName      string    `json:"roleName"`
	Status        string    `json:"status"`
	Notes         string    `json:"notes"`
	UserID        string    `json:"userID"`
	Username      string    `json:"username"`
	Users         []string  `gorm:"type:jsonb;serializer:json" json:"users"`
	Namespaces    []string  `gorm:"type:jsonb;serializer:json" json:"namespaces"`
	Justification string    `json:"justification"`
	StartDate     time.Time `json:"startDate"`
	EndDate       time.Time `json:"endDate"`
	FullyApproved bool      `gorm:"default:false"`
}

// RequestNamespace represents the namespace-level approval tracking
type RequestNamespace struct {
	ID           uint   `gorm:"primaryKey"`
	RequestID    uint   `gorm:"not null;index"`
	Namespace    string `gorm:"not null"`
	GroupID      string `gorm:"not null"`
	Approved     bool   `gorm:"default:false"`
	ApproverID   string `json:"approverID"`
	ApproverName string `json:"approverName"`
}

// GitHubTokenResponse represents the response from GitHub's OAuth token endpoint
type GitHubTokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
}

// GitHubUser represents a GitHub user's profile
type GitHubUser struct {
	Login     string `json:"login"`
	ID        int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
}

// GoogleUser represents a Google user's profile
type GoogleUser struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

// AzureTokenResponse represents the response from Azure's OAuth token endpoint
type AzureUser struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
}

// NormalizedUserData represents a normalized user profile structure
type NormalizedUserData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
	Provider  string `json:"provider"`
}
