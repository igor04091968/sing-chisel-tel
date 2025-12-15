<template>
  <v-dialog v-model="props.visible" max-width="600px">
    <v-card :title="isNew ? $t('actions.add') : $t('actions.edit')" rounded="lg">
      <v-card-text>
        <v-text-field v-model="tunnel.Name" label="Name" variant="outlined" />
        <v-select
          v-model="tunnel.Mode"
          :items="['faketcp', 'icmp', 'raw_udp']"
          label="Mode"
          variant="outlined"
        />
        <v-select
          v-model="tunnel.Role"
          :items="['client', 'server']"
          label="Role"
          variant="outlined"
        />
        <v-text-field v-model.number="tunnel.ListenPort" label="Listen Port" type="number" variant="outlined" />
        <v-text-field v-model="tunnel.RemoteAddress" label="Remote Address (host:port)" variant="outlined" />
        <v-expansion-panels title="Advanced Options">
          <v-expansion-panel>
            <v-expansion-panel-title>Advanced Options</v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-text-field v-model.number="tunnel.VLANID" label="VLAN ID" type="number" variant="outlined" />
              <v-text-field v-model.number="tunnel.DSCP" label="DSCP (0-63)" type="number" variant="outlined" />
              <v-text-field v-model="tunnel.InterfaceName" label="Interface Name (e.g., eth0)" variant="outlined" />
              <v-text-field v-model="tunnel.DestMAC" label="Destination MAC" variant="outlined" />
              <v-text-field v-model="tunnel.FakeTCPFlags" label="Fake TCP Flags (e.g., SYN,ACK)" variant="outlined" />
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>
      </v-card-text>
      <v-divider />
      <v-card-actions>
        <v-spacer />
        <v-btn color="primary" variant="text" @click="save">{{ $t('actions.save') }}</v-btn>
        <v-btn color="secondary" variant="text" @click="emit('close')">{{ $t('actions.close') }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, watch } from 'vue'
import { push } from 'notivue'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'

const props = defineProps<{
  id: number,
  visible: boolean,
  data: string,
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const isNew = ref(true)
const tunnel = ref<any>({})

watch(() => props.visible, (newVal) => {
  if (newVal) {
    isNew.value = props.id === 0
    if (!isNew.value) {
      tunnel.value = JSON.parse(props.data)
    } else {
      tunnel.value = { Mode: 'faketcp', Role: 'client' } // Default values
    }
  }
})

const save = async () => {
  const action = isNew.value ? 'udp_tunnel_save' : 'udp_tunnel_update'
  const options = {
    headers: {
      'Content-Type': 'application/json; charset=UTF-8'
    }
  }
  const msg = await HttpUtils.post(`api/${action}`, tunnel.value, options)
  if (msg.success) {
    push.success({
      title: i18n.global.t('success'),
      message: `${i18n.global.t('actions.save')} ${tunnel.value.Name}`
    })
    Data().loadData() // Refresh data
    emit('close')
  }
}
</script>
