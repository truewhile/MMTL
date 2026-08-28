import { useEffect, useState } from 'react'
import toast from 'react-hot-toast'
import {
  Activity,
  ArrowRightLeft,
  CheckCircle2,
  Database,
  HardDrive,
  HelpCircle,
  Loader2,
  RefreshCw,
  Save,
  Server,
  ShieldCheck,
  Zap,
} from 'lucide-react'

import {
  adminAPI,
  type DatabaseConnectionPayload,
  type DatabaseMigrationResult,
  type DatabaseStatus,
  type PostgresTestResult,
} from '../api/admin'
import { confirmAction } from '../components/confirmAction'

export function DatabaseSettingsPanel() {
  const [status, setStatus] = useState<DatabaseStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [mode, setMode] = useState<'form' | 'dsn'>('form')

  // 表单状态
  const [formData, setFormData] = useState<DatabaseConnectionPayload>({
    host: '127.0.0.1',
    port: 5432,
    user: 'postgres',
    password: '',
    dbname: 'mmtl',
    sslmode: 'disable',
    dsn: '',
  })

  // 测试与操作状态
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<PostgresTestResult | null>(null)
  const [migrating, setMigrating] = useState(false)
  const [migrationResult, setMigrationResult] = useState<DatabaseMigrationResult | null>(null)
  const [saving, setSaving] = useState(false)

  const refreshStatus = () => {
    setLoading(true)
    return adminAPI
      .getDatabaseStatus()
      .then(setStatus)
      .catch((err) => toast.error('获取数据库状态失败: ' + (err.message || '网络错误')))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    refreshStatus().catch(() => undefined)
  }, [])

  const getPayload = (): DatabaseConnectionPayload => {
    if (mode === 'dsn') {
      return { type: 'postgres', dsn: formData.dsn?.trim() || '' }
    }
    return {
      type: 'postgres',
      host: formData.host?.trim() || '',
      port: Number(formData.port) || 5432,
      user: formData.user?.trim() || '',
      password: formData.password || '',
      dbname: formData.dbname?.trim() || 'mmtl',
      sslmode: formData.sslmode || 'disable',
    }
  }

  const handleTestConnection = async () => {
    const payload = getPayload()
    if (mode === 'form' && (!payload.host || !payload.user)) {
      toast.error('请填写 PostgreSQL 主机和用户名')
      return
    }
    if (mode === 'dsn' && !payload.dsn) {
      toast.error('请填写 PostgreSQL DSN')
      return
    }

    setTesting(true)
    setTestResult(null)
    try {
      const res = await adminAPI.testDatabaseConnection(payload)
      setTestResult(res)
      if (res.success) {
        toast.success(`连接成功！延迟: ${res.latency_ms}ms`)
      } else {
        toast.error(res.error || '连接失败')
      }
    } catch (err: any) {
      const errorMsg = err.response?.data?.error || err.message || '测试连接异常'
      setTestResult({ success: false, error: errorMsg })
      toast.error(errorMsg)
    } finally {
      setTesting(false)
    }
  }

  const handleMigrate = async () => {
    const payload = getPayload()
    if (status?.type === 'postgres') {
      const ok = await confirmAction({
        title: '覆盖/同步确认',
        message: '当前已经处于 PostgreSQL 模式，继续迁移将覆盖/合并目标库的数据，确定继续吗？',
      })
      if (!ok) return
    } else {
      const ok = await confirmAction({
        title: '开始数据库迁移',
        message: '即将把当前 SQLite 数据库中的所有媒体、用户、播放记录、设置等全量迁移到目标 PostgreSQL 数据库。确定开始吗？',
      })
      if (!ok) return
    }

    setMigrating(true)
    setMigrationResult(null)
    try {
      const res = await adminAPI.migrateDatabase(payload)
      setMigrationResult(res)
      if (res.success) {
        toast.success(`数据迁移完成！共迁移 ${res.total_rows} 条记录`)
      } else {
        toast.error(res.error || '数据迁移失败')
      }
    } catch (err: any) {
      const errorMsg = err.response?.data?.error || err.message || '迁移发生错误'
      setMigrationResult({ success: false, total_rows: 0, duration_ms: 0, error: errorMsg })
      toast.error(errorMsg)
    } finally {
      setMigrating(false)
    }
  }

  const handleSaveAndSwitch = async () => {
    const payload = getPayload()
    const ok = await confirmAction({
      title: '切换数据库',
      message: '保存后系统配置将更新为使用 PostgreSQL。需要重启 MMTL 服务使新数据库生效。确定保存吗？',
    })
    if (!ok) return

    setSaving(true)
    try {
      const res = await adminAPI.saveDatabaseConfig(payload)
      toast.success(res.message || '数据库配置已保存，请重启服务生效')
      refreshStatus()
    } catch (err: any) {
      toast.error(err.response?.data?.error || err.message || '保存配置失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      {/* 头部标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Database className="h-6 w-6 text-brand-500" />
          <div>
            <h2 className="font-display text-lg font-semibold text-ink-600">数据库设置与迁移</h2>
            <p className="text-xs text-ink-50">
              管理系统底层数据库，支持在 SQLite（本地嵌入式）与 PostgreSQL（高性能关系库）之间平滑切换与数据迁移
            </p>
          </div>
        </div>
        <button
          onClick={refreshStatus}
          disabled={loading}
          className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-sand-200/50 px-3 py-1.5 text-xs text-ink-100 hover:bg-sand-200 disabled:opacity-50"
        >
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          刷新状态
        </button>
      </div>

      {/* 当前数据库状态卡片 */}
      <div className="glass-panel p-5 space-y-4">
        <div className="flex items-center justify-between border-b border-gray-200 pb-3">
          <div className="flex items-center gap-2">
            <Server size={18} className="text-brand-500" />
            <span className="font-medium text-sm text-ink-600">当前运行引擎</span>
          </div>
          <div className="flex items-center gap-2">
            <span
              className={
                'inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ' +
                (status?.type === 'postgres'
                  ? 'bg-blue-500/10 text-blue-400 border border-blue-500/20'
                  : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20')
              }
            >
              <Zap size={12} />
              {status?.type === 'postgres' ? 'PostgreSQL' : 'SQLite (WAL 优化)'}
            </span>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs">
          <div className="space-y-1 rounded-lg bg-sand-200/30 p-3">
            <p className="text-sand-500">存储位置 / 连接</p>
            <p className="font-mono text-ink-600 break-all">
              {status?.type === 'postgres'
                ? status.dsn || '配置的 PostgreSQL 实例'
                : status?.db_path || './data/mmtl.db'}
            </p>
          </div>
          <div className="space-y-1 rounded-lg bg-sand-200/30 p-3">
            <p className="text-sand-500">连接池活跃 / 最大</p>
            <p className="font-mono text-ink-600">
              活跃: {status?.in_use ?? 0} · 空闲: {status?.idle ?? 0} · 上限:{' '}
              {status?.max_open_conns ?? 16}
            </p>
          </div>
          <div className="space-y-1 rounded-lg bg-sand-200/30 p-3">
            <p className="text-sand-500">核心表记录概览</p>
            <p className="text-ink-600">
              媒体: {status?.table_counts?.media ?? 0} · 用户: {status?.table_counts?.users ?? 0} ·
              播放记录: {status?.table_counts?.playback_histories ?? 0}
            </p>
          </div>
        </div>
      </div>

      {/* 配置 PostgreSQL */}
      <div className="glass-panel p-5 space-y-5">
        <div className="flex items-center justify-between border-b border-gray-200 pb-3">
          <div className="flex items-center gap-2">
            <HardDrive size={18} className="text-brand-500" />
            <span className="font-medium text-sm text-ink-600">配置目标 PostgreSQL</span>
          </div>
          <div className="flex rounded-lg bg-sand-200/40 p-0.5 text-xs">
            <button
              onClick={() => setMode('form')}
              className={
                'rounded-md px-3 py-1 transition ' +
                (mode === 'form'
                  ? 'bg-brand-500 text-white font-medium shadow-sm'
                  : 'text-sand-500 hover:text-ink-600')
              }
            >
              分段表单
            </button>
            <button
              onClick={() => setMode('dsn')}
              className={
                'rounded-md px-3 py-1 transition ' +
                (mode === 'dsn'
                  ? 'bg-brand-500 text-white font-medium shadow-sm'
                  : 'text-sand-500 hover:text-ink-600')
              }
            >
              完整 DSN
            </button>
          </div>
        </div>

        {mode === 'form' ? (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">主机地址 (Host)</label>
              <input
                type="text"
                value={formData.host || ''}
                onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                placeholder="例如 127.0.0.1 或 postgres"
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">端口 (Port)</label>
              <input
                type="number"
                value={formData.port || 5432}
                onChange={(e) => setFormData({ ...formData, port: Number(e.target.value) })}
                placeholder="5432"
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">数据库名 (Database)</label>
              <input
                type="text"
                value={formData.dbname || ''}
                onChange={(e) => setFormData({ ...formData, dbname: e.target.value })}
                placeholder="mmtl"
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">用户名 (User)</label>
              <input
                type="text"
                value={formData.user || ''}
                onChange={(e) => setFormData({ ...formData, user: e.target.value })}
                placeholder="postgres"
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">密码 (Password)</label>
              <input
                type="password"
                value={formData.password || ''}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                placeholder="••••••••"
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-sand-500">SSL 模式 (SSL Mode)</label>
              <select
                value={formData.sslmode || 'disable'}
                onChange={(e) => setFormData({ ...formData, sslmode: e.target.value })}
                className="w-full rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 focus:border-brand-500 focus:outline-none"
              >
                <option value="disable">disable (关闭 SSL)</option>
                <option value="require">require (强制 SSL)</option>
                <option value="verify-ca">verify-ca (验证 CA)</option>
                <option value="verify-full">verify-full (严格验证证书与主机名)</option>
              </select>
            </div>
          </div>
        ) : (
          <div className="space-y-1.5 text-sm">
            <label className="text-xs font-medium text-sand-500">
              PostgreSQL DSN 字符串 (URL 格式)
            </label>
            <input
              type="text"
              value={formData.dsn || ''}
              onChange={(e) => setFormData({ ...formData, dsn: e.target.value })}
              placeholder="postgres://user:password@127.0.0.1:5432/mmtl?sslmode=disable"
              className="w-full font-mono text-xs rounded-lg border border-gray-200 bg-sand-200/40 px-3 py-2 text-ink-600 placeholder:text-gray-400 focus:border-brand-500 focus:outline-none"
            />
          </div>
        )}

        {/* 测试结果卡片 */}
        {testResult && (
          <div
            className={
              'flex items-start gap-2.5 rounded-lg p-3.5 text-xs ' +
              (testResult.success
                ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400'
                : 'bg-rose-500/10 border border-rose-500/20 text-rose-400')
            }
          >
            {testResult.success ? (
              <CheckCircle2 size={16} className="mt-0.5 shrink-0" />
            ) : (
              <HelpCircle size={16} className="mt-0.5 shrink-0" />
            )}
            <div className="space-y-0.5">
              <p className="font-semibold">
                {testResult.success ? `连接测试通过 (${testResult.latency_ms} ms)` : '连接测试未通过'}
              </p>
              {testResult.version && <p className="text-ink-100">{testResult.version}</p>}
              {testResult.error && <p className="text-rose-300 font-mono">{testResult.error}</p>}
            </div>
          </div>
        )}

        {/* 迁移结果卡片 */}
        {migrationResult && (
          <div
            className={
              'rounded-lg p-3.5 text-xs space-y-2 ' +
              (migrationResult.success
                ? 'bg-blue-500/10 border border-blue-500/20 text-blue-400'
                : 'bg-rose-500/10 border border-rose-500/20 text-rose-400')
            }
          >
            <div className="flex items-center gap-2 font-semibold">
              <ShieldCheck size={16} />
              <span>{migrationResult.message || (migrationResult.success ? '迁移完成' : '迁移失败')}</span>
              {migrationResult.success && (
                <span className="text-sand-500 text-[11px]">
                  (耗时: {migrationResult.duration_ms} ms)
                </span>
              )}
            </div>
            {migrationResult.table_rows && Object.keys(migrationResult.table_rows).length > 0 && (
              <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 pt-1 font-mono text-[11px] text-ink-100">
                {Object.entries(migrationResult.table_rows).map(([tbl, count]) => (
                  <div key={tbl} className="rounded bg-sand-200/40 px-2 py-1 flex justify-between">
                    <span>{tbl}:</span>
                    <span className="font-bold text-brand-400">{count} 条</span>
                  </div>
                ))}
              </div>
            )}
            {migrationResult.error && (
              <p className="text-rose-300 font-mono">{migrationResult.error}</p>
            )}
          </div>
        )}

        {/* 操作按钮区 */}
        <div className="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-gray-200">
          <button
            type="button"
            onClick={handleTestConnection}
            disabled={testing || migrating || saving}
            className="flex items-center gap-1.5 rounded-lg border border-gray-200 bg-sand-200/60 px-4 py-2 text-xs font-medium text-ink-600 hover:bg-sand-200 disabled:opacity-50"
          >
            {testing ? <Loader2 size={14} className="animate-spin" /> : <Activity size={14} />}
            测试连接
          </button>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleMigrate}
              disabled={testing || migrating || saving}
              className="flex items-center gap-1.5 rounded-lg border border-primary-500/30 bg-primary-500/10 px-4 py-2 text-xs font-medium text-brand-400 hover:bg-primary-500/20 disabled:opacity-50"
            >
              {migrating ? (
                <Loader2 size={14} className="animate-spin" />
              ) : (
                <ArrowRightLeft size={14} />
              )}
              一键数据迁移到 PostgreSQL
            </button>

            <button
              type="button"
              onClick={handleSaveAndSwitch}
              disabled={testing || migrating || saving}
              className="neon-button text-xs disabled:opacity-50"
            >
              {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
              保存并切换
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
