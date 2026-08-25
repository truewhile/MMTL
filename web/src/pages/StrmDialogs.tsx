import { FormEvent, useEffect, useRef, useState } from 'react'
import toast from 'react-hot-toast'
import {
  ChevronRight,
  Cloud,
  ExternalLink,
  FolderPlus,
  HardDrive,
  Loader2,
  QrCode,
  RefreshCw,
  Search,
  X,
} from 'lucide-react'

import QRCode from 'qrcode'

import { strmAPI, type Strm115Source, type StrmRemoteEntry } from '../api/strm'
import type { SettingDef } from './SettingsRow'
import { SettingRow } from './SettingsRow'
import type { StrmAccount, StrmSyncPath, StrmSyncPathInput, StrmProvider } from '../types/strm'
import { STRM_PROVIDER_LABELS } from '../types/strm'
import { apiErrorMessage } from './StrmManagePage'
import { LocalDirBrowserDialog } from '../components/LocalDirBrowserDialog'

// ─── 弹框外壳 ────────────────────────────────────────────────────────────────

function DialogShell({
  title,
  subtitle,
  children,
  onClose,
  wide,
}: {
  title: string
  subtitle?: string
  children: React.ReactNode
  onClose: () => void
  wide?: boolean
}) {
  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/35 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        className={
          'flex max-h-[88vh] w-full flex-col overflow-hidden rounded-3xl border border-white/70 bg-white shadow-2xl ' +
          (wide ? 'max-w-3xl' : 'max-w-lg')
        }
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4">
          <div>
            <h3 className="font-display text-lg font-bold text-ink-600">{title}</h3>
            {subtitle && <p className="text-xs text-sand-500">{subtitle}</p>}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl p-1.5 text-ink-50 transition hover:bg-gray-100 hover:text-ink-600"
            title="关闭"
          >
            <X size={20} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-6">{children}</div>
      </div>
    </div>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1">
      <span className="text-sm font-medium text-ink-100">{label}</span>
      {children}
      {hint && <span className="block text-xs text-sand-500">{hint}</span>}
    </label>
  )
}

const inputCls = 'input-base w-full'

// ─── 添加/编辑网盘账号 ────────────────────────────────────────────────────────

const PROVIDER_OPTIONS: { provider: StrmProvider; label: string; desc: string }[] = [
  { provider: 'cloud115', label: '115 网盘', desc: '开放平台 OAuth 授权（扫码/网页）' },
  { provider: 'clouddrive2', label: 'CloudDrive2', desc: 'WebDAV 桥接多种网盘' },
  { provider: 'openlist', label: 'OpenList', desc: 'OpenList / AList 兼容服务' },
]

