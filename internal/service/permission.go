// Package service — per-user feature toggles.
//
// PermissionService persists model.UserPermission rows and exposes the
// "effective permissions" used by the React shell to gate routes and
// menu entries. Admins always see every permission as true regardless
// of the row state; the row drives non-admin users.
package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
)

// PermissionService manages user permissions.
type PermissionService struct {
	log  *zap.Logger
	repo *repository.Container
}

// NewPermissionService is the constructor.
func NewPermissionService(log *zap.Logger, repo *repository.Container) *PermissionService {
	return &PermissionService{log: log, repo: repo}
}

// Defaults returns a non-admin's default permission set.
func DefaultPermissions(userID string) *model.UserPermission {
	return &model.UserPermission{
		UserID:                 userID,
		CanViewDashboard:       true,
		CanPlayMedia:           true,
		CanCast:                true,
		CanExternalPlayer:      true,
		CanFavorite:            true,
		CanViewHistory:         true,
		CanEditMedia:           false,
		CanRescrape:            false,
		CanUseAI:               false,
		CanCaptureFrames:       false,
		CanManageDownloads:     false,
		CanManageSubscriptions: false,
		CanManageSites:         false,
		CanUseAIAssistant:      false,
		CanManageUsers:         false,
		CanManageFiles:         false,
		CanManageStrm:          false,
		CanAccessSettings:      false,
	}
}

// adminGrant returns the all-true permission set for admin users.
func adminGrant(userID string) *model.UserPermission {
	return &model.UserPermission{
		UserID:                 userID,
		CanViewDashboard:       true,
		CanPlayMedia:           true,
		CanCast:                true,
		CanExternalPlayer:      true,
		CanFavorite:            true,
		CanViewHistory:         true,
		CanEditMedia:           true,
		CanRescrape:            true,
		CanUseAI:               true,
		CanCaptureFrames:       true,
		CanManageDownloads:     true,
		CanManageSubscriptions: true,
		CanManageSites:         true,
		CanUseAIAssistant:      true,
		CanManageUsers:         true,
		CanManageFiles:         true,
		CanManageStrm:          true,
		CanAccessSettings:      true,
	}
}

func FallbackPermissions(userID, role string) *model.UserPermission {
	if role == "admin" {
		return adminGrant(userID)
	}
	return DefaultPermissions(userID)
}

// Effective returns the permission set the React UI should consume.
// Admins skip the table entirely and get a synthetic all-grant row.
func (s *PermissionService) Effective(ctx context.Context, userID string) (*model.UserPermission, error) {
	u, err := s.repo.User.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}
	if u.Role == "admin" {
		return adminGrant(userID), nil
	}
	row, err := s.repo.Permission.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row != nil {
		return row, nil
	}
	// Seed defaults on first read so subsequent updates have a row to
	// patch.
	def := DefaultPermissions(userID)
	if err := s.repo.Permission.Upsert(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// Save persists the user permission patch (admin only — caller checks).
func (s *PermissionService) Save(ctx context.Context, userID string, in *model.UserPermission) error {
	in.UserID = userID
	return s.repo.Permission.Upsert(ctx, in)
}

// EnsureForUser guarantees a permission row exists for the given user.
// If one already exists it is a no-op; otherwise a default row is created.
func (s *PermissionService) EnsureForUser(ctx context.Context, userID string) (*model.UserPermission, error) {
	row, err := s.repo.Permission.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row != nil {
		return row, nil
	}
	def := DefaultPermissions(userID)
	if err := s.repo.Permission.Upsert(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

// Reset reverts to the non-admin defaults.
func (s *PermissionService) Reset(ctx context.Context, userID string) (*model.UserPermission, error) {
	def := DefaultPermissions(userID)
	if err := s.repo.Permission.Upsert(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}
