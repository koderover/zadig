<template>
  <div class="container-home">

  </div>
</template>
<script>
import bus from '@utils/event_bus'
export default {
  methods: {},
  mounted () {
    bus.$emit('show-sidebar', true)
    this.$router.replace('/v1/enterprise/users/manage')
  }
}
</script>

<style lang="less" >
.container-home {
  position: relative;
  flex: 1;
  width: 100%;
  padding: 15px 20px;
  overflow: auto;
}
</style>
