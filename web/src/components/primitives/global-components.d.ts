import type WfAnno from './WfAnno.vue'
import type WfBox from './WfBox.vue'
import type WfBtn from './WfBtn.vue'
import type WfCheckbox from './WfCheckbox.vue'
import type WfChip from './WfChip.vue'
import type WfPlaceholder from './WfPlaceholder.vue'
import type WfTag from './WfTag.vue'

declare module 'vue' {
  export interface GlobalComponents {
    WfAnno: typeof WfAnno
    WfBox: typeof WfBox
    WfBtn: typeof WfBtn
    WfCheckbox: typeof WfCheckbox
    WfChip: typeof WfChip
    WfPlaceholder: typeof WfPlaceholder
    WfTag: typeof WfTag
  }
}

export {}
