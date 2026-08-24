import type { SettingGroup } from './settingsGroupTypes'

// 弹幕配置：对接 dandanplay（弹弹play）协议弹幕源 + 渲染通用功能。
// 来源地址留空时使用官方 https://api.dandanplay.net；填写任何符合
// dandanplay 协议（/api/v2/search/episodes + /api/v2/comment/:id）的服务地址均可。
export const danmakuSettingsGroup: SettingGroup = {
  key: 'danmaku',
  label: '弹幕',
  description: '弹幕来源（dandanplay 协议）与渲染参数（透明度 / 字号 / 显示区域）',
  items: [
    {
      key: 'danmaku.enabled',
      label: '启用弹幕',
      type: 'toggle',
      hint: '关闭后播放页不拉取、不渲染弹幕',
      defaultValue: 'true',
    },
    {
      key: 'danmaku.source',
      label: '弹幕服务地址（dandanplay 协议）',
      type: 'text',
      hint: '留空使用官方 https://api.dandanplay.net。可填写自建或第三方符合 dandanplay 协议的服务地址（含 /api/v2/search/episodes 搜索与 /api/v2/comment/:id 弹幕接口）。播放时按视频名称搜索番剧并拉取 Bilibili 格式 XML 弹幕。',
      placeholder: 'https://api.dandanplay.net',
    },
    {
      key: 'danmaku.app_id',
      label: 'AppId（弹弹play 开放 API）',
      type: 'text',
      hint: '弹弹play DevCenter 申请的应用 ID（https://doc.dandanplay.com/open/）。官方接口要求应用认证，留空使用内置凭据（签名认证，开箱即用）；填写自己的 AppId/AppKey 可覆盖内置凭据。仅对官方 api.dandanplay.net 生效，第三方协议源不会携带凭据。',
      placeholder: '在 DevCenter 申请的应用 ID',
    },
    {
      key: 'danmaku.app_key',
      label: 'AppKey（弹弹play 应用密钥）',
      type: 'text',
      hint: '与 AppId 配套的 AppSecret，只保存在服务器上用于计算请求签名（base64(sha256(AppId+Timestamp+Path+Secret))），不会下发到播放器。',
      placeholder: '在 DevCenter 申请的应用密钥',
    },
    {
      key: 'danmaku.opacity',
      label: '弹幕透明度',
      type: 'number',
      hint: '0.1 ~ 1，数值越小越透明（1 为完全不透明）',
      defaultValue: '1',
      placeholder: '1',
    },
    {
      key: 'danmaku.font_size',
      label: '弹幕字号',
      type: 'number',
      hint: '弹幕渲染字号（px），默认 24',
      defaultValue: '24',
      placeholder: '24',
    },
    {
      key: 'danmaku.area',
      label: '弹幕显示区域',
      type: 'number',
      hint: '0 ~ 1，弹幕占视频高度比例（1 为全屏，0.5 为上半屏）',
      defaultValue: '1',
      placeholder: '1',
    },
  ],
}