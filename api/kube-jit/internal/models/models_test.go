package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginResponse_JSON(t *testing.T) {
	original := LoginResponse{
		UserData: NormalizedUserData{
			ID:        "user123",
			Name:      "Test User",
			Email:     "test@example.com",
			AvatarURL: "http://example.com/avatar.png",
			Provider:  "github",
		},
		ExpiresIn: 3600,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	require.NoError(t, err, "Failed to marshal LoginResponse to JSON")

	// Unmarshal back to struct
	var unmarshaled LoginResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err, "Failed to unmarshal JSON to LoginResponse")

	// Assert equality
	assert.Equal(t, original, unmarshaled, "Original and unmarshaled LoginResponse should be equal")

	// Optional: Assert specific JSON structure or values if needed
	expectedJSON := `{"userData":{"id":"user123","name":"Test User","email":"test@example.com","avatar_url":"http://example.com/avatar.png","provider":"github"},"expiresIn":3600}`
	assert.JSONEq(t, expectedJSON, string(jsonData), "JSON output for LoginResponse is not as expected")
}

func TestRequestData_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	startTime := now.Add(-time.Hour)
	endTime := now.Add(time.Hour)
	createdAt := now.Add(-2 * time.Hour)
	updatedAt := now.Add(-1 * time.Hour)

	original := RequestData{
		GormModel: GormModel{
			ID:        1,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			DeletedAt: nil,
		},
		ApproverIDs:   []string{"approver1", "approver2"},
		ApproverNames: []string{"Approver One", "Approver Two"},
		ClusterName:   "test-cluster",
		RoleName:      "admin-role",
		Status:        "Pending",
		Notes:         "Test request notes",
		UserID:        "user456",
		Username:      "Requester User",
		Users:         []string{"targetUser1", "targetUser2"},
		Namespaces:    []string{"ns1", "ns2"},
		Justification: "Need access for testing",
		StartDate:     startTime,
		EndDate:       endTime,
		FullyApproved: false,
		Email:         "requester@example.com",
	}

	jsonData, err := json.Marshal(original)
	require.NoError(t, err)

	var unmarshaled RequestData
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, original, unmarshaled)

	// Verify time formatting (RFC3339Nano is default for time.Time in JSON)
	var rawMap map[string]interface{}
	err = json.Unmarshal(jsonData, &rawMap)
	require.NoError(t, err)
	assert.Equal(t, startTime.Format(time.RFC3339Nano), rawMap["startDate"])
	assert.Equal(t, endTime.Format(time.RFC3339Nano), rawMap["endDate"])
	assert.Equal(t, createdAt.Format(time.RFC3339Nano), rawMap["CreatedAt"])
	assert.Equal(t, updatedAt.Format(time.RFC3339Nano), rawMap["UpdatedAt"])
	assert.Nil(t, rawMap["DeletedAt"], "DeletedAt should be null when not set")
}

func TestGitHubUser_JSON(t *testing.T) {
	original := GitHubUser{
		Login:     "ghUser",
		ID:        12345,
		AvatarURL: "http://github.com/avatar.png",
		Email:     "gh@example.com",
	}
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	var unmarshaled GitHubUser
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
	expectedJSON := `{"login":"ghUser","id":12345,"avatar_url":"http://github.com/avatar.png","email":"gh@example.com"}`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestGoogleUser_JSON(t *testing.T) {
	original := GoogleUser{
		ID:            "google123",
		Name:          "Google User",
		Email:         "google@example.com",
		Picture:       "http://google.com/pic.png",
		VerifiedEmail: true,
	}
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	var unmarshaled GoogleUser
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
	expectedJSON := `{"id":"google123","name":"Google User","email":"google@example.com","picture":"http://google.com/pic.png","verified_email":true}`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestAzureUser_JSON(t *testing.T) {
	original := AzureUser{
		ID:                "azure-user-id",
		DisplayName:       "Azure Display Name",
		Mail:              "azure.user@example.com",
		UserPrincipalName: "azure.user@example.com",
	}
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	var unmarshaled AzureUser
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
	expectedJSON := `{"id":"azure-user-id","displayName":"Azure Display Name","mail":"azure.user@example.com","userPrincipalName":"azure.user@example.com"}`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestNormalizedUserData_JSON(t *testing.T) {
	original := NormalizedUserData{
		ID:        "norm123",
		Name:      "Normalized User",
		Email:     "norm@example.com",
		AvatarURL: "http://example.com/norm_avatar.png",
		Provider:  "azure",
	}
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	var unmarshaled NormalizedUserData
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
	expectedJSON := `{"id":"norm123","name":"Normalized User","email":"norm@example.com","avatar_url":"http://example.com/norm_avatar.png","provider":"azure"}`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestRequestNamespace_JSON(t *testing.T) {
	original := RequestNamespace{
		ID:           1,
		RequestID:    100,
		Namespace:    "kube-system",
		GroupID:      "group-abc",
		GroupName:    "Kube Admins",
		Approved:     true,
		ApproverID:   "approver-xyz",
		ApproverName: "Admin Approver",
	}
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	var unmarshaled RequestNamespace
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, original, unmarshaled)
	expectedJSON := `{"ID":1,"RequestID":100,"Namespace":"kube-system","GroupID":"group-abc","groupName":"Kube Admins","Approved":true,"approverID":"approver-xyz","approverName":"Admin Approver"}`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestGormModel_JSON_OmitEmpty(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deletedTime := now.Add(time.Hour)

	// Case 1: DeletedAt is nil
	model1 := GormModel{ID: 1, CreatedAt: now, UpdatedAt: now, DeletedAt: nil}
	jsonData1, err := json.Marshal(model1)
	require.NoError(t, err)
	assert.NotContains(t, string(jsonData1), `"DeletedAt"`, "DeletedAt should be omitted when nil")

	// Case 2: DeletedAt is set
	model2 := GormModel{ID: 2, CreatedAt: now, UpdatedAt: now, DeletedAt: &deletedTime}
	jsonData2, err := json.Marshal(model2)
	require.NoError(t, err)
	assert.Contains(t, string(jsonData2), `"DeletedAt"`, "DeletedAt should be present when set")

	var unmarshaledModel2 GormModel
	err = json.Unmarshal(jsonData2, &unmarshaledModel2)
	require.NoError(t, err)
	require.NotNil(t, unmarshaledModel2.DeletedAt)
	assert.Equal(t, deletedTime, *(unmarshaledModel2.DeletedAt))
}