export function StrmAccountDialog({
  existing,
  onClose,
  onSaved,
}: {
  existing: StrmAccount | null
  onClose: () => void
  onSaved: () => void
}) {
  const [provider, setProvider] = useState<StrmProvider>(existing?.provider ?? 'cloud115')
  const [name, setName] = useState(existing?.name ?? '')
  const [enabled, setEnabled] = useState(existing?.enabled ?? true)
  const [url, setUrl] = useState('')
  const [server, setServer] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const [saving, setSaving] = useState(false)

  const buildConfig = (): Record<string, string> => {
    switch (provider) {
      case 'clouddrive2':
        return {
          ...(url ? { url } : {}),
          ...(username ? { username } : {}),
          ...(password ? { password } : {}),
        }
      case 'openlist':
        return {
          ...(server ? { server } : {}),
          ...(username ? { username } : {}),
          ...(password ? { password } : {}),
          ...(token ? { token } : {}),
        }
      default:
        return {}
    }
  }

  const canSave = () => {
    switch (provider) {
      case 'clouddrive2':
        return url !== ''
      case 'openlist':
        return server !== '' && (token !== '' || password !== '')
      default:
        return false
    }
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setSaving(true)
    try {
      if (existing) {
        await strmAPI.updateAccount(existing.id, {
          name,
          provider: existing.provider,
          enabled,
          config: existing.has_credential && !Object.keys(buildConfig()).length ? {} : buildConfig(),
        })
        toast.success('网盘账号已更新')
      } else {
        if (!canSave()) {
          toast.error('请先完成凭据配置')
          return
        }
        await strmAPI.createAccount({ name, provider, config: buildConfig() })
        toast.success('网盘账号已添加')
      }
      onSaved()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <DialogShell
      title={existing ? '编辑网盘账号' : '添加网盘账号'}
      subtitle="115 开放平台授权（内置应用目录 / 中继 / 第三方服务）；CloudDrive2 / OpenList 填写服务地址与凭据"
      onClose={onClose}
      wide
    >
      <form onSubmit={submit} className="space-y-5">
        {/* 提供方选择 */}
        {!existing && (
          <div className="grid grid-cols-3 gap-3">
            {PROVIDER_OPTIONS.map((option) => {
              const Icon = option.provider === 'cloud115' ? QrCode : option.provider === 'clouddrive2' ? HardDrive : FolderPlus
              const active = provider === option.provider
              return (
                <button
                  key={option.provider}
                  type="button"
                  onClick={() => setProvider(option.provider)}
                  className={
                    'rounded-2xl border-2 p-3 text-left transition ' +
                    (active ? 'border-brand-400 bg-brand-50' : 'border-gray-100 bg-white hover:border-gray-200')
                  }
                >
                  <Icon size={20} className={active ? 'text-brand-500' : 'text-sand-500'} />
                  <p className="mt-1.5 text-sm font-bold text-ink-600">{option.label}</p>
                  <p className="text-[11px] text-sand-500">{option.desc}</p>
                </button>
              )
            })}
          </div>
        )}

        <Field label="账号名称">
          <input
            className={inputCls}
            value={name}
            placeholder={STRM_PROVIDER_LABELS[provider]}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>

        {provider === 'cloud115' && (
          <Strm115AuthPanel
            existing={existing}
            accountName={name}
            onAuthed={(account) => {
              toast.success(`「${account.name}」授权成功`)
              onSaved()
            }}
          />
        )}

        {provider === 'clouddrive2' && (
          <Field label="WebDAV 地址" hint="例如 http://192.168.1.10:19798/dav">
            <input className={inputCls} value={url} placeholder="http://host:19798/dav" onChange={(e) => setUrl(e.target.value)} />
          </Field>
        )}

        {provider === 'openlist' && (
          <Field label="服务器地址" hint="OpenList / AList 服务地址，如 http://192.168.1.10:5244">
            <input className={inputCls} value={server} placeholder="http://host:5244" onChange={(e) => setServer(e.target.value)} />
          </Field>
        )}

        {provider !== 'cloud115' && (
          <div className="grid grid-cols-2 gap-3">
            <Field label="用户名">
              <input className={inputCls} value={username} onChange={(e) => setUsername(e.target.value)} />
            </Field>
            <Field label="密码">
              <input className={inputCls} type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
          </div>
        )}

        {provider === 'openlist' && (
          <Field label="Token（可选，优先于密码）">
            <input className={inputCls} value={token} onChange={(e) => setToken(e.target.value)} />
          </Field>
        )}

        {existing && provider !== 'cloud115' && (
          <label className="flex cursor-pointer items-center gap-2 text-sm text-ink-100">
            <input
              type="checkbox"
              className="h-4 w-4 accent-primary-400"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
            />
            启用该账号
          </label>
        )}

        {provider !== 'cloud115' && (
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="rounded-xl border border-gray-200 px-4 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50">
              取消
            </button>
            <button type="submit" disabled={saving} className="neon-button disabled:opacity-50">
              {saving ? <Loader2 size={16} className="animate-spin" /> : <Cloud size={16} />}
              保存
            </button>
          </div>
        )}
      </form>
    </DialogShell>
  )
}

// ─── 115 登录二维码（canvas 渲染；115 返回的是扫码页面链接而非图片） ───────────

// QR 尺寸：物理像素 = CSS 像素（避免 canvas 被 CSS 缩放导致裁切/模糊）
const QR_SIZE = 176

function QrCanvas({ content }: { content: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || !content) return
    QRCode.toCanvas(canvas, content, {
      width: QR_SIZE,
      margin: 2,
      errorCorrectionLevel: 'M',
      color: { dark: '#1f2937', light: '#ffffff' },
    })
      .then(() => setError(''))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : '二维码生成失败'))
  }, [content])

  return (
    <div className="relative h-44 w-44 shrink-0 rounded-xl border border-gray-200 bg-white">
      <canvas
        ref={canvasRef}
        width={QR_SIZE}
        height={QR_SIZE}
        role="img"
        aria-label="115 授权二维码"
        style={{ width: QR_SIZE, height: QR_SIZE }}
      />
      {error && (
        <div className="absolute inset-0 grid place-items-center rounded-xl bg-gray-50 p-3 text-center text-xs text-rose-500">
          {error}
        </div>
      )}
    </div>
  )
}

