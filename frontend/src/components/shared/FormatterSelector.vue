<script lang="ts" setup>
import { computed } from 'vue'
import { FORMATTER_TYPES } from '../../utils/formatter'
import CustomSelect from './CustomSelect.vue'

const props = defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const selectOptions = computed(() =>
  FORMATTER_TYPES.map(opt => ({ value: opt.value, label: opt.label }))
)

function handleUpdate(value: string | number) {
  emit('update:modelValue', String(value))
}
</script>

<template>
  <div class="flex items-center gap-1">
    <label class="text-[10px] text-gray-500 flex-shrink-0">Format as:</label>
    <CustomSelect
      :model-value="modelValue"
      :options="selectOptions"
      :disabled="disabled"
      @update:model-value="handleUpdate"
      class="min-w-[100px]"
    />
  </div>
</template>
