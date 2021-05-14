<template>
  <el-option :key="`${source.name}-${source.tag}`"
             :label="source.tag"
             :value="source.full">
  </el-option>
</template>

<script>
export default {
  props: {
    source: {
      type: Object,
      default() {
        return {}
      }
    }
  },
}
</script>