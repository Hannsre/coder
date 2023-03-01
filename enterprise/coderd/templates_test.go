package coderd_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/coder/coder/coderd/audit"
	"github.com/coder/coder/coderd/coderdtest"
	"github.com/coder/coder/coderd/database"
	"github.com/coder/coder/coderd/rbac"
	"github.com/coder/coder/codersdk"
	"github.com/coder/coder/cryptorand"
	"github.com/coder/coder/enterprise/coderd/coderdenttest"
	"github.com/coder/coder/enterprise/coderd/license"
	"github.com/coder/coder/provisioner/echo"
	"github.com/coder/coder/testutil"
)

func TestTemplates(t *testing.T) {
	t.Parallel()

	t.Run("SetMaxTTL", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, &coderdenttest.Options{
			Options: &coderdtest.Options{
				IncludeProvisionerDaemon: true,
			},
		})
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureAdvancedTemplateScheduling: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		coderdtest.AwaitTemplateVersionJob(t, client, version.ID)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		require.EqualValues(t, 0, template.MaxTTLMillis)

		// Create some workspaces to test propagation to user-defined TTLs.
		workspace1 := coderdtest.CreateWorkspace(t, client, user.OrganizationID, template.ID, func(cwr *codersdk.CreateWorkspaceRequest) {
			ttl := (24 * time.Hour).Milliseconds()
			cwr.TTLMillis = &ttl
		})
		workspace2TTL := (1 * time.Hour).Milliseconds()
		workspace2 := coderdtest.CreateWorkspace(t, client, user.OrganizationID, template.ID, func(cwr *codersdk.CreateWorkspaceRequest) {
			cwr.TTLMillis = &workspace2TTL
		})
		workspace3 := coderdtest.CreateWorkspace(t, client, user.OrganizationID, template.ID)
		// To unset TTL you have to update, as setting a nil TTL on create
		// copies the template default TTL.
		ctx, _ := testutil.Context(t)
		err := client.UpdateWorkspaceTTL(ctx, workspace3.ID, codersdk.UpdateWorkspaceTTLRequest{
			TTLMillis: nil,
		})

		updated, err := client.UpdateTemplateMeta(ctx, template.ID, codersdk.UpdateTemplateMeta{
			Name:                         template.Name,
			DisplayName:                  template.DisplayName,
			Description:                  template.Description,
			Icon:                         template.Icon,
			AllowUserCancelWorkspaceJobs: template.AllowUserCancelWorkspaceJobs,
			DefaultTTLMillis:             time.Hour.Milliseconds(),
			MaxTTLMillis:                 (2 * time.Hour).Milliseconds(),
		})
		require.NoError(t, err)
		require.Equal(t, 2*time.Hour, time.Duration(updated.MaxTTLMillis)*time.Millisecond)

		template, err = client.Template(ctx, template.ID)
		require.NoError(t, err)
		require.Equal(t, 2*time.Hour, time.Duration(template.MaxTTLMillis)*time.Millisecond)

		// Verify that only the first workspace has been updated.
		workspace1, err = client.Workspace(ctx, workspace1.ID)
		require.NoError(t, err)
		require.Equal(t, &template.MaxTTLMillis, workspace1.TTLMillis)

		workspace2, err = client.Workspace(ctx, workspace2.ID)
		require.NoError(t, err)
		require.Equal(t, &workspace2TTL, workspace2.TTLMillis)

		workspace3, err = client.Workspace(ctx, workspace3.ID)
		require.NoError(t, err)
		require.Nil(t, workspace3.TTLMillis)
	})

	t.Run("CreateUpdateWorkspaceMaxTTL", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, &coderdenttest.Options{
			Options: &coderdtest.Options{
				IncludeProvisionerDaemon: true,
			},
		})
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureAdvancedTemplateScheduling: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		exp := 24 * time.Hour.Milliseconds()
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID, func(ctr *codersdk.CreateTemplateRequest) {
			ctr.DefaultTTLMillis = &exp
			ctr.MaxTTLMillis = &exp
		})
		coderdtest.AwaitTemplateVersionJob(t, client, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		// No TTL provided should use template default
		req := codersdk.CreateWorkspaceRequest{
			TemplateID: template.ID,
			Name:       "testing",
		}
		ws, err := client.CreateWorkspace(ctx, template.OrganizationID, codersdk.Me, req)
		require.NoError(t, err)
		require.EqualValues(t, exp, *ws.TTLMillis)

		// Editing a workspace to have a higher TTL than the template's max
		// should error
		exp = exp + time.Minute.Milliseconds()
		err = client.UpdateWorkspaceTTL(ctx, ws.ID, codersdk.UpdateWorkspaceTTLRequest{
			TTLMillis: &exp,
		})
		require.Error(t, err)
		var apiErr *codersdk.Error
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, http.StatusBadRequest, apiErr.StatusCode())
		require.Len(t, apiErr.Validations, 1)
		require.Equal(t, apiErr.Validations[0].Field, "ttl_ms")
		require.Contains(t, apiErr.Validations[0].Detail, "time until shutdown must be less than or equal to the template's maximum TTL")

		// Creating workspace with TTL higher than max should error
		req.Name = "testing2"
		req.TTLMillis = &exp
		ws, err = client.CreateWorkspace(ctx, template.OrganizationID, codersdk.Me, req)
		require.Error(t, err)
		apiErr = nil
		require.ErrorAs(t, err, &apiErr)
		require.Equal(t, http.StatusBadRequest, apiErr.StatusCode())
		require.Len(t, apiErr.Validations, 1)
		require.Equal(t, apiErr.Validations[0].Field, "ttl_ms")
		require.Contains(t, apiErr.Validations[0].Detail, "time until shutdown must be less than or equal to the template's maximum TTL")
	})

	t.Run("BlockDisablingAutoOffWithMaxTTL", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, &coderdenttest.Options{
			Options: &coderdtest.Options{
				IncludeProvisionerDaemon: true,
			},
		})
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureAdvancedTemplateScheduling: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		exp := 24 * time.Hour.Milliseconds()
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID, func(ctr *codersdk.CreateTemplateRequest) {
			ctr.MaxTTLMillis = &exp
		})
		coderdtest.AwaitTemplateVersionJob(t, client, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		// No TTL provided should use template default
		req := codersdk.CreateWorkspaceRequest{
			TemplateID: template.ID,
			Name:       "testing",
		}
		ws, err := client.CreateWorkspace(ctx, template.OrganizationID, codersdk.Me, req)
		require.NoError(t, err)
		require.EqualValues(t, exp, *ws.TTLMillis)

		// Editing a workspace to disable the TTL should do nothing
		err = client.UpdateWorkspaceTTL(ctx, ws.ID, codersdk.UpdateWorkspaceTTLRequest{
			TTLMillis: nil,
		})
		require.NoError(t, err)
		ws, err = client.Workspace(ctx, ws.ID)
		require.NoError(t, err)
		require.EqualValues(t, exp, *ws.TTLMillis)

		// Editing a workspace to have a TTL of 0 should do nothing
		zero := int64(0)
		err = client.UpdateWorkspaceTTL(ctx, ws.ID, codersdk.UpdateWorkspaceTTLRequest{
			TTLMillis: &zero,
		})
		require.NoError(t, err)
		ws, err = client.Workspace(ctx, ws.ID)
		require.NoError(t, err)
		require.EqualValues(t, exp, *ws.TTLMillis)
	})
}

