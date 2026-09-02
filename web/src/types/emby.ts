// Emby 远程挂载类型定义（与后端 internal/model/emby_mount.go 对应）。
export interface EmbyMount {
  id: string
  account_id: string
  remote_view_id: string
  remote_view_name: string
  collection_type: string
  name: string
  sort_order?: number
  proxy_play: boolean
  enabled: boolean
  created_at: string
  updated_at: string
  /** 所属账号名（接口侧拼装） */
  account_name?: string
}

export interface EmbyMountInput {
  account_id: string
  views: EmbyViewInput[]
}

export interface EmbyViewInput {
  remote_view_id: string
  remote_view_name: string
  collection_type: string
  name?: string
  proxy_play: boolean
}

export interface RemoteEmbyView {
  remote_view_id: string
  remote_view_name: string
  collection_type: string
  child_count: number
  already_mounted: boolean
}