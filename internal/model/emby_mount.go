// Emby 媒体库挂载模型。
//
// 远程 Emby 账号（StrmAccount.Provider = emby_remote）只是一个服务器连接；
// 「挂载」才决定把该服务器的哪个媒体库（View）暴露到本项目的媒体库中。
// 这样同一个 Emby 服务器可以按库选择挂载，且每个挂载独立控制是否由 MMTL
// 代理播放流量。
package model

// EmbyMount 是远程 Emby 服务器上一个媒体库（View）的挂载配置。
type EmbyMount struct {
	Base
	AccountID      string `gorm:"size:36;index" json:"account_id"`   // StrmAccount.ID（provider=emby_remote）
	RemoteViewID   string `gorm:"size:128" json:"remote_view_id"`    // 远程 Emby 的 View Id
	RemoteViewName string `gorm:"size:255" json:"remote_view_name"`  // 远程媒体库原名（展示冗余）
	CollectionType string `gorm:"size:32" json:"collection_type"`    // movies / tvshows / music ...
	Name           string `gorm:"size:255" json:"name,omitempty"`    // 覆盖显示名（可选，默认「账号 · 库名」）
	SortOrder      int    `gorm:"default:0;index" json:"sort_order"` // 手动排序用，越小越靠前
	ProxyPlay      bool   `gorm:"default:false" json:"proxy_play"`   // 该挂载播放流量是否经 MMTL 反向代理
	Enabled        bool   `gorm:"default:true" json:"enabled"`       // 是否在媒体库中展示
}