// ─── 115 开放平台授权面板 ─────────────────────────────────────────────────────

type AuthSourceKey = 'built_in_appid' | 'custom_appid' | 'built_in_relay' | 'third_party_service'

const AUTH_SOURCE_OPTIONS: { key: AuthSourceKey; label: string; desc: string; needsRelayKey?: boolean }[] = [
  { key: 'built_in_appid', label: '官方应用目录', desc: '设备码扫码授权，无需回跳' },
  { key: 'custom_appid', label: '自定义 APP ID', desc: '使用自己申请的开放平台应用' },
  { key: 'built_in_relay', label: '中继授权', desc: 'QMediaSync / MQFamily 中继', needsRelayKey: true },
  { key: 'third_party_service', label: '第三方服务', desc: 'MoviePilot / CloudDrive' },
]

function Strm115AuthPanel({
  existing,
  accountName,
  onAuthed,
}: {
  existing: StrmAccount | null
  accountName: string
  onAuthed: (account: StrmAccount) => void
}) {
  const [sources, setSources] = useState<{ built_in: Strm115Source[]; relay: Strm115Source[]; third_party: Strm115Source[] } | null>(null)
  const [relayKeyConfigured, setRelayKeyConfigured] = useState(false)
  const [authSource, setAuthSource] = useState<AuthSourceKey>('built_in_appid')
  const [thirdParty, setThirdParty] = useState<'moviepilot' | 'clouddrive'>('moviepilot')
  const [relayProvider, setRelayProvider] = useState<'qmediasync' | 'mqfamily'>('qmediasync')
  const [appID, setAppID] = useState('100195125')
  const [appKeyword, setAppKeyword] = useState('')

  const [starting, setStarting] = useState(false)
  const [authUI, setAuthUI] = useState<{ sessionId: string; accountId: string; mode: 'qrcode' | 'url'; authUrl?: string; qrcodeUrl?: string } | null>(null)
  const [authStatus, setAuthStatus] = useState('')

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const stopPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }
  useEffect(() => stopPolling, [])

  useEffect(() => {
    ;(async () => {
      try {
        const data = await strmAPI.list115Sources()
        setSources(data)
        const settings = await strmAPI.getSettings().catch(() => null)
        const key = settings?.['strm.115_relay_key']
        setRelayKeyConfigured(Boolean(key && key.trim()))
      } catch (err) {
        toast.error(apiErrorMessage(err))
      }
    })()
  }, [])

  const filteredApps = (sources?.built_in ?? []).filter(
    (s) => !appKeyword || s.app_name.toLowerCase().includes(appKeyword.toLowerCase()) || s.app_id.includes(appKeyword),
  )

  const buildStartPayload = () => {
    switch (authSource) {
      case 'built_in_appid':
        return { auth_source: 'built_in_appid', app_id: appID }
      case 'custom_appid':
        return { auth_source: 'custom_appid', app_id: appID }
      case 'built_in_relay':
        return { auth_source: 'built_in_relay', provider: relayProvider }
      case 'third_party_service':
        return { auth_source: 'third_party_service', provider: thirdParty }
    }
  }

  const startAuth = async () => {
    setStarting(true)
    setAuthStatus('')
    try {
      // 115 授权需要账号 ID：没有则先创建空凭据账号
      let account = existing
      if (!account) {
        account = await strmAPI.createAccount({ name: accountName || '115 网盘', provider: 'cloud115', config: {} })
      }
      const result = await strmAPI.start115OAuth(account.id, buildStartPayload())
      setAuthUI(
        result.mode === 'qrcode'
          ? { sessionId: result.session_id, accountId: account.id, mode: 'qrcode', qrcodeUrl: result.qrcode?.qrcode }
          : { sessionId: result.session_id, accountId: account.id, mode: 'url', authUrl: result.auth_url },
      )
      stopPolling()
      pollRef.current = setInterval(async () => {
        try {
          const status = await strmAPI.poll115OAuth(account.id, result.session_id)
          setAuthStatus(status.tip)
          if (status.status === 'confirmed') {
            stopPolling()
            const updated = await strmAPI.testAccount(account.id)
            onAuthed(updated)
          }
          if (status.status === 'expired') {
            stopPolling()
            setAuthUI(null)
            toast.error('授权已过期，请重新发起')
          }
        } catch {
          /* 轮询失败等下一次 */
        }
      }, 3000)
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setStarting(false)
    }
  }

  const openAuthWindow = () => {
    if (authUI?.mode === 'url' && authUI.authUrl) {
      window.open(authUI.authUrl, '_blank', 'noopener')
    }
  }

  const resetAuth = () => {
    stopPolling()
    setAuthUI(null)
    setAuthStatus('')
  }

  return (
    <div className="space-y-4 rounded-2xl bg-gray-50 p-4">
      {existing?.has_credential && (
        <p className="rounded-xl bg-emerald-50 px-3 py-2 text-sm text-emerald-600">
          ✓ 该账号已授权；重新授权会替换现有令牌
        </p>
      )}

      {!authUI && (
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-2">
            {AUTH_SOURCE_OPTIONS.map((option) => {
              const active = authSource === option.key
              const disabled = Boolean(option.needsRelayKey && !relayKeyConfigured)
              return (
                <button
                  key={option.key}
                  type="button"
                  disabled={disabled}
                  onClick={() => setAuthSource(option.key)}
                  className={
                    'rounded-xl border-2 p-2.5 text-left transition disabled:cursor-not-allowed disabled:opacity-40 ' +
                    (active ? 'border-brand-400 bg-brand-50' : 'border-gray-100 bg-white hover:border-gray-200')
                  }
                >
                  <p className="text-sm font-bold text-ink-600">{option.label}</p>
                  <p className="text-[11px] text-sand-500">{option.desc}</p>
                  {disabled && <p className="text-[10px] text-rose-400">需在 STRM 设置配置共享密钥</p>}
                </button>
              )
            })}
          </div>

          {authSource === 'built_in_appid' && (
            <div className="space-y-2">
              <Field label="选择官方应用" hint="应用目录来自 QMediaSync 内置；媒体播放器 / 飞牛 / 恒星等">
                <div className="flex items-center gap-2">
                  <Search size={14} className="shrink-0 text-sand-400" />
                  <input className={inputCls} placeholder="搜索应用名称或 ID…" value={appKeyword} onChange={(e) => setAppKeyword(e.target.value)} />
                </div>
                <select className={inputCls} value={appID} onChange={(e) => setAppID(e.target.value)} size={5}>
                  {filteredApps.map((source) => (
                    <option key={source.app_id} value={source.app_id}>
                      {source.display_name}（{source.app_id}）
                    </option>
                  ))}
                </select>
              </Field>
            </div>
          )}
          {authSource === 'custom_appid' && (
            <Field label="自定义 APP ID" hint="使用自己在 115 开放平台申请的应用 ID">
              <input className={inputCls} value={appID} placeholder="100195125" onChange={(e) => setAppID(e.target.value)} />
            </Field>
          )}
          {authSource === 'built_in_relay' && (
            <Field label="中继服务">
              <select className={inputCls} value={relayProvider} onChange={(e) => setRelayProvider(e.target.value as typeof relayProvider)}>
                <option value="qmediasync">QMediaSync（oauth.qmediasync.cn）</option>
                <option value="mqfamily">MQFamily（api.mqfamily.top）</option>
              </select>
            </Field>
          )}
          {authSource === 'third_party_service' && (
            <Field label="第三方授权服务">
              <select className={inputCls} value={thirdParty} onChange={(e) => setThirdParty(e.target.value as typeof thirdParty)}>
                <option value="moviepilot">MoviePilot（https://movie-pilot.org）</option>
                <option value="clouddrive">CloudDrive（redirect115.zhenyunpan.com）</option>
              </select>
            </Field>
          )}

          <button type="button" onClick={startAuth} disabled={starting} className="neon-button disabled:opacity-50">
            {starting ? <Loader2 size={16} className="animate-spin" /> : <QrCode size={16} />}
            {authSource === 'built_in_appid' || authSource === 'custom_appid' ? '获取登录二维码' : '获取授权链接'}
          </button>
        </div>
      )}

      {authUI && (
        <div className="space-y-3">
          {authUI.mode === 'qrcode' && authUI.qrcodeUrl ? (
            <div className="flex items-center gap-4">
              <QrCanvas content={authUI.qrcodeUrl} />
              <div className="space-y-1.5 text-sm">
                <p className="font-medium text-ink-600">{authStatus || '等待扫码…'}</p>
                <p className="text-xs text-sand-500">请使用 115 手机客户端扫码并确认授权，5 分钟内有效</p>
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-ink-100">请点击下方按钮在新窗口完成授权：</p>
              <button type="button" onClick={openAuthWindow} className="neon-button">
                <ExternalLink size={16} />
                打开授权页面
              </button>
              <p className="text-xs text-sand-500">完成后回到此页面等待自动确认（授权成功后可关闭弹窗）</p>
            </div>
          )}
          <div className="flex items-center gap-2 text-sm">
            <Loader2 size={14} className="animate-spin text-brand-500" />
            <span className="text-ink-50">{authStatus}</span>
          </div>
          <button
            type="button"
            onClick={resetAuth}
            className="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-semibold text-ink-100 transition hover:bg-gray-50"
          >
            <RefreshCw size={14} />
            重新发起授权
          </button>
        </div>
      )}
    </div>
  )
}

