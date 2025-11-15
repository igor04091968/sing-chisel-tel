<template>
  <ServiceVue 
    v-model="modal.visible"
    :visible="modal.visible"
    :id="modal.id"
    :data="modal.data"
    :inTags="inTags"
    :tsTags="tsTags"
    :ssTags="ssTags"
    :tlsConfigs="tlsConfigs"
    @close="closeModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showModal(null)">{{ $t('actions.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>services" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ item.type }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('in.addr') }}</v-col>
            <v-col>
              {{ item.listen }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('in.port') }}</v-col>
            <v-col>
              {{ item.listen_port }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.tls') }}</v-col>
            <v-col>
              {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
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
                <v-btn color="error" variant="outlined" @click="delSrv(item, index)">{{ $t('yes') }}</v-btn>
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
import Data from '@/store/modules/data'
import { Srv } from '@/types/services'
import { computed, ref } from 'vue'
import ServiceVue from '@/layouts/modals/Service.vue'

const services = computed(() => {
  const data = Data()
  const genericServices = (data.services as any[]).map(s => ({...s, _objectType: 'services'}))
  const mappedChisel = (data.chisel as any[]).map(c => {
    const isServer = c.Mode === 'server';
    return {
      id: c.ID,
      tag: c.Name,
      type: `chisel-${c.Mode}`,
      listen: isServer ? c.ListenAddress : c.ServerAddress,
      listen_port: isServer ? c.ListenPort : c.ServerPort,
      tls_id: (c.Args as string).includes('--tls') ? 1 : 0, // Best effort to show TLS status
      _objectType: 'chisel',
      _original: c,
    }
  })
  return genericServices.concat(mappedChisel)
})

const srvTags = computed((): any[] => {
  return services.value?.map((o:Srv) => o.tag)
})

const tsTags = computed((): any[] => {
  return Data().endpoints?.filter((o:any) => o.type == "tailscale")?.map((o:any) => o.tag)
})

const ssTags = computed((): any[] => {
  return Data().inbounds?.filter((o:any) => o.type == "shadowsocks" && !o.users)?.map((o:any) => o.tag)
})

const inTags = computed((): any[] => {
  return [...Data().inbounds?.map((o:any) => o.tag).filter(t => t != null), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
})

const tlsConfigs = computed((): any[] => {
  return <any[]> Data().tlsConfigs
})

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})

let delOverlay = ref(new Array<boolean>)

const showModal = (item: any) => {
  const id = item?.id ?? 0;
  modal.value.id = id;
  let dataObject = null;
  if (item) {
    if (item._objectType === 'chisel') {
      dataObject = item._original;
    } else {
      // Find the original service object from the store
      dataObject = Data().services.find(s => s.id === id);
    }
  }
  modal.value.data = id === 0 ? '' : JSON.stringify(dataObject);
  modal.value.visible = true;
}

const closeModal = () => {
  modal.value.visible = false
}

const delSrv = async (item: any, index: number) => {
  const objectType = item._objectType;
  // For 'services', the delete payload is the tag. For 'chisel', it's the ID.
  const payload = objectType === 'chisel' ? item.id : item.tag;
  const success = await Data().save(objectType, "del", payload)
  if (success) delOverlay.value[index] = false
}
</script>