package connector

import (
	"context"
	"fmt"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	ent "github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	resources "github.com/conductorone/baton-sdk/pkg/types/resource"
	enterprise "github.com/conductorone/baton-slack/pkg/connector/client"
)

const (
	// Enterprise grid system roles.
	AnalyticsAdmin         = "Rl0L"
	AuditLogsAdmin         = "Rl0C"
	ChannelAdmin           = "Rl01"
	ChannelManager         = "Rl0A"
	ConversationAdmin      = "Rl05"
	DLPAdmin               = "Rl09"
	ExportsAdmin           = "Rl0F"
	IntegrationsManager    = "Rl0D"
	MessageActivityManager = "Rl04"
	RoleAdmin              = "Rl02"
	SalesAdmin             = "Rl0G"
	SalesUser              = "Rl0H"
	SecurityAdmin          = "Rl0J"
	SlackPlatformDeveloper = "Rl0B"
	UserAdmin              = "Rl03"
	WorkflowAdmin          = "Rl0K"

	// Enterprise grid organization roles.
	OrganizationPrimaryOwnerID = "organization_primary_owner"
	OrganizationOwnerID        = "organization_owner"
	OrganizationAdminID        = "organization_admin"
)

var systemRoles = map[string]string{
	AnalyticsAdmin:         "Analytics Admin",
	AuditLogsAdmin:         "Audit Logs Admin",
	ChannelAdmin:           "Channel Admin",
	ChannelManager:         "Channel Manager",
	ConversationAdmin:      "Conversation Admin",
	DLPAdmin:               "DLP Admin",
	ExportsAdmin:           "Exports Admin",
	IntegrationsManager:    "Integrations Manager",
	MessageActivityManager: "Message Activity Manager",
	RoleAdmin:              "Role Admin",
	SalesAdmin:             "Sales Admin",
	SalesUser:              "Sales User",
	SecurityAdmin:          "Security Admin",
	SlackPlatformDeveloper: "Slack Platform Developer",
	UserAdmin:              "User Admin",
	WorkflowAdmin:          "Workflow Admin",
}

var organizationRoles = map[string]string{
	OrganizationPrimaryOwnerID: "Organization primary owner",
	OrganizationOwnerID:        "Organization owner",
	OrganizationAdminID:        "Organization admin",
}

type enterpriseRoleType struct {
	resourceType     *v2.ResourceType
	enterpriseClient *enterprise.Client
	enterpriseID     string
}

func (o *enterpriseRoleType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

func enterpriseRoleBuilder(enterpriseID string, enterpriseClient *enterprise.Client) *enterpriseRoleType {
	return &enterpriseRoleType{
		resourceType:     resourceTypeEnterpriseRole,
		enterpriseClient: enterpriseClient,
		enterpriseID:     enterpriseID,
	}
}

func enterpriseRoleResource(roleID string, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	var roleName string
	systemRoleName, ok := systemRoles[roleID]
	if !ok {
		orgRoleName, ok := organizationRoles[roleID]
		if !ok {
			return nil, fmt.Errorf("invalid system or organization roleID: %s", roleID)
		} else {
			roleName = orgRoleName
		}
	} else {
		roleName = systemRoleName
	}

	r, err := resources.NewRoleResource(
		roleName,
		resourceTypeEnterpriseRole,
		roleID,
		nil)

	if err != nil {
		return nil, err
	}

	return r, nil
}

func (o *enterpriseRoleType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	var ret []*v2.Resource
	// no need to sync roles if we don't have an enterprise plan
	if o.enterpriseID == "" {
		return nil, "", nil, nil
	}

	bag, err := parseRolesPageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeEnterpriseRole.Id})
	if err != nil {
		return nil, "", nil, err
	}

	// We only want to do this once
	if bag.Cursor == "" {
		for orgRoleID := range organizationRoles {
			r, err := enterpriseRoleResource(orgRoleID, parentResourceID)
			if err != nil {
				return nil, "", nil, err
			}

			ret = append(ret, r)
		}
	}

	roleAssignments, nextPage, err := o.enterpriseClient.GetRoleAssignments(ctx, "", bag.Cursor)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	bag.Cursor = nextPage

	for _, roleAssignment := range roleAssignments {
		if _, ok := bag.FoundMap[roleAssignment.RoleID]; ok {
			continue
		}

		if _, ok := systemRoles[roleAssignment.RoleID]; !ok {
			continue
		}

		r, err := enterpriseRoleResource(roleAssignment.RoleID, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}

		ret = append(ret, r)

		bag.FoundMap[roleAssignment.RoleID] = true
	}

	nextPageToken, err := bag.Marshal()
	if err != nil {
		return nil, "", nil, err
	}

	return ret, nextPageToken, nil, nil
}

func (o *enterpriseRoleType) Entitlements(ctx context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	rv := []*v2.Entitlement{
		ent.NewAssignmentEntitlement(
			resource,
			RoleAssignmentEntitlement,
			ent.WithGrantableTo(resourceTypeUser),
			ent.WithDescription(fmt.Sprintf("Has the %s role in the Slack enterprise", resource.DisplayName)),
			ent.WithDisplayName(fmt.Sprintf("%s Enterprise Role", resource.DisplayName)),
		),
	}

	return rv, "", nil, nil
}

func (o *enterpriseRoleType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant

	bag, err := parsePageToken(pt.Token, &v2.ResourceId{ResourceType: resourceTypeEnterpriseRole.Id})
	if err != nil {
		return nil, "", nil, err
	}

	// if current role is one of organization roles, don't return any grants since we grant those on the user itself
	if _, ok := organizationRoles[resource.Id.Resource]; ok {
		return nil, "", nil, nil
	}

	roleAssignments, nextPage, err := o.enterpriseClient.GetRoleAssignments(ctx, resource.Id.Resource, bag.PageToken())
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	pageToken, err := bag.NextToken(nextPage)
	if err != nil {
		return nil, "", nil, err
	}

	for _, assignment := range roleAssignments {
		userID, err := resources.NewResourceID(resourceTypeUser, assignment.UserID)
		if err != nil {
			return nil, "", nil, fmt.Errorf("failed to create resourceID for user: %w", err)
		}

		rv = append(rv, grant.NewGrant(resource, RoleAssignmentEntitlement, userID))
	}

	return rv, pageToken, nil, nil
}
