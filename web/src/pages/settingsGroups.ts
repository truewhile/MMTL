import { adultSettingsGroup } from './settingsGroupAccess'
import { apiConfigsSettingsGroup } from './settingsGroupAPIConfigs'
import { danmakuSettingsGroup } from './settingsGroupDanmaku'
import { generalSettingsGroup } from './settingsGroupGeneral'
import { recognitionWordsSettingsGroup } from './settingsGroupRecognitionWords'
import type { SettingGroup } from './settingsGroupTypes'

export type { SettingGroup } from './settingsGroupTypes'

export const GROUPS: SettingGroup[] = [
  generalSettingsGroup,
  apiConfigsSettingsGroup,
  recognitionWordsSettingsGroup,
  danmakuSettingsGroup,
  adultSettingsGroup,
]

export const ALL_KEYS = new Set(GROUPS.flatMap((group) => group.items.map((item) => item.key)))
