<template>
  <div class="step-container">
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
        <el-step title="创建服务"
                 description=""></el-step>
        <el-step title="加入运行环境"
                 description=""></el-step>
        <el-step title="工作流交付"
                 description=""></el-step>
      </el-steps>
    </div>

    <div class="el-steps el-steps--vertical env-container">
      <div class="el-step is-vertical"
           style="flex-basis: 50%;">
        <div class="el-step__head">
          <div class="el-step__line"
               style="margin-right: 0px;"><i class="el-step__line-inner"
               style="transition-delay: 0ms; border-width: 0px; height: 0%;"></i></div>
          <div class="el-step__icon is-text">
            <div class="el-step__icon-inner"></div>
          </div>
        </div>
        <div class="el-step__main">
          <div class="el-step__title">开发环境</div>
        </div>
      </div>
      <div class="el-step is-vertical"
           style="flex-basis: 50%;">
        <div class="el-step__head">
          <div class="el-step__line"
               style="margin-right: 0px;"><i class="el-step__line-inner"
               style="transition-delay: -150ms; border-width: 0px; height: 0%;"></i></div>
          <div class="el-step__icon is-text">
            <div class="el-step__icon-inner"></div>
          </div>
        </div>
        <div class="el-step__main">
          <div class="el-step__title">测试环境</div>
        </div>
      </div>
      <div class="el-step is-vertical is-flex"
           style="flex-basis: 50%;">
        <div class="el-step__head">
          <div class="el-step__line"><i class="el-step__line-inner"></i></div>
          <div class="el-step__icon is-text">
            <div class="el-step__icon-inner"></div>
          </div>
        </div>
        <div class="el-step__main">
          <div class="el-step__title">预发布环境</div>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
export default {
  data() {
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
.step-container {
  flex-direction: row;
  display: flex;
  .step-detail {
    width: 80%;
    .guide-title {
      display: flex;
      h4 {
        font-weight: 300;
        margin: 0 0 10px 20px;
        padding: 8px;
        font-size: 18px;
        background: #3289e4;
        color: #fff;
        width: 110px;
        text-align: center;
      }
    }
    .process-container {
      flex-shrink: 1;
      align-items: flex-end;
      padding-bottom: 27px;
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
      font-size: 12px;
      color: #3289e4;
    }
    flex-shrink: 2;
    .el-step__line {
      width: 2px;
      top: 34px;
      bottom: 0;
      left: 4px;
    }
    .el-step__title {
      font-size: 13px;
      line-height: 36px;
      color: #ccc;
      padding-bottom: 7px;
    }
  }
}
</style>