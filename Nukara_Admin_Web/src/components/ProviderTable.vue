<script setup>
defineProps({
  providers: {
    type: Array,
    default: () => [],
  },
  loading: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['edit', 'delete', 'test', 'switch'])
</script>

<template>
  <section class="provider-table-wrap">
    <h2>Providers</h2>
    <p v-if="loading">Loading providers...</p>
    <p v-else-if="providers.length === 0">No providers yet.</p>

    <table v-else class="provider-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Base URL</th>
          <th>Models</th>
          <th>Priority</th>
          <th>Active</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="provider in providers" :key="provider.id">
          <td>{{ provider.name }}</td>
          <td>{{ provider.base_url }}</td>
          <td>{{ Array.isArray(provider.models) ? provider.models.join(', ') : provider.models }}</td>
          <td>{{ provider.priority }}</td>
          <td>{{ provider.is_active ? 'Yes' : 'No' }}</td>
          <td class="actions">
            <button type="button" @click="emit('edit', provider)">Edit</button>
            <button type="button" @click="emit('delete', provider)">Delete</button>
            <button type="button" @click="emit('test', provider)">Test</button>
            <button type="button" @click="emit('switch', provider)">Switch</button>
          </td>
        </tr>
      </tbody>
    </table>
  </section>
</template>
