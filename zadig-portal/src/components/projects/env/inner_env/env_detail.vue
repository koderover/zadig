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
      return `/v1/projects/detail/${this.projectName}/envs/detail`;
    },

  },
  methods: {

  },
  created() {
    bus.$emit(`set-topbar-title`, {
      title: "",
      breadcrumb: [
        { title: "项目", url: "/v1/projects" },
        {
          title: this.projectName,
          url: `/v1/projects/detail/${this.projectName}`
        },
        { title: "集成环境", url: "" }
      ]
    });
  },
  destroyed() {

  },
  watch: {

  },
  components: {
    envDetail
  }
};
</script>

