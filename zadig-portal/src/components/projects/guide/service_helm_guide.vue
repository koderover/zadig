<template>
<div class="con">
    <div class="guide-container">
      <step :activeStep="2">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第二步</span>
          <span class="second">创建服务模板，后续均可在项目中重新配置</span>
        </div>
      </div>
    </div>
  <div class="content">
    <Code ref="code" :service="serviceList" >
    </Code>
  </div>
      <div class="controls__wrap">
      <div class="controls__right">
        <el-button type="primary"
                   size="small"
                   class="save-btn"
                   @click="toNext"
                   :disabled="!showNext"
                   plain>下一步</el-button>
      </div>
    </div>
</div>
</template>
<script>
import { mapState } from 'vuex'
import step from './common/step.vue'
import Code from '../web_code/code'

export default {
  name: 'service_helm',
  components: {
    Code,
    step
  },
  data () {
    return {
      serviceList: []
    }
  },
  methods: {
    toNext () {
      this.$router.push(`/v1/projects/create/${this.projectName}/basic/runtime?serviceName=${this.serviceName}&serviceType=${this.serviceType}`)
    },
    async querytHelmChartService () {
      this.$store.dispatch('queryService', { projectName: this.projectName })
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    ...mapState({
      showNext: (state) => state.service_manage.showNext
    }),
    serviceName () {
      return this.$route.query.service_name
    },
    serviceType () {
      return this.$route.query.service_type
    }
  },
  mounted () {
    this.querytHelmChartService()
  }
}
</script>
<style lang="less" scoped>
.con {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: calc(~"100% - 61px");
  background-color: #f5f7f7;

  .guide-container {
    margin-top: 20px;
  }

  .current-step-container {
    .title-container {
      margin-left: 20px;

      .first {
        display: inline-block;
        width: 110px;
        padding: 8px;
        color: #fff;
        font-weight: 300;
        font-size: 18px;
        text-align: center;
        background: #3289e4;
      }

      .second {
        color: #4c4c4c;
        font-size: 13px;
      }
    }
  }

  .controls__wrap {
    position: relative;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 55px;
    margin: 0 5px;
    background-color: #fff;
    border-top: 1px solid #ccc;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);

    > * {
      margin-right: 10px;
    }

    .controls__right {
      display: flex;
      align-items: center;
      padding: 0 15px;

      .save-btn {
        margin-right: 15px;
        padding: 10px 17px;
        color: #fff;
        font-weight: bold;
        font-size: 13px;
        text-decoration: none;
        background-color: #1989fa;
        border: 1px solid #1989fa;
        cursor: pointer;
        transition: background-color 300ms, color 300ms, border 300ms;
      }

      .save-btn[disabled] {
        background-color: #9ac9f9;
        border: 1px solid #9ac9f9;
        cursor: not-allowed;
      }
    }
  }
}

.content {
  display: flex;
  flex: 1;
  overflow: auto;
  background-color: #fff;
  user-select: none;
}
</style>
