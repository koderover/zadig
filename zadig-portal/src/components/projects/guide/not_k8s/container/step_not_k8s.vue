<template>
  <div class="not-k8s-step-container">
    <div class="step-detail">
      <div class="guide-title">
        <h4>
          产品交付向导
        </h4>
      </div>
      <el-steps :active="activeStep"
                class="process-container"
                align-center
                finish-status="success">
        <el-step title="项目配置"
                 description=""></el-step>
        <el-step title="服务配置"
                 description=""></el-step>
        <el-step title="加入环境"
                 description=""></el-step>
        <el-step title="工作流交付"
                 description=""></el-step>
      </el-steps>
    </div>
  </div>
</template>
<script>
export default {
  data () {
    return {

    }
  },
  props: {
    activeStep: {
      required: true,
      type: Number
    }
  }
}
</script>
<style lang="less">
.not-k8s-step-container {
  display: flex;
  flex-direction: row;

  .step-detail {
    width: 100%;

    .guide-title {
      display: flex;

      h4 {
        margin: 0 0 10px 20px;
        padding: 8px;
        color: #fff;
        font-weight: 300;
        font-size: 18px;
        text-align: center;
        background: #3289e4;
      }
    }

    .process-container {
      flex-shrink: 1;
      align-items: flex-end;
    }

    .el-step__icon {
      width: 18px;
      height: 18px;
      font-size: 12px;
    }

    .el-step__title {
      font-size: 14px;
      line-height: 30px;
    }
  }

  .env-container {
    .el-step__icon {
      width: 10px;
      height: 10px;
      color: #3289e4;
      font-size: 12px;
    }

    flex-shrink: 2;

    .el-step__line {
      top: 34px;
      bottom: 0;
      left: 4px;
      width: 2px;
    }

    .el-step__title {
      padding-bottom: 7px;
      color: #ccc;
      font-size: 13px;
      line-height: 36px;
    }
  }
}
</style>
