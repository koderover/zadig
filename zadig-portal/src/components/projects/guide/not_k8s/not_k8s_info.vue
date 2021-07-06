<template>
  <div class="projects-not-k8s-info-container">
    <transition name="el-fade-in-linear">
      <div class="page-title-container">
        <h1>恭喜你成功创建新的项目 {{this.projectName}}</h1>
      </div>
    </transition>

    <div class="guide-container">
      <step :activeStep="0">
      </step>
      <div class="current-step-container">
        <div class="title-container">
          <span class="first">第一步</span>
          <span class="second">对项目的流程做初步定义后，后续可在项目中进行调整。当您创建好服务后，我们会为你做如下的智能交付准备。KodeRover
            会自动生成以下资源：</span>
        </div>
        <div class="account-integrations cf-block__list">
          <div class="cf-block__item">
            <div class="account-box-item">
              <div class="account-box-item__info integration-card">
                <div class="integration-card__image">
                  <el-button type="success"
                             icon="el-icon-check"
                             circle></el-button>
                </div>
                <div class="integration-card__info">
                  <div class="integration-name cf-sub-title">2 套测试环境</div>
                  <div class="integration-details">dev,qa
                  </div>
                </div>
              </div>
              <div class="account-box-item__controls">

              </div>
            </div>
          </div>
          <div class="cf-block__item">
            <div class="account-box-item">
              <div class="account-box-item__info integration-card">
                <div class="integration-card__image">
                  <el-button type="success"
                             icon="el-icon-check"
                             circle></el-button>
                </div>
                <div class="integration-card__info">
                  <div class="integration-name cf-sub-title">2 条工作流</div>
                  <div class="integration-details">
                    {{projectName}}-workflow-dev , {{projectName}}-workflow-qa
                  </div>
                </div>
              </div>
              <div class="account-box-item__controls">

              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="controls__wrap">
      <div class="controls__right">
        <router-link :to="`/v1/projects/create/${this.projectName}/not_k8s/config`">
          <button type="primary"
                  class="save-btn"
                  plain>下一步</button>
        </router-link>
        <button type="primary"
                class="save-btn"
                @click="jumpOnboarding">
          <i v-if="jumpLoading"
             class="el-icon-loading"></i>
          <span>跳过向导</span>
        </button>
      </div>
    </div>
  </div>
</template>
<script>
import bus from '@utils/event_bus'
import step from './container/step_not_k8s.vue'
export default {
  data () {
    return {
      jumpLoading: false
    }
  },
  methods: {
    jumpOnboarding () {
      this.jumpLoading = true
      this.saveOnboardingStatus(this.projectName, 0).then((res) => {
        this.$router.push(`/v1/projects/detail/${this.projectName}`)
      }).catch(() => {
        this.jumpLoading = false
      })
    }
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    }
  },
  created () {
    bus.$emit(`show-sidebar`, true)
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: '' }] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
  },
  components: {
    step
  },
  onboardingStatus: 1
}
</script>

<style lang="less">
.projects-not-k8s-info-container {
  position: relative;
  flex: 1;
  overflow: auto;
  background-color: #f5f7f7;

  .page-title-container {
    display: flex;
    padding: 0 20px;

    h1 {
      width: 100%;
      color: #4c4c4c;
      font-weight: 300;
      text-align: center;
    }
  }

  .guide-container {
    min-height: calc(~"100% - 150px");
    margin-top: 10px;

    .current-step-container {
      .title-container {
        margin-bottom: 10px;
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
          display: inline-block;
          color: #4c4c4c;
          font-size: 13px;
        }
      }

      .cf-block__list {
        -ms-flex: 1;
        flex: 1;
        margin-top: 15px;
        padding: 0 30px;
        overflow-y: auto;
        background-color: inherit;
        -webkit-box-flex: 1;

        .cf-block__item {
          .account-box-item {
            display: -webkit-box;
            display: -ms-flexbox;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 10px;
            padding: 20px 30px;
            background-color: #fff;
            -webkit-box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            box-shadow: 0 3px 2px 1px rgba(0, 0, 0, 0.05);
            filter: progid:dximagetransform.microsoft.dropshadow(OffX=0, OffY=3px, Color='#0D000000');
            -webkit-box-align: center;
            -ms-flex-align: center;
            -webkit-box-pack: justify;
            -ms-flex-pack: justify;

            .integration-card {
              display: -webkit-box;
              display: -ms-flexbox;
              display: flex;
              align-items: center;
              justify-content: flex-start;
              -webkit-box-align: center;
              -ms-flex-align: center;
              -webkit-box-pack: start;
              -ms-flex-pack: start;

              .integration-card__image {
                width: 64px;

                .el-button.is-circle {
                  padding: 6px;
                  border-radius: 50%;
                }
              }

              .cf-sub-title {
                color: #2f2f2f;
                font-weight: bold;
                font-size: 16px;
                text-align: left;
              }

              .integration-details {
                color: #4c4c4c;
                font-size: 13px;
              }
            }

            .integration-card > * {
              -ms-flex: 0 0 auto;
              flex: 0 0 auto;
              -webkit-box-flex: 0;
            }
          }
        }
      }
    }
  }

  .alert {
    display: flex;
    padding: 0 25px;

    .el-alert {
      margin-bottom: 35px;

      .el-alert__title {
        font-size: 15px;
      }
    }
  }

  .controls__wrap {
    position: relative;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 2;
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 60px;
    margin: 0 15px;
    padding: 0 10px;
    background-color: #fff;
    box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);

    .controls__right {
      display: -webkit-box;
      display: -ms-flexbox;
      display: flex;
      align-items: center;
      margin-right: 10px;
      -webkit-box-align: center;
      -ms-flex-align: center;

      .save-btn,
      .next-btn {
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

      .save-btn[disabled],
      .next-btn[disabled] {
        background-color: #9ac9f9;
        border: 1px solid #9ac9f9;
        cursor: not-allowed;
      }
    }
  }
}
</style>
