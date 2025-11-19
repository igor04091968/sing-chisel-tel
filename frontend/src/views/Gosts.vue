<template>
  <v-row>
    <v-col cols="12">
      <v-card>
        <v-card-title>
          GOST Tunnels
          <v-spacer />
          <v-btn color="primary" @click="showAdd = true">{{ $t('actions.add') }}</v-btn>
        </v-card-title>
        <v-data-table :items="gosts" :headers="headers">
          <template #item.actions="{ item }">
            <v-btn small color="success" @click="start(item.ID)" v-if="item.status != 'up'">Start</v-btn>
            <v-btn small color="warning" @click="stop(item.ID)" v-if="item.status == 'up'">Stop</v-btn>
            <v-btn small color="error" @click="del(item.ID)">Delete</v-btn>
            <v-btn small color="info" @click="edit(item)">Edit</v-btn>
          </template>
        </v-data-table>
      </v-card>
    </v-col>

    <v-dialog v-model="showAdd" persistent max-width="600px">
      <v-card>
        <v-card-title>Add GOST</v-card-title>
        <v-card-text>
          <v-text-field v-model="form.name" label="Name" />
          <v-select v-model="form.mode" :items="['server','client']" label="Mode" />
          <v-text-field v-model.number="form.listen_port" label="Listen Port" />
          <v-text-field v-model="form.listen_address" label="Listen Address" />
          <v-text-field v-model.number="form.server_port" label="Server Port (client)" />
          <v-text-field v-model="form.server_address" label="Server Address (client)" />
          <v-text-field v-model="form.args" label="Args" />
        </v-card-text>
        <v-card-actions>
          <v-btn color="primary" @click="create">Create</v-btn>
          <v-btn text @click="showAdd=false">Cancel</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-row>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import HttpUtils from '@/plugins/httputil'

const showEdit = ref(false)
const editForm = ref({ id: 0, name: '', mode: 'server', listen_address: '0.0.0.0', listen_port: 9999, server_address: '', server_port: 0, args: '' })
const gosts = ref<any[]>([])
const showAdd = ref(false)
const form = ref({ name: '', mode: 'server', listen_address: '0.0.0.0', listen_port: 9999, server_address: '', server_port: 0, args: '' })

const headers = [{ title: 'Name', key: 'name' }, { title: 'Mode', key: 'mode' }, { title: 'Listen', key: 'listen_port' }, { title: 'Status', key: 'status' }, { title: 'Actions', key: 'actions' }]

const load = async () => {
  const msg = await HttpUtils.get('api/gosts')
  if (msg.success) {
    gosts.value = msg.obj.gosts || []
  }
}

const edit = (item: any) => {
  editForm.value = { ...item }
  showEdit.value = true
}

const start = async (id:number) => {
  // базовая валидация
  if (!editForm.value.name || !editForm.value.mode || !editForm.value.listen_port) {
    alert('Name, Mode, Listen Port required')
    return
  }
  const payload = JSON.stringify(editForm.value)
  const msg = await HttpUtils.post('api/gost_update', { id: String(editForm.value.id), data: payload })
  if (msg.success) {
    showEdit.value = false
    load()
  }
  const startMsg = await HttpUtils.post('api/gost_start', { id: String(id) })
  if (startMsg.success) load()
}

const stop = async (id:number) => {
  const msg = await HttpUtils.post('api/gost_stop', { id: String(id) })
  if (msg.success) load()
}

const del = async (id:number) => {
  const msg = await HttpUtils.post('api/gost_delete', { id: String(id) })
  if (msg.success) load()
}

const create = async () => {
  const payload = JSON.stringify(form.value)
  const msg = await HttpUtils.post('api/gost_save', { action: 'new', data: payload })
  if (msg.success) {
    showAdd.value = false
    load()
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
</style>