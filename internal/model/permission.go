// Package model 定义权限相关的数据模型。
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserPermission 定义用户细粒度权限（19项）。
// 默认开启（6项）：CanViewDashboard, CanPlayMedia, CanCast, CanExternalPlayer, CanFavorite, CanViewHistory
// 默认关闭（13项）：其他权限需要管理员分配
type UserPermission struct {
	ID     string `gorm:"primaryKey;size:36" json:"id"`
	UserID string `gorm:"uniqueIndex;size:36;not null" json:"user_id"`

	// 默认开启（6项）- Basic
	CanViewDashboard  bool `gorm:"default:true" json:"can_view_dashboard"`
	CanPlayMedia      bool `gorm:"default:true" json:"can_play_media"`
	CanCast           bool `gorm:"default:true" json:"can_cast"`
	CanExternalPlayer bool `gorm:"default:true" json:"can_external_player"`
	CanFavorite       bool `gorm:"default:true" json:"can_favorite"`
	CanViewHistory    bool `gorm:"default:true" json:"can_view_history"`

	// 默认关闭（13项）- Advanced
	CanEditMedia           bool `gorm:"default:false" json:"can_edit_media"`
	CanRescrape            bool `gorm:"default:false" json:"can_rescrape"`
	CanUseAI               bool `gorm:"default:false" json:"can_use_ai"`
	CanCaptureFrames       bool `gorm:"default:false" json:"can_capture_frames"`
	CanManageDownloads     bool `gorm:"default:false" json:"can_manage_downloads"`
	CanManageSubscriptions bool `gorm:"default:false" json:"can_manage_subscriptions"`
	CanManageSites         bool `gorm:"default:false" json:"can_manage_sites"`
	CanUseAIAssistant      bool `gorm:"default:false" json:"can_use_ai_assistant"`
	CanManageUsers         bool `gorm:"default:false" json:"can_manage_users"`
	CanManageFiles         bool `gorm:"default:false" json:"can_manage_files"`
	CanManageStrm          bool `gorm:"default:false" json:"can_manage_strm"`
	CanAccessSettings      bool `gorm:"default:false" json:"can_access_settings"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate 生成 UUID。
func (p *UserPermission) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return nil
}

// NewDefaultPermission 创建带有默认权限的 UserPermission。
func NewDefaultPermission(userID string) *UserPermission {
	return &UserPermission{
		ID:                     uuid.NewString(),
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

// PermissionMap 将权限结构转换为 map[string]bool 便于检查。
func (p *UserPermission) PermissionMap() map[string]bool {
	return map[string]bool{
		"can_view_dashboard":       p.CanViewDashboard,
		"can_play_media":           p.CanPlayMedia,
		"can_cast":                 p.CanCast,
		"can_external_player":      p.CanExternalPlayer,
		"can_favorite":             p.CanFavorite,
		"can_view_history":         p.CanViewHistory,
		"can_edit_media":           p.CanEditMedia,
		"can_rescrape":             p.CanRescrape,
		"can_use_ai":               p.CanUseAI,
		"can_capture_frames":       p.CanCaptureFrames,
		"can_manage_downloads":     p.CanManageDownloads,
		"can_manage_subscriptions": p.CanManageSubscriptions,
		"can_manage_sites":         p.CanManageSites,
		"can_use_ai_assistant":     p.CanUseAIAssistant,
		"can_manage_users":         p.CanManageUsers,
		"can_manage_files":         p.CanManageFiles,
		"can_manage_strm":          p.CanManageStrm,
		"can_access_settings":      p.CanAccessSettings,
	}
}
