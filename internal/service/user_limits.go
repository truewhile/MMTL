package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/truewhile/MeBox/internal/repository"
)

const (
	// SettingMaxUsers stores the per-instance user cap in the settings table.
	SettingMaxUsers = "user.max_users"

	// DefaultMaxUsers is used when the setting is missing or invalid.
	DefaultMaxUsers = 20

	// MaxUsersHardCap prevents absurd values from admin input.
	MaxUsersHardCap = 10000
)

var (
	ErrInvalidMaxUsers = errors.New("invalid max users")
)

// LoadMaxUsers reads the configured user cap, falling back to DefaultMaxUsers.
func LoadMaxUsers(ctx context.Context, repo *repository.Container) (int, error) {
	if repo == nil || repo.Setting == nil {
		return DefaultMaxUsers, nil
	}
	raw, err := repo.Setting.Get(ctx, SettingMaxUsers)
	if err != nil {
		return DefaultMaxUsers, err
	}
	return parseMaxUsersSetting(raw), nil
}

func parseMaxUsersSetting(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 {
		return DefaultMaxUsers
	}
	if n > MaxUsersHardCap {
		return MaxUsersHardCap
	}
	return n
}

// ValidateMaxUsers checks an admin-provided cap before persisting it.
func ValidateMaxUsers(n int) error {
	if n < 1 {
		return fmt.Errorf("%w: must be at least 1", ErrInvalidMaxUsers)
	}
	if n > MaxUsersHardCap {
		return fmt.Errorf("%w: must be at most %d", ErrInvalidMaxUsers, MaxUsersHardCap)
	}
	return nil
}

// SaveMaxUsers persists the user cap in settings.
func SaveMaxUsers(ctx context.Context, repo *repository.Container, n int) error {
	if err := ValidateMaxUsers(n); err != nil {
		return err
	}
	if repo == nil || repo.Setting == nil {
		return errors.New("settings repository unavailable")
	}
	return repo.Setting.Set(ctx, SettingMaxUsers, strconv.Itoa(n))
}
