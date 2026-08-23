import { adultSettingsGroup } from './settingsGroupAccess'
import { apiConfigsSettingsGroup } from './settingsGroupAPIConfigs'
import { generalSettingsGroup } from './settingsGroupGeneral'
import { librarySettingsGroup } from './settingsGroupLibrary'
import { recognitionWordsSettingsGroup } from './settingsGroupRecognitionWords'
import type { SettingGroup } from './settingsGroupTypes'

export type { SettingGroup } from './settingsGroupTypes'

export const GROUPS: SettingGroup[] = [
  generalSettingsGroup,
  librarySettingsGroup,
  apiConfigsSettingsGroup,
  recognitionWordsSettingsGroup,
  adultSettingsGroup,
]

export const ALL_KEYS = new Set(GROUPS.flatMap((group) => group.items.map((item) => item.key)))