func TestTemplateACL(t *testing.T) {
	t.Parallel()

	t.Run("UserRoles", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		_, user3 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleUse,
				user3.ID.String(): codersdk.TemplateRoleAdmin,
			},
		})
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		templateUser2 := codersdk.TemplateUser{
			User: user2,
			Role: codersdk.TemplateRoleUse,
		}

		templateUser3 := codersdk.TemplateUser{
			User: user3,
			Role: codersdk.TemplateRoleAdmin,
		}

		require.Len(t, acl.Users, 2)
		require.Contains(t, acl.Users, templateUser2)
		require.Contains(t, acl.Users, templateUser3)
	})

	t.Run("everyoneGroup", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		// Create a user to assert they aren't returned in the response.
		_, _ = coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 1)
		// We don't return members for the 'Everyone' group.
		require.Len(t, acl.Groups[0].Members, 0)
		require.Len(t, acl.Users, 0)
	})

	t.Run("NoGroups", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client1, _ := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 1)
		require.Len(t, acl.Users, 0)

		// User should be able to read template due to allUsers group.
		_, err = client1.Template(ctx, template.ID)
		require.NoError(t, err)

		allUsers := acl.Groups[0]

		err = client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			GroupPerms: map[string]codersdk.TemplateRole{
				allUsers.ID.String(): codersdk.TemplateRoleDeleted,
			},
		})
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 0)
		require.Len(t, acl.Users, 0)

		// User should not be able to read template due to allUsers group being deleted.
		_, err = client1.Template(ctx, template.ID)
		require.Error(t, err)
		cerr, ok := codersdk.AsError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusNotFound, cerr.StatusCode())
	})

	// Test that we do not return deleted users.
	t.Run("FilterDeletedUsers", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user1 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user1.ID.String(): codersdk.TemplateRoleUse,
			},
		})
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user1,
			Role: codersdk.TemplateRoleUse,
		})

		err = client.DeleteUser(ctx, user1.ID)
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Len(t, acl.Users, 0, "deleted users should be filtered")
	})

	// Test that we do not return suspended users.
	t.Run("FilterSuspendedUsers", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user1 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user1.ID.String(): codersdk.TemplateRoleUse,
			},
		})
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user1,
			Role: codersdk.TemplateRoleUse,
		})

		_, err = client.UpdateUserStatus(ctx, user1.ID.String(), codersdk.UserStatusSuspended)
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Len(t, acl.Users, 0, "suspended users should be filtered")
	})

	// Test that we do not return deleted groups.
	t.Run("FilterDeletedGroups", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		group, err := client.CreateGroup(ctx, user.OrganizationID, codersdk.CreateGroupRequest{
			Name: "test",
		})
		require.NoError(t, err)

		err = client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			GroupPerms: map[string]codersdk.TemplateRole{
				group.ID.String(): codersdk.TemplateRoleUse,
			},
		})
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		// Length should be 2 for test group and the implicit allUsers group.
		require.Len(t, acl.Groups, 2)

		require.Contains(t, acl.Groups, codersdk.TemplateGroup{
			Group: group,
			Role:  codersdk.TemplateRoleUse,
		})

		err = client.DeleteGroup(ctx, group.ID)
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		// Length should be 1 for the allUsers group.
		require.Len(t, acl.Groups, 1)
		require.NotContains(t, acl.Groups, codersdk.TemplateGroup{
			Group: group,
			Role:  codersdk.TemplateRoleUse,
		})
	})

	t.Run("AdminCanPushVersions", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client1, user1 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user1.ID.String(): codersdk.TemplateRoleUse,
			},
		})
		require.NoError(t, err)

		data, err := echo.Tar(nil)
		require.NoError(t, err)
		file, err := client1.Upload(context.Background(), codersdk.ContentTypeTar, bytes.NewReader(data))
		require.NoError(t, err)

		_, err = client1.CreateTemplateVersion(ctx, user.OrganizationID, codersdk.CreateTemplateVersionRequest{
			Name:          "testme",
			TemplateID:    template.ID,
			FileID:        file.ID,
			StorageMethod: codersdk.ProvisionerStorageMethodFile,
			Provisioner:   codersdk.ProvisionerTypeEcho,
		})
		require.Error(t, err)

		err = client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user1.ID.String(): codersdk.TemplateRoleAdmin,
			},
		})
		require.NoError(t, err)

		_, err = client1.CreateTemplateVersion(ctx, user.OrganizationID, codersdk.CreateTemplateVersionRequest{
			Name:          "testme",
			TemplateID:    template.ID,
			FileID:        file.ID,
			StorageMethod: codersdk.ProvisionerStorageMethodFile,
			Provisioner:   codersdk.ProvisionerTypeEcho,
		})
		require.NoError(t, err)
	})
}

func TestUpdateTemplateACL(t *testing.T) {
	t.Parallel()

	t.Run("UserPerms", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		_, user3 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		err := client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleUse,
				user3.ID.String(): codersdk.TemplateRoleAdmin,
			},
		})
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		templateUser2 := codersdk.TemplateUser{
			User: user2,
			Role: codersdk.TemplateRoleUse,
		}

		templateUser3 := codersdk.TemplateUser{
			User: user3,
			Role: codersdk.TemplateRoleAdmin,
		}

		require.Len(t, acl.Users, 2)
		require.Contains(t, acl.Users, templateUser2)
		require.Contains(t, acl.Users, templateUser3)
	})

	t.Run("Audit", func(t *testing.T) {
		t.Parallel()

		auditor := audit.NewMock()
		client := coderdenttest.New(t, &coderdenttest.Options{
			AuditLogging: true,
			Options: &coderdtest.Options{
				IncludeProvisionerDaemon: true,
				Auditor:                  auditor,
			},
		})

		user := coderdtest.CreateFirstUser(t, client)

		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
				codersdk.FeatureAuditLog:     1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		numLogs := len(auditor.AuditLogs)

		req := codersdk.UpdateTemplateACL{
			GroupPerms: map[string]codersdk.TemplateRole{
				user.OrganizationID.String(): codersdk.TemplateRoleDeleted,
			},
		}
		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)
		numLogs++

		require.Len(t, auditor.AuditLogs, numLogs)
		require.Equal(t, database.AuditActionWrite, auditor.AuditLogs[numLogs-1].Action)
		require.Equal(t, template.ID, auditor.AuditLogs[numLogs-1].ResourceID)
	})

	t.Run("DeleteUser", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		_, user3 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleUse,
				user3.ID.String(): codersdk.TemplateRoleAdmin,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user2,
			Role: codersdk.TemplateRoleUse,
		})
		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user3,
			Role: codersdk.TemplateRoleAdmin,
		})

		req = codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleAdmin,
				user3.ID.String(): codersdk.TemplateRoleDeleted,
			},
		}

		err = client.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user2,
			Role: codersdk.TemplateRoleAdmin,
		})

		require.NotContains(t, acl.Users, codersdk.TemplateUser{
			User: user3,
			Role: codersdk.TemplateRoleAdmin,
		})
	})

	t.Run("InvalidUUID", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				"hi": "admin",
			},
		}

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.Error(t, err)
		cerr, _ := codersdk.AsError(err)
		require.Equal(t, http.StatusBadRequest, cerr.StatusCode())
	})

	t.Run("InvalidUser", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				uuid.NewString(): "admin",
			},
		}

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.Error(t, err)
		cerr, _ := codersdk.AsError(err)
		require.Equal(t, http.StatusBadRequest, cerr.StatusCode())
	})

	t.Run("InvalidRole", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		_, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): "updater",
			},
		}

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.Error(t, err)
		cerr, _ := codersdk.AsError(err)
		require.Equal(t, http.StatusBadRequest, cerr.StatusCode())
	})

	t.Run("RegularUserCannotUpdatePerms", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client2, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleUse,
			},
		}

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)

		req = codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleAdmin,
			},
		}

		err = client2.UpdateTemplateACL(ctx, template.ID, req)
		require.Error(t, err)
		cerr, _ := codersdk.AsError(err)
		require.Equal(t, http.StatusNotFound, cerr.StatusCode())
	})

	t.Run("RegularUserWithAdminCanUpdate", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client2, user2 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		_, user3 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)
		req := codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user2.ID.String(): codersdk.TemplateRoleAdmin,
			},
		}

		ctx, _ := testutil.Context(t)

		err := client.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)

		req = codersdk.UpdateTemplateACL{
			UserPerms: map[string]codersdk.TemplateRole{
				user3.ID.String(): codersdk.TemplateRoleUse,
			},
		}

		err = client2.UpdateTemplateACL(ctx, template.ID, req)
		require.NoError(t, err)

		acl, err := client2.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Contains(t, acl.Users, codersdk.TemplateUser{
			User: user3,
			Role: codersdk.TemplateRoleUse,
		})
	})

	t.Run("allUsersGroup", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 1)
		require.Len(t, acl.Users, 0)
	})

	t.Run("CustomGroupHasAccess", func(t *testing.T) {
		t.Parallel()

		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client1, user1 := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, _ := testutil.Context(t)

		// Create a group to add to the template.
		group, err := client.CreateGroup(ctx, user.OrganizationID, codersdk.CreateGroupRequest{
			Name: "test",
		})
		require.NoError(t, err)

		// Check that the only current group is the allUsers group.
		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)
		require.Len(t, acl.Groups, 1)

		// Update the template to only allow access to the 'test' group.
		err = client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			GroupPerms: map[string]codersdk.TemplateRole{
				// The allUsers group shares the same ID as the organization.
				user.OrganizationID.String(): codersdk.TemplateRoleDeleted,
				group.ID.String():            codersdk.TemplateRoleUse,
			},
		})
		require.NoError(t, err)

		// Get the ACL list for the template and assert the test group is
		// present.
		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 1)
		require.Len(t, acl.Users, 0)
		require.Equal(t, group.ID, acl.Groups[0].ID)

		// Try to get the template as the regular user. This should
		// fail since we haven't been added to the template yet.
		_, err = client1.Template(ctx, template.ID)
		require.Error(t, err)
		cerr, ok := codersdk.AsError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusNotFound, cerr.StatusCode())

		// Patch the group to add the regular user.
		group, err = client.PatchGroup(ctx, group.ID, codersdk.PatchGroupRequest{
			AddUsers: []string{user1.ID.String()},
		})
		require.NoError(t, err)
		require.Len(t, group.Members, 1)
		require.Equal(t, user1.ID, group.Members[0].ID)

		// Fetching the template should succeed since our group has view access.
		_, err = client1.Template(ctx, template.ID)
		require.NoError(t, err)
	})

	t.Run("NoAccess", func(t *testing.T) {
		t.Parallel()
		client := coderdenttest.New(t, nil)
		user := coderdtest.CreateFirstUser(t, client)
		_ = coderdenttest.AddLicense(t, client, coderdenttest.LicenseOptions{
			Features: license.Features{
				codersdk.FeatureTemplateRBAC: 1,
			},
		})

		client1, _ := coderdtest.CreateAnotherUser(t, client, user.OrganizationID)
		version := coderdtest.CreateTemplateVersion(t, client, user.OrganizationID, nil)
		template := coderdtest.CreateTemplate(t, client, user.OrganizationID, version.ID)

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		acl, err := client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 1)
		require.Len(t, acl.Users, 0)

		// User should be able to read template due to allUsers group.
		_, err = client1.Template(ctx, template.ID)
		require.NoError(t, err)

		allUsers := acl.Groups[0]

		err = client.UpdateTemplateACL(ctx, template.ID, codersdk.UpdateTemplateACL{
			GroupPerms: map[string]codersdk.TemplateRole{
				allUsers.ID.String(): codersdk.TemplateRoleDeleted,
			},
		})
		require.NoError(t, err)

		acl, err = client.TemplateACL(ctx, template.ID)
		require.NoError(t, err)

		require.Len(t, acl.Groups, 0)
		require.Len(t, acl.Users, 0)

		// User should not be able to read template due to allUsers group being deleted.
		_, err = client1.Template(ctx, template.ID)
		require.Error(t, err)
		cerr, ok := codersdk.AsError(err)
		require.True(t, ok)
		require.Equal(t, http.StatusNotFound, cerr.StatusCode())
	})
}

