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
           初始化 Helm，支持两种方式：平台生成和仓库导入
        </p>
      </li>
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          2
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
          编写 Helm 配置
        </p>
      </li>
      <li class="pipelines-aside-help__step-list-item">
        <div class="pipelines-aside-help__step-list-item-counter">
          3
        </div>
        <p class="pipelines-aside-help__step-list-item-text">
           配置可更新镜像
        <ul>
          <li>
            系统可自动从 values.yaml 中解析以下格式的镜像信息 <br>
- 格式一：<br>
 image:<br>
 &nbsp;&nbsp;&nbsp;&nbsp;repository： 镜像仓库/项目名/镜像名<br>
 &nbsp;&nbsp;&nbsp;&nbsp;tag：标签名<br>
 - 格式二：<br>
key：镜像仓库/项目名/镜像名 : 标签名<br>
通过对镜像配置构建脚本，实现版本升级（代码构建->构建镜像->镜像部署）
          </li>
        </ul>
        </p>
      </li>
    </ul>
    <h3 class="pipelines-aside-help__step-header">
      您可能会用到的指南
    </h3>
    <ul class="pipelines-aside-help__step-list">
      <el-row :gutter="20">
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a target="_blank"
               href="/zadig/project/service/#%E6%97%A0%E7%8A%B6%E6%80%81%E6%9C%8D%E5%8A%A1"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">
                无状态服务</span>
            </a>
          </li>
        </el-col>
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a href="/zadig/project/service/#%E6%9C%89%E7%8A%B6%E6%80%81%E6%9C%8D%E5%8A%A1"
               target="_blank"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">有状态服务 </span>
            </a>
          </li>
        </el-col>
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a target="_blank"
               href="/zadig/project/service/#helm-chart-服务"
               class="pipelines-aside-help__step-list-item-link">
              <i class="icon el-icon-link"></i>
              <span class="pipelines-aside-help__step-list-item-link-text">
                Helm Chart 服务</span>
            </a>
          </li>
        </el-col>

      </el-row>
      <el-row :gutter="20">
        <el-col :span="8">
          <li class="pipelines-aside-help__step-list-item">
            <a href="/zadig/project/build/"
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
