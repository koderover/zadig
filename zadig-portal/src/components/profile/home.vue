<template>
  <div class="profile-home">

  </div>
</template>
<script>
import bus from '@utils/event_bus';
export default {
  methods: {},
  mounted() {
    bus.$emit(`show-sidebar`, true);
    this.$router.replace('/v1/profile/info');
  }
};
</script>

<style lang="less" >
.profile-home {
}
</style>
