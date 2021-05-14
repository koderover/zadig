<template>
  <div class="pipeline-manage-view">
    <router-view></router-view>
  </div>

</template>

<script>
export default {
  data() {
    return {};
  },
  methods: {},
  created() {},
  components: {}
};
</script>

<style lang="less">
.pipeline-manage-view {
  background-color: #F0F2F5;
  display: flex;
  .tabMenu {
    margin-top: 20px;
  }
}
</style>
