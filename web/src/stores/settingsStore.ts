import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getSettings, updateSettings, type Settings } from '../lib/api'

export const useSettingsStore = defineStore('settings', () => {
  const showSettings = ref(false)
  const settingsForm = ref<Settings>({
    llm_url: '',
    llm_model: '',
    llm_api_key: '',
    system_prompt: '',
  })
  const settingsSaving = ref(false)
  const settingsError = ref('')

  async function openSettings() {
    settingsError.value = ''
    try {
      const settings = await getSettings()
      settingsForm.value = { ...settings }
    } catch {
      settingsForm.value = { llm_url: '', llm_model: '', llm_api_key: '', system_prompt: '' }
    }
    showSettings.value = true
  }

  function closeSettings() {
    showSettings.value = false
  }

  async function saveSettings() {
    settingsSaving.value = true
    settingsError.value = ''
    try {
      const saved = await updateSettings(settingsForm.value)
      settingsForm.value = { ...saved }
      showSettings.value = false
    } catch (error) {
      settingsError.value = error instanceof Error ? error.message : 'Failed to save settings'
    } finally {
      settingsSaving.value = false
    }
  }

  return {
    showSettings,
    settingsForm,
    settingsSaving,
    settingsError,
    openSettings,
    closeSettings,
    saveSettings,
  }
})
