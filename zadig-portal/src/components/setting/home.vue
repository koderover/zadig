<template>
  <div class="container-home">

  </div>
</template>
<script>
import bus from '@utils/event_bus';
export default {
  methods: {},
  mounted() {
    bus.$emit(`show-sidebar`, true);
    this.$router.replace('/v1/system/integration');
  }
};
</script>

<style lang="less" >
.container-home {
  width: 100%;
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 20px;
}
</style>
