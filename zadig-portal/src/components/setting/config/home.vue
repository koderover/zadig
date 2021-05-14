<template>
  <div class="config-home">
    <div class="tab-container">
      <el-tabs v-model="activeTab" type="card">
        <el-tab-pane name="proxy" label="代理配置">
          <keep-alive >
            <Proxy v-if="activeTab === 'proxy'" />
          </keep-alive>
        </el-tab-pane>
        <el-tab-pane name="cache" label="缓存清理">
          <keep-alive >
            <Cache v-if="activeTab === 'cache'" />
          </keep-alive>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>
<script>
import Proxy from './proxy.vue'
import Cache from './cache.vue'
import bus from '@utils/event_bus';

export default {
  name: 'config',
  components: {
    Proxy,
    Cache,
  },
  data() {
    return {
      activeTab: 'proxy',
    }
  },
  mounted () {
    bus.$emit(`set-topbar-title`, { title: '系统配置', breadcrumb: [] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
  }
}
</script>

<style lang="less" >
.config-home {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 30px;
}
</style>