// ─── STRM 设置 ────────────────────────────────────────────────────────────────

const SETTING_DEFS: SettingDef[] = [
  { key: 'strm.base_url', label: 'STRM 链接基础地址', type: 'text', hint: '生成的 strm 文件指向的播放地址；留空依次自动使用 app.server_url，最后回退到本机地址（http://127.0.0.1:端口）。Emby 在其他设备时请配置局域网/公网地址' },
  { key: 'strm.video_ext', label: '视频扩展名', type: 'text', hint: '逗号分隔，命中即生成 .strm' },
  { key: 'strm.meta_ext', label: '元数据扩展名', type: 'text', hint: '逗号分隔，进入下载/上传队列（nfo/图片/字幕）' },
  { key: 'strm.exclude_name', label: '排除文件名', type: 'text', hint: '逗号分隔，文件名包含任一关键词即跳过' },
  { key: 'strm.min_video_size_mb', label: '最小视频大小(MB)', type: 'number', hint: '小于该大小的视频不生成 STRM，0 表示不限' },
  {
    key: 'strm.add_path',
    label: 'STRM 链接 path 参数',
    type: 'select',
    hint: '1=附带完整远端路径 2=仅文件名 3=不带 path',
    options: [
      { value: '1', label: '完整路径' },
      { value: '2', label: '仅文件名' },
      { value: '3', label: '不带 path' },
    ],
  },
  { key: 'strm.115_relay_key', label: '115 中继授权共享密钥', type: 'text', hint: 'QMediaSync/MQFamily 中继授权的共享 AES 密钥；不配置则中继授权不可用' },
  { key: 'strm.download_threads', label: '下载队列线程数', type: 'number', hint: '元数据下载并发数' },
  { key: 'strm.upload_threads', label: '上传队列线程数', type: 'number', hint: '元数据上传并发数' },
]

