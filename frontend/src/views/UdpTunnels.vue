<template>
  <UdpTunnelVue
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    @close="closeModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showModal(null)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6" md="4" lg="3" v-for="(item, index) in tunnels" :key="item.ID">
      <v-card rounded="xl" elevation="5">
        <v-card-title>{{ item.Name }}</v-card-title>
        <v-card-subtitle>
          <v-chip :color="item.Status === 'running' ? 'success' : 'grey'" class="mr-2">{{ item.Status }}</v-chip>
          Mode: {{ item.Mode }}, Role: {{ item.Role }}
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>Listen Port</v-col>
            <v-col>{{ item.ListenPort }}</v-col>
          </v-row>
          <v-row>
            <v-col>Remote Address</v-col>
            <v-col>{{ item.RemoteAddress }}</v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-play" color="success" @click="startTunnel(item.ID)" :disabled="item.Status === 'running'">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.start')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-stop" color="error" @click="stopTunnel(item.ID)" :disabled="item.Status !== 'running'">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.stop')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-edit" @click="showModal(item)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="deleteTunnel(item.ID, index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import Data from '@/store/modules/data'
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import UdpTunnelVue from '@/layouts/modals/UdpTunnel.vue'

const dataStore = Data()
const tunnels = computed(() => dataStore.udptunnels)

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

let delOverlay = ref(new Array<boolean>())

const showModal = (item: any) => {
  const id = item?.ID ?? 0
  modal.value.id = id
  modal.value.data = id === 0 ? '' : JSON.stringify(item)
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

const startTunnel = async (id: number) => {
  const msg = await HttpUtils.post('api/udp_tunnel_start', { id })
  if (msg.success) {
    push.success({ title: i18n.global.t('success'), message: "Tunnel started" })
    dataStore.loadData() // Refresh data
  }
}

const stopTunnel = async (id: number) => {
  const msg = await HttpUtils.post('api/udp_tunnel_stop', { id })
  if (msg.success) {
    push.success({ title: i18n.global.t('success'), message: "Tunnel stopped" })
    dataStore.loadData() // Refresh data
  }
}

const deleteTunnel = async (id: number, index: number) => {
  const msg = await HttpUtils.post('api/udp_tunnel_delete', { id })
  if (msg.success) {
    push.success({ title: i18n.global.t('success'), message: "Tunnel deleted" })
    delOverlay.value[index] = false
    dataStore.loadData() // Refresh data
  }
}
</script>
