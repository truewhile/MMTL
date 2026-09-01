// 远程 Emby 挂载条目 / 媒体库的识别与只读标记。
// 与后端 internal/service/emby_remote_ids.go 的 ID 伪装协议保持一致：
// embyremote~{accountID}~{remoteID}。
export function isRemoteEmbyID(id?: string | null): boolean {
  return Boolean(id && id.startsWith('embyremote~'))
}