export function StrmSettingsDialog({ onClose }: { onClose: () => void }) {
  const [values, setValues] = useState<Record<string, string> | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    strmAPI
      .getSettings()
      .then(setValues)
      .catch((err) => toast.error(apiErrorMessage(err)))
  }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!values) return
    setSaving(true)
    try {
      await strmAPI.updateSettings(values)
      toast.success('STRM 设置已保存')
      onClose()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <DialogShell title="STRM 设置" subtitle="全局默认配置；同步目录可单独覆盖" onClose={onClose} wide>
      {!values ? (
        <div className="flex justify-center py-10 text-ink-50">
          <Loader2 className="animate-spin" />
        </div>
      ) : (
        <form onSubmit={submit} className="space-y-4">
          {SETTING_DEFS.map((def) => (
            <SettingRow
              key={def.key}
              def={def}
              value={values[def.key] ?? ''}
              onChange={(value) => setValues((v) => ({ ...v!, [def.key]: value }))}
            />
          ))}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="rounded-xl border border-gray-200 px-4 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50">
              取消
            </button>
            <button type="submit" disabled={saving} className="neon-button disabled:opacity-50">
              {saving ? <Loader2 size={16} className="animate-spin" /> : null}
              保存设置
            </button>
          </div>
        </form>
      )}
    </DialogShell>
  )
}

// ─── 添加/编辑同步目录 ────────────────────────────────────────────────────────

export function StrmSyncPathDialog({
  accounts,
  existing,
  onClose,
  onSaved,
}: {
  accounts: StrmAccount[]
  existing: StrmSyncPath | null
  onClose: () => void
  onSaved: () => void
}) {
  const [form, setForm] = useState<StrmSyncPathInput>(() => ({
    name: existing?.name ?? '',
    provider: existing?.provider ?? 'cloud115',
    account_id: existing?.account_id ?? '',
    remote_path: existing?.remote_path ?? '',
    local_path: existing?.local_path ?? '',
    strm_base_url: existing?.strm_base_url ?? '',
    video_ext: existing?.video_ext ?? '',
    meta_ext: existing?.meta_ext ?? '',
    exclude_name: existing?.exclude_name ?? '',
    min_video_size_mb: existing?.min_video_size_mb ?? 0,
    add_path: existing?.add_path ?? 1,
    download_meta: existing?.download_meta ?? true,
    upload_meta: existing?.upload_meta ?? false,
    delete_dir: existing?.delete_dir ?? false,
    cron: existing?.cron ?? '',
    enable_cron: existing?.enable_cron ?? false,
    sync_mode: existing?.sync_mode ?? 'incremental',
    enabled: existing?.enabled ?? true,
  }))
  const [saving, setSaving] = useState(false)
  const [browsing, setBrowsing] = useState(false)
  const [browsingLocal, setBrowsingLocal] = useState<null | 'remote_path' | 'local_path'>(null)

  const set = <K extends keyof StrmSyncPathInput>(key: K, value: StrmSyncPathInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }))

  const isLocal = form.provider === 'local'
  const availableAccounts = accounts.filter((a) => a.provider === form.provider && a.has_credential)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setSaving(true)
    try {
      if (existing) {
        await strmAPI.updatePath(existing.id, form)
        toast.success('同步目录已更新')
      } else {
        await strmAPI.createPath(form)
        toast.success('同步目录已添加')
      }
      onSaved()
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <DialogShell
      title={existing ? '编辑同步目录' : '添加同步目录'}
      subtitle="把网盘 / 本地目录的视频生成 .strm 文件到本地输出目录"
      onClose={onClose}
      wide
    >
      <form onSubmit={submit} className="space-y-4">
        <Field label="目录名称">
          <input className={inputCls} value={form.name} placeholder="例如：电影库 115" onChange={(e) => set('name', e.target.value)} />
        </Field>

        <div className="grid gap-3 md:grid-cols-2">
          <Field label="同步类型">
            <select
              className={inputCls}
              value={form.provider}
              onChange={(e) => {
                const provider = e.target.value as StrmProvider
                set('provider', provider)
                set('account_id', '')
              }}
            >
              <option value="cloud115">115 网盘</option>
              <option value="clouddrive2">CloudDrive2</option>
              <option value="openlist">OpenList</option>
              <option value="local">本地目录</option>
            </select>
          </Field>
          {!isLocal && (
            <Field label="网盘账号">
              <select
                className={inputCls}
                value={form.account_id ?? ''}
                onChange={(e) => set('account_id', e.target.value)}
                disabled={availableAccounts.length === 0}
              >
                <option value="">{availableAccounts.length === 0 ? '请先添加已授权的网盘账号' : '请选择账号'}</option>
                {availableAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name}{account.enabled ? '' : '（已停用）'}
                  </option>
                ))}
              </select>
            </Field>
          )}
        </div>

        <div className="space-y-1">
          <div className="flex items-end gap-2">
            <div className="flex-1">
              <Field
                label={isLocal ? '本地源目录' : '远端目录'}
                hint={
                  isLocal
                    ? '扫描该目录下的视频生成 strm'
                    : form.provider === 'cloud115'
                      ? '115 目录 ID（可通过浏览选择）'
                      : '远端路径（可通过浏览选择）'
                }
              >
                <input className={inputCls} value={form.remote_path} placeholder={isLocal ? 'D:\\movies' : '/'} onChange={(e) => set('remote_path', e.target.value)} />
              </Field>
            </div>
            {isLocal && (
              <button
                type="button"
                onClick={() => setBrowsingLocal('remote_path')}
                className="mb-1 rounded-xl border border-gray-200 px-3 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50"
              >
                浏览
              </button>
            )}
            {!isLocal && form.account_id && (
              <button type="button" onClick={() => setBrowsing(true)} className="mb-1 rounded-xl border border-gray-200 px-3 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50">
                浏览
              </button>
            )}
          </div>
        </div>

        <div className="space-y-1">
          <div className="flex items-end gap-2">
            <div className="flex-1">
              <Field label="本地输出目录" hint="生成的 .strm 与下载的元数据写到这里的对应目录结构下">
                <input className={inputCls} value={form.local_path} placeholder="D:\\strm\\movies" onChange={(e) => set('local_path', e.target.value)} />
              </Field>
            </div>
            <button
              type="button"
              onClick={() => setBrowsingLocal('local_path')}
              className="mb-1 rounded-xl border border-gray-200 px-3 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50"
            >
              浏览
            </button>
          </div>
        </div>

        <details className="rounded-2xl border border-gray-100 bg-gray-50/50 p-3">
          <summary className="cursor-pointer text-sm font-semibold text-ink-600">高级选项</summary>
          <div className="mt-3 space-y-3">
            <div className="grid gap-3 md:grid-cols-2">
              <Field label="STRM 基础地址（覆盖全局）" hint="留空使用全局 STRM 设置">
                <input className={inputCls} value={form.strm_base_url ?? ''} placeholder="http://host:port" onChange={(e) => set('strm_base_url', e.target.value)} />
              </Field>
              <Field label="最小视频大小(MB)" hint="0 表示继承全局设置">
                <input
                  className={inputCls}
                  type="number"
                  min={0}
                  value={form.min_video_size_mb ?? 0}
                  onChange={(e) => set('min_video_size_mb', Number(e.target.value))}
                />
              </Field>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <Field label="视频扩展名（覆盖全局）">
                <input className={inputCls} value={form.video_ext ?? ''} placeholder="mkv,mp4,avi" onChange={(e) => set('video_ext', e.target.value)} />
              </Field>
              <Field label="元数据扩展名（覆盖全局）">
                <input className={inputCls} value={form.meta_ext ?? ''} placeholder="nfo,jpg,srt" onChange={(e) => set('meta_ext', e.target.value)} />
              </Field>
              <Field label="排除文件名（覆盖全局）">
                <input className={inputCls} value={form.exclude_name ?? ''} placeholder="sample,trailer" onChange={(e) => set('exclude_name', e.target.value)} />
              </Field>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <Field label="STRM 链接 path 参数">
                <select className={inputCls} value={form.add_path ?? 1} onChange={(e) => set('add_path', Number(e.target.value))}>
                  <option value={1}>完整远端路径</option>
                  <option value={2}>仅文件名</option>
                  <option value={3}>不带 path</option>
                </select>
              </Field>
              <Field label="默认同步模式" hint="定时触发或快速同步时的策略">
                <select className={inputCls} value={form.sync_mode ?? 'incremental'} onChange={(e) => set('sync_mode', e.target.value as 'incremental' | 'full')}>
                  <option value="incremental">增量同步（快速）</option>
                  <option value="full">全量同步（全量校验）</option>
                </select>
              </Field>
              <Field label="定时同步 Cron" hint="5 段表达式，如 0 */6 * * *">
                <input className={inputCls} value={form.cron ?? ''} placeholder="0 */6 * * *" onChange={(e) => set('cron', e.target.value)} />
              </Field>
            </div>
            <div className="grid gap-2 md:grid-cols-2">
              <ToggleRow label="下载元数据" checked={form.download_meta ?? true} onChange={(v) => set('download_meta', v)} />
              <ToggleRow label="上传元数据" checked={form.upload_meta ?? false} onChange={(v) => set('upload_meta', v)} />
              <ToggleRow label="清理空目录" checked={form.delete_dir ?? false} onChange={(v) => set('delete_dir', v)} />
              <ToggleRow label="启用定时同步" checked={form.enable_cron ?? false} onChange={(v) => set('enable_cron', v)} />
              <ToggleRow label="启用该目录" checked={form.enabled ?? true} onChange={(v) => set('enabled', v)} />
            </div>
          </div>
        </details>

        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={onClose} className="rounded-xl border border-gray-200 px-4 py-2 text-sm font-semibold text-ink-100 hover:bg-gray-50">
            取消
          </button>
          <button type="submit" disabled={saving} className="neon-button disabled:opacity-50">
            {saving ? <Loader2 size={16} className="animate-spin" /> : <FolderPlus size={16} />}
            保存
          </button>
        </div>
      </form>

      {browsing && form.account_id && (
        <StrmDirBrowserDialog
          accountId={form.account_id}
          initialDir={form.remote_path || undefined}
          onSelect={(id) => {
            set('remote_path', id)
            setBrowsing(false)
          }}
          onClose={() => setBrowsing(false)}
        />
      )}

      {browsingLocal && (
        <LocalDirBrowserDialog
          initialDir={form[browsingLocal] || undefined}
          onSelect={(path) => {
            set(browsingLocal, path)
            setBrowsingLocal(null)
          }}
          onClose={() => setBrowsingLocal(null)}
        />
      )}
    </DialogShell>
  )
}

