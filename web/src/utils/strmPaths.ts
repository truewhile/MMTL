/** 取路径最后一级目录/文件名（兼容 / 与 \\）。 */
export function lastPathSegment(path: string): string {
  const trimmed = path.trim().replace(/[\\/]+$/, '')
  if (!trimmed) return ''
  const parts = trimmed.split(/[/\\]/).filter(Boolean)
  return parts[parts.length - 1] ?? ''
}

function pathSeparator(path: string): '/' | '\\' {
  return path.includes('\\') ? '\\' : '/'
}

function normalizeSlashes(path: string): string {
  return path.replace(/\\/g, '/').replace(/\/+$/, '')
}

/** 去掉 local 末尾与 prevRemoteTail 匹配的自动拼接段。 */
function stripRemoteTail(localPath: string, prevRemoteTail: string): string {
  if (!prevRemoteTail) return localPath.trim()
  const trimmed = localPath.trim()
  if (!trimmed) return ''
  const norm = normalizeSlashes(trimmed)
  const suffix = '/' + prevRemoteTail
  if (norm.endsWith(suffix)) {
    const stripped = norm.slice(0, -suffix.length)
    if (!stripped) return ''
    return stripped.split('/').join(pathSeparator(trimmed))
  }
  if (norm === prevRemoteTail) return ''
  return trimmed
}

/** 将远端最后一级目录名拼到本地输出目录末尾；远端变更时替换旧尾段。 */
export function syncLocalPathWithRemote(
  localPath: string,
  remotePath: string,
  prevRemoteTail = '',
): { localPath: string; remoteTail: string } {
  const remoteTail = lastPathSegment(remotePath)
  const base = stripRemoteTail(localPath, prevRemoteTail)
  if (!remoteTail) {
    return { localPath: base || localPath.trim(), remoteTail: '' }
  }
  if (!base) {
    return { localPath: localPath.trim(), remoteTail }
  }
  const sep = pathSeparator(base)
  const normBase = normalizeSlashes(base)
  if (normBase.endsWith('/' + remoteTail)) {
    return { localPath: base, remoteTail }
  }
  return { localPath: `${base}${sep}${remoteTail}`, remoteTail }
}
