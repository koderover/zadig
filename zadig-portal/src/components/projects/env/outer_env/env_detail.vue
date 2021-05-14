<template>
  <envDetail :envBasePath="envBasePath">
  </envDetail>
</template>

<script>
import envDetail from '../env_detail/env_detail_comp.vue';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
    };
  },
  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
    envBasePath() {
      return `/v1/envs/detail/${this.projectName}`;
    },
    envName() {
      return this.$route.query.envName;
    }
  },
  methods: {
    setTopbarTitle() {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [{ title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
        { title: '集成环境', url: `` },
        { title: this.envName, url: `` }]
      });
    }
  },
  created() {
    this.setTopbarTitle();
  },
  destroyed() {

  },
  watch: {
    $route(to, from) {
      if (this.projectName && this.envName) {
        this.setTopbarTitle();
      }
    }
  },
  components: {
    envDetail
  }
};
</script>

