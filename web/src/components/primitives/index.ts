import type { App } from 'vue'
import WfAnno from './WfAnno.vue'
import WfBox from './WfBox.vue'
import WfBtn from './WfBtn.vue'
import WfCheckbox from './WfCheckbox.vue'
import WfChip from './WfChip.vue'
import WfPlaceholder from './WfPlaceholder.vue'
import WfTag from './WfTag.vue'

export { WfAnno, WfBox, WfBtn, WfCheckbox, WfChip, WfPlaceholder, WfTag }

export function registerPrimitives(app: App): void {
  app.component('WfAnno', WfAnno)
  app.component('WfBox', WfBox)
  app.component('WfBtn', WfBtn)
  app.component('WfCheckbox', WfCheckbox)
  app.component('WfChip', WfChip)
  app.component('WfPlaceholder', WfPlaceholder)
  app.component('WfTag', WfTag)
}
