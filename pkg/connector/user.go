package connector

import (
	"context"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/slack-go/slack"
)

type userResourceType struct {
	resourceType *v2.ResourceType
	client       *slack.Client
}

func (o *userResourceType) ResourceType(_ context.Context) *v2.ResourceType {
	return o.resourceType
}

// Create a new connector resource for a Slack user.
func userResource(ctx context.Context, user *slack.User, parentResourceID *v2.ResourceId) (*v2.Resource, error) {
	profile := make(map[string]interface{})
	profile["first_name"] = user.Profile.FirstName
	profile["last_name"] = user.Profile.LastName
	profile["login"] = user.Profile.Email
	profile["workspace"] = user.Profile.Team
	profile["user_id"] = user.ID

	var userStatus v2.UserTrait_Status_Status
	if user.Deleted {
		userStatus = v2.UserTrait_Status_STATUS_DELETED
	} else {
		userStatus = v2.UserTrait_Status_STATUS_ENABLED
	}

	userTraitOptions := []resource.UserTraitOption{resource.WithUserProfile(profile), resource.WithEmail(user.Profile.Email, true), resource.WithStatus(userStatus)}
	ret, err := resource.NewUserResource(user.Name, resourceTypeUser, user.ID, userTraitOptions, resource.WithParentResourceID(parentResourceID))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (o *userResourceType) Entitlements(_ context.Context, _ *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func (o *userResourceType) Grants(ctx context.Context, resource *v2.Resource, pt *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	var rv []*v2.Grant

	user, err := o.client.GetUserInfoContext(ctx, resource.Id.Resource)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	var userRoles []*v2.Resource

	if user.IsPrimaryOwner {
		rr, err := roleResource(PrimaryOwnerRoleID, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}
		userRoles = append(userRoles, rr)
	}

	if user.IsOwner {
		rr, err := roleResource(OwnerRoleID, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}
		userRoles = append(userRoles, rr)
	}

	if user.IsAdmin {
		rr, err := roleResource(AdminRoleID, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}
		userRoles = append(userRoles, rr)
	}

	if user.IsRestricted {
		if user.IsUltraRestricted {
			rr, err := roleResource(SingleChannelGuestRoleID, resource.ParentResourceId)
			if err != nil {
				return nil, "", nil, err
			}
			userRoles = append(userRoles, rr)
		} else {
			rr, err := roleResource(MultiChannelGuestRoleID, resource.ParentResourceId)
			if err != nil {
				return nil, "", nil, err
			}
			userRoles = append(userRoles, rr)
		}
	}

	if user.IsInvitedUser {
		rr, err := roleResource(InvitedMemberRoleID, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}
		userRoles = append(userRoles, rr)
	}

	if user.IsBot {
		rr, err := roleResource(BotRoleID, resource.ParentResourceId)
		if err != nil {
			return nil, "", nil, err
		}
		userRoles = append(userRoles, rr)
	}

	for _, ur := range userRoles {
		rv = append(rv, grant.NewGrant(ur, RoleAssignmentEntitlement, resource.Id))
	}

	return rv, "", nil, nil
}

func (o *userResourceType) List(ctx context.Context, parentResourceID *v2.ResourceId, pt *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	if parentResourceID == nil {
		return nil, "", nil, nil
	}

	users, err := o.client.GetUsersContext(ctx)
	if err != nil {
		annos, err := annotationsForError(err)
		return nil, "", annos, err
	}

	var rv []*v2.Resource
	for _, user := range users {
		userCopy := user
		ur, err := userResource(ctx, &userCopy, parentResourceID)
		if err != nil {
			return nil, "", nil, err
		}
		rv = append(rv, ur)
	}

	return rv, "", nil, nil
}

func userBuilder(client *slack.Client) *userResourceType {
	return &userResourceType{
		resourceType: resourceTypeUser,
		client:       client,
	}
}
