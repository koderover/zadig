<template>
  <section class="pipeline-help">
    <p class="pipelines-aside-help__welcome-message">
      欢迎使用帮助指南。可参考以下步骤配置服务
    </p>
    <h3 class="pipelines-aside-help__step-header">
    </h3>
    <ul class="pipelines-aside-help__step-list">
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          1
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
          输入服务名称，例如：voting
        </p>
      </li>
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          2
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
          填写该服务的配置信息，支持两种方式
        <ul>
          <span> 方式一：服务的 YAML 保存在平台上</span>
          <span> 方式二：服务的 YAML 保存在代码仓库中，平台通过配置的代码仓库地址实时获取服务配置</span>
        </ul>
        </p>
      </li>
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          3
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
          添加和修改 YAML 中的变量，在运行环境中会被作为环境变量使用
        <ul>
          <li>容器(服务组件)的镜像版本：通过配置构建步骤，实现版本升级（代码构建->构建镜像->镜像部署）
          </li>
          <li>系统内置变量：可以直接在服务模板配置中引用</li>
          <li>自定义变量：在运行环境中会被作为环境变量使用</li>
        </ul>
        </p>
      </li>
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          4
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
          添加多个服务，加入运行环境； 也可后续在项目中添加或者删除
        </p>
      </li>
    </ul>
    <h3 class="pipelines-aside-help__step-header">
      文档指南
    </h3>
    <ul class="pipelines-aside-help__step-list">
      <el-row :gutter="20">
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a target="_blank"
               href="https://docs.koderover.com/zadig/project/service/#%E6%97%A0%E7%8A%B6%E6%80%81%E6%9C%8D%E5%8A%A1"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">
                无状态服务</span>
            </a>
          </li>
        </el-col>
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a href="https://docs.koderover.com/zadig/project/service/#%E6%9C%89%E7%8A%B6%E6%80%81%E6%9C%8D%E5%8A%A1"
               target="_blank"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">有状态服务 </span>
            </a>
          </li>
        </el-col>

      </el-row>
      <el-row :gutter="20">
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a href="https://docs.koderover.com/zadig/project/build/#%E6%96%B0%E5%BB%BA%E6%9E%84%E5%BB%BA"
               target="_blank"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">新增构建步骤</span>
            </a>
          </li>
        </el-col>
      </el-row>
    </ul>
  </section>
</template>
<script>
export default {

}
</script>
<style lang="less" scoped>
.pipeline-help {
  p {
    margin: 0 0 10px;
  }

  .pipelines-aside-help__welcome-message {
    width: 100%;
    margin: 0;
    margin-bottom: 10px;
    color: #2f2f2f;
    font-weight: 500;
    font-size: 14px;
    line-height: 18px;
  }

  .pipelines-aside-help__step-header {
    width: 100%;
    margin: 0;
    margin: 10px 0;
    color: #000;
    font-weight: bold;
    font-size: 14px;
    text-transform: uppercase;
  }

  .pipelines-aside-help__step-list {
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    align-items: unset;
    justify-content: flex-start;
    width: 100%;
    margin: 10px 0;
    padding: 0;
    list-style: none;

    .pipelines-aside-help__step-header {
      width: 100%;
      margin: 0;
      margin: 10px 0;
      color: #000;
      font-weight: bold;
      font-size: 12px;
      text-transform: uppercase;
    }

    .pipelines-aside-help__step-list-item {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      align-items: flex-start;
      justify-content: flex-start;
      width: 100%;
      margin-bottom: 8px;
      -webkit-box-align: start;
      -ms-flex-align: start;
      -webkit-box-pack: start;
      -ms-flex-pack: start;

      ul {
        padding-left: 10px;

        span {
          display: inline-block;
          padding-left: 5px;
          color: #606266;
          font-size: 12px;
        }
      }

      ul > li {
        color: #606266;
      }

      .pipelines-aside-help__step-list-item-counter {
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        align-items: center;
        justify-content: center;
        width: 20px;
        height: 20px;
        margin-right: 13px;
        color: #fff;
        font-weight: bold;
        font-size: 13px;
        background-color: #9b51e0;
        border-radius: 50%;
        -webkit-box-align: center;
        -ms-flex-align: center;
        -webkit-box-pack: center;
        -ms-flex-pack: center;
      }

      .pipelines-aside-help__step-list-item-text {
        -ms-flex: 1;
        flex: 1;
        margin: 0;
        color: #000;
        font-size: 13px;
        line-height: 24px;
        -webkit-box-flex: 1;
      }

      .pipelines-aside-help__step-list-item-link,
      .pipelines-aside-help__step-list-item-link:hover,
      .pipelines-aside-help__step-list-item-link:focus,
      .pipelines-aside-help__step-list-item-link:active {
        color: #518ff6;
        text-decoration: none;

        .icon {
          margin-right: 5px;
        }

        .pipelines-aside-help__step-list-item-link-text {
          font-size: 12px;
        }
      }
    }
  }
}
</style>
