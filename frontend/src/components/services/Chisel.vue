<template>
  <v-container>
    <v-row>
      <v-col cols="12" sm="6">
        <v-select
          :label="$t('chisel.mode')"
          :items="['server', 'client']"
          v-model="chisel.mode"
          hide-details
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6">
        <v-text-field
          :label="$t('chisel.args')"
          v-model="chisel.args"
          hide-details
        ></v-text-field>
      </v-col>
    </v-row>

    <v-row v-if="chisel.mode === 'server'">
      <v-col cols="12" sm="6">
        <v-text-field
          :label="$t('chisel.listen_address')"
          v-model="chisel.listen_address"
          hide-details
        ></v-text-field>
      </v-col>
      <v-col cols="12" sm="6">
        <v-text-field
          :label="$t('chisel.listen_port')"
          v-model.number="chisel.listen_port"
          type="number"
          hide-details
        ></v-text-field>
      </v-col>
    </v-row>

    <v-row v-if="chisel.mode === 'client'">
      <v-col cols="12" sm="6">
        <v-text-field
          :label="$t('chisel.server_address')"
          v-model="chisel.server_address"
          hide-details
        ></v-text-field>
      </v-col>
      <v-col cols="12" sm="6">
        <v-text-field
          :label="$t('chisel.server_port')"
          v-model.number="chisel.server_port"
          type="number"
          hide-details
        ></v-text-field>
      </v-col>
    </v-row>
  </v-container>
</template>

<script lang="ts">
import { CHISEL } from '@/types/services'

export default {
  props: {
    data: {
      type: Object as () => CHISEL,
      required: true,
    },
  },
  computed: {
    chisel: {
      get(): CHISEL {
        return this.data
      },
      set(value: CHISEL) {
        this.$emit('update:data', value)
      },
    },
  },
}
</script>
