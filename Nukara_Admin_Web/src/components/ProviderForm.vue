<script setup>
import { reactive, watch } from 'vue'

const props = defineProps({
  provider: {
    type: Object,
    default: null,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['submit', 'cancel'])

const form = reactive({
  name: '',
  api_key: '',
  base_url: '',
  models: '',
  priority: 0,
  is_active: false,
})

function assignForm(provider) {
  form.name = provider?.name || ''
  form.api_key = provider?.api_key || ''
  form.base_url = provider?.base_url || ''
  form.models = Array.isArray(provider?.models)
    ? provider.models.join(',')
    : provider?.models || ''
  form.priority = provider?.priority ?? 0
  form.is_active = Boolean(provider?.is_active)
}

watch(
  () => props.provider,
  (provider) => {
    assignForm(provider)
  },
  { immediate: true },
)

function onSubmit() {
  emit('submit', { ...form })
}
</script>

<template>
  <form class="provider-form" @submit.prevent="onSubmit">
    <h2>{{ provider ? 'Edit Provider' : 'Create Provider' }}</h2>

    <label>
      Name
      <input v-model.trim="form.name" required :disabled="disabled" />
    </label>

    <label>
      API Key
      <input v-model="form.api_key" required :disabled="disabled" />
    </label>

    <label>
      Base URL
      <input v-model.trim="form.base_url" required :disabled="disabled" />
    </label>

    <label>
      Models (comma separated)
      <input v-model="form.models" :disabled="disabled" />
    </label>

    <label>
      Priority
      <input v-model.number="form.priority" type="number" :disabled="disabled" />
    </label>

    <label class="checkbox-row">
      <input v-model="form.is_active" type="checkbox" :disabled="disabled" />
      Active
    </label>

    <div class="form-actions">
      <button type="submit" :disabled="disabled">{{ provider ? 'Update' : 'Create' }}</button>
      <button type="button" class="ghost" :disabled="disabled" @click="emit('cancel')">Cancel</button>
    </div>
  </form>
</template>