// TestTemplateAccess tests the rego -> sql conversion. We need to implement
// this test on at least 1 table type to ensure that the conversion is correct.
// The rbac tests only assert against static SQL queries.
// This is a full rbac test of many of the common role combinations.
//
//nolint:tparallel
func TestTemplateAccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
	t.Cleanup(cancel)

	ownerClient := coderdenttest.New(t, nil)
	owner := coderdtest.CreateFirstUser(t, ownerClient)
	_ = coderdenttest.AddLicense(t, ownerClient, coderdenttest.LicenseOptions{
		Features: license.Features{
			codersdk.FeatureTemplateRBAC: 1,
		},
	})

	type coderUser struct {
		*codersdk.Client
		User codersdk.User
	}

	type orgSetup struct {
		Admin         coderUser
		MemberInGroup coderUser
		MemberNoGroup coderUser

		DefaultTemplate codersdk.Template
		AllRead         codersdk.Template
		UserACL         codersdk.Template
		GroupACL        codersdk.Template

		Group codersdk.Group
		Org   codersdk.Organization
	}

	// Create the following users
	// - owner: Site wide owner
	// - template-admin
	// - org-admin (org 1)
	// - org-admin (org 2)
	// - org-member (org 1)
	// - org-member (org 2)

	// Create the following templates in each org
	// - template 1, default acls
	// - template 2, all_user read
	// - template 3, user_acl read for member
	// - template 4, group_acl read for groupMember

	templateAdmin, _ := coderdtest.CreateAnotherUser(t, ownerClient, owner.OrganizationID, rbac.RoleTemplateAdmin())

	makeTemplate := func(t *testing.T, client *codersdk.Client, orgID uuid.UUID, acl codersdk.UpdateTemplateACL) codersdk.Template {
		version := coderdtest.CreateTemplateVersion(t, client, orgID, nil)
		template := coderdtest.CreateTemplate(t, client, orgID, version.ID)

		err := client.UpdateTemplateACL(ctx, template.ID, acl)
		require.NoError(t, err, "failed to update template acl")

		return template
	}

	makeOrg := func(t *testing.T) orgSetup {
		// Make org
		orgName, err := cryptorand.String(5)
		require.NoError(t, err, "org name")

		// Make users
		newOrg, err := ownerClient.CreateOrganization(ctx, codersdk.CreateOrganizationRequest{Name: orgName})
		require.NoError(t, err, "failed to create org")

		adminCli, adminUsr := coderdtest.CreateAnotherUser(t, ownerClient, newOrg.ID, rbac.RoleOrgAdmin(newOrg.ID))
		groupMemCli, groupMemUsr := coderdtest.CreateAnotherUser(t, ownerClient, newOrg.ID, rbac.RoleOrgMember(newOrg.ID))
		memberCli, memberUsr := coderdtest.CreateAnotherUser(t, ownerClient, newOrg.ID, rbac.RoleOrgMember(newOrg.ID))

		// Make group
		group, err := adminCli.CreateGroup(ctx, newOrg.ID, codersdk.CreateGroupRequest{
			Name: "SingleUser",
		})
		require.NoError(t, err, "failed to create group")

		group, err = adminCli.PatchGroup(ctx, group.ID, codersdk.PatchGroupRequest{
			AddUsers: []string{groupMemUsr.ID.String()},
		})
		require.NoError(t, err, "failed to add user to group")

		// Make templates

		return orgSetup{
			Admin:         coderUser{Client: adminCli, User: adminUsr},
			MemberInGroup: coderUser{Client: groupMemCli, User: groupMemUsr},
			MemberNoGroup: coderUser{Client: memberCli, User: memberUsr},
			Org:           newOrg,
			Group:         group,

			DefaultTemplate: makeTemplate(t, adminCli, newOrg.ID, codersdk.UpdateTemplateACL{
				GroupPerms: map[string]codersdk.TemplateRole{
					newOrg.ID.String(): codersdk.TemplateRoleDeleted,
				},
			}),
			AllRead: makeTemplate(t, adminCli, newOrg.ID, codersdk.UpdateTemplateACL{
				GroupPerms: map[string]codersdk.TemplateRole{
					newOrg.ID.String(): codersdk.TemplateRoleUse,
				},
			}),
			UserACL: makeTemplate(t, adminCli, newOrg.ID, codersdk.UpdateTemplateACL{
				GroupPerms: map[string]codersdk.TemplateRole{
					newOrg.ID.String(): codersdk.TemplateRoleDeleted,
				},
				UserPerms: map[string]codersdk.TemplateRole{
					memberUsr.ID.String(): codersdk.TemplateRoleUse,
				},
			}),
			GroupACL: makeTemplate(t, adminCli, newOrg.ID, codersdk.UpdateTemplateACL{
				GroupPerms: map[string]codersdk.TemplateRole{
					group.ID.String():  codersdk.TemplateRoleUse,
					newOrg.ID.String(): codersdk.TemplateRoleDeleted,
				},
			}),
		}
	}

	// Make 2 organizations
	orgs := []orgSetup{
		makeOrg(t),
		makeOrg(t),
	}

	testTemplateRead := func(t *testing.T, org orgSetup, usr *codersdk.Client, read []codersdk.Template) {
		found, err := usr.TemplatesByOrganization(ctx, org.Org.ID)
		if len(read) == 0 && err != nil {
			require.ErrorContains(t, err, "Resource not found")
			return
		}
		require.NoError(t, err, "failed to get templates")

		exp := make(map[uuid.UUID]codersdk.Template)
		for _, tmpl := range read {
			exp[tmpl.ID] = tmpl
		}

		for _, f := range found {
			if _, ok := exp[f.ID]; !ok {
				t.Errorf("found unexpected template %q", f.Name)
			}
			delete(exp, f.ID)
		}
		require.Len(t, exp, 0, "expected templates not found")
	}

	// nolint:paralleltest
	t.Run("OwnerReadAll", func(t *testing.T) {
		for _, o := range orgs {
			// Owners can read all templates in all orgs
			exp := []codersdk.Template{o.DefaultTemplate, o.AllRead, o.UserACL, o.GroupACL}
			testTemplateRead(t, o, ownerClient, exp)
		}
	})

	// nolint:paralleltest
	t.Run("TemplateAdminReadAll", func(t *testing.T) {
		for _, o := range orgs {
			// Template Admins can read all templates in all orgs
			exp := []codersdk.Template{o.DefaultTemplate, o.AllRead, o.UserACL, o.GroupACL}
			testTemplateRead(t, o, templateAdmin, exp)
		}
	})

	// nolint:paralleltest
	t.Run("OrgAdminReadAllTheirs", func(t *testing.T) {
		for i, o := range orgs {
			cli := o.Admin.Client
			// Only read their own org
			exp := []codersdk.Template{o.DefaultTemplate, o.AllRead, o.UserACL, o.GroupACL}
			testTemplateRead(t, o, cli, exp)

			other := orgs[(i+1)%len(orgs)]
			require.NotEqual(t, other.Org.ID, o.Org.ID, "this test needs at least 2 orgs")
			testTemplateRead(t, other, cli, []codersdk.Template{})
		}
	})

	// nolint:paralleltest
	t.Run("TestMemberNoGroup", func(t *testing.T) {
		for i, o := range orgs {
			cli := o.MemberNoGroup.Client
			// Only read their own org
			exp := []codersdk.Template{o.AllRead, o.UserACL}
			testTemplateRead(t, o, cli, exp)

			other := orgs[(i+1)%len(orgs)]
			require.NotEqual(t, other.Org.ID, o.Org.ID, "this test needs at least 2 orgs")
			testTemplateRead(t, other, cli, []codersdk.Template{})
		}
	})

	// nolint:paralleltest
	t.Run("TestMemberInGroup", func(t *testing.T) {
		for i, o := range orgs {
			cli := o.MemberInGroup.Client
			// Only read their own org
			exp := []codersdk.Template{o.AllRead, o.GroupACL}
			testTemplateRead(t, o, cli, exp)

			other := orgs[(i+1)%len(orgs)]
			require.NotEqual(t, other.Org.ID, o.Org.ID, "this test needs at least 2 orgs")
			testTemplateRead(t, other, cli, []codersdk.Template{})
		}
	})
}