function ToggleRow({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-ink-100">
      <input
        type="checkbox"
        className="h-4 w-4 accent-primary-400"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      {label}
    </label>
  )
}

// ─── 远端目录浏览选择器 ───────────────────────────────────────────────────────

function StrmDirBrowserDialog({
  accountId,
  initialDir,
  onSelect,
  onClose,
}: {
  accountId: string
  initialDir?: string
  onSelect: (path: string) => void
  onClose: () => void
}) {
  const [dir, setDir] = useState(initialDir ?? '')
  const [crumbs, setCrumbs] = useState<string[]>([])
  const [entries, setEntries] = useState<StrmRemoteEntry[]>([])
  const [loading, setLoading] = useState(true)

  const load = async (target: string) => {
    setLoading(true)
    try {
      const list = await strmAPI.listRemoteDir(accountId, target)
      setEntries(list)
      setDir(target)
      setCrumbs(target ? target.split('/').filter(Boolean) : [])
    } catch (err) {
      toast.error(apiErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(initialDir ?? '').catch(() => undefined)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accountId])

  const enterDir = (id: string) => load(id).catch(() => undefined)

  return (
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center bg-black/35 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        className="flex max-h-[80vh] w-full max-w-2xl flex-col overflow-hidden rounded-3xl border border-white/70 bg-white shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4">
          <h3 className="font-display text-lg font-bold text-ink-600">选择远端目录</h3>
          <button type="button" onClick={onClose} className="rounded-xl p-1.5 text-ink-50 transition hover:bg-gray-100 hover:text-ink-600">
            <X size={20} />
          </button>
        </div>
        <div className="flex items-center gap-1 border-b border-gray-100 px-6 py-2.5 text-xs text-sand-500">
          <button type="button" className="hover:text-brand-500" onClick={() => enterDir('')}>
            根目录
          </button>
          {crumbs.map((crumb, index) => (
            <span key={crumb + index} className="flex items-center gap-1">
              <ChevronRight size={12} />
              <span>{crumb}</span>
            </span>
          ))}
          <span className="ml-2 text-ink-50">{dir || '（根目录 / 0）'}</span>
        </div>
        <div className="min-h-[260px] flex-1 overflow-y-auto p-3">
          {loading ? (
            <div className="flex justify-center py-10 text-ink-50">
              <Loader2 className="animate-spin" />
            </div>
          ) : entries.length === 0 ? (
            <p className="py-10 text-center text-sm text-sand-500">该目录为空</p>
          ) : (
            <div className="space-y-1">
              {entries.map((entry) => {
                const Icon = entry.is_dir ? FolderPlus : HardDrive
                return (
                  <button
                    key={entry.id}
                    type="button"
                    className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition hover:bg-gray-50"
                    onClick={() => (entry.is_dir ? enterDir(entry.id) : undefined)}
                    onDoubleClick={() => entry.is_dir && enterDir(entry.id)}
                  >
                    <Icon size={16} className={entry.is_dir ? 'text-brand-400' : 'text-sand-400'} />
                    <span className="flex-1 truncate text-ink-600">{entry.name}</span>
                    {entry.is_dir ? (
                      <button
                        type="button"
                        className="rounded-lg border border-brand-200 bg-brand-50 px-2.5 py-1 text-xs font-semibold text-brand-500 hover:bg-brand-100"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelect(entry.id)
                        }}
                      >
                        选择此目录
                      </button>
                    ) : (
                      <span className="text-xs text-sand-400">{formatEntrySize(entry.size)}</span>
                    )}
                  </button>
                )
              })}
            </div>
          )}
        </div>
        <div className="flex items-center justify-between border-t border-gray-100 px-6 py-3">
          <span className="text-xs text-sand-500">双击进入目录，点击「选择此目录」使用该目录 ID / 路径</span>
          <button type="button" className="neon-button" onClick={() => onSelect(dir)} disabled={loading}>
            选择当前目录
          </button>
        </div>
      </div>
    </div>
  )
}

function formatEntrySize(size: number): string {
  if (!size) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[unit]}`
}