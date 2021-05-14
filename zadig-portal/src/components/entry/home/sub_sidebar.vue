<template>
  <div class="sub-side-bar-wrapper"
       :class="showSubSidebar?'sub-side-bar-wrapper-opened':'sub-side-bar-wrapper-closed'"
       style="">
    <div v-show="showSubSidebar"
         class="sub-side-bar">
      <div class="sub-side-bar-inner">

        <div class="pipelines-sub-side-bar__wrap">
          <div class="pipelines-sub-side-bar__header">
            <div class="cf-registry-selector__preview">
              <span v-if="content.url"
                    class="cf-registry-selector__text url">
                <router-link :to="content.url">{{content.title}}</router-link>
              </span>
              <span v-else-if="content.title"
                    class="cf-registry-selector__text">{{content.title}}</span>

              <i class="fa cf-registry-selector__arrow fa-angle-down"></i>
            </div>
          </div>
          <div class="pipelines-sub-side-bar__content">

            <ul v-if="content.routerList.length > 0"
                class="pipelines-sub-side-bar__list">
              <router-link v-for="(item,index) in content.routerList"
                           :key="index"
                           active-class="selected"
                           :to="item.url">
                <li class="pipelines-sub-side-bar__item"
                    style="">
                  <span class="pipeline-name">{{item.name}}</span>
                </li>
              </router-link>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import bus from '@utils/event_bus';
import { mapGetters } from 'vuex';
export default {
  data() {
    return {
      content: {
        title: '',
        routerList: []
      },
    }
  },
  methods: {
    changeTitle(params) {
      this.content = params;
      if (this.content.title !== '' && this.content.routerList.length > 0) {
        bus.$emit(`show-sidebar`, false);
        bus.$emit(`sub-sidebar-opened`, true);
      }
      else if (this.content.routerList.length === 0) {
        bus.$emit(`show-sidebar`, true);
        bus.$emit(`sub-sidebar-opened`, false);
      }
    }
  },
  computed: {
    showSubSidebar: {
      get: function () {
        if (this.content.title !== '' && this.content.routerList.length > 0) {
          bus.$emit(`show-sidebar`, false);
          bus.$emit(`sub-sidebar-opened`, true);
          return true;
        }
        else if (this.content.routerList.length === 0) {
          bus.$emit(`show-sidebar`, true);
          bus.$emit(`sub-sidebar-opened`, false);
          return false;
        }
      },
      set: function (newValue) {
      }
    },
    ...mapGetters([
      'getTemplates'
    ])
  },
  beforeDestroy() {
    bus.$emit(`sub-sidebar-opened`, false);
  },
  created() {
    bus.$on('set-sub-sidebar-title', async (params) => {
      let isProjectList = false;
      if (params.routerList && params.routerList.length > 2 && params.routerList[1].name === '集成环境') {
        isProjectList = true;
      }
      if (isProjectList) {
        let findPro = false;

        let findTem = this.getTemplates.filter(tem => {
          return tem.product_name === params.title
        })
        if (findTem.length === 0) {
          await this.$store.dispatch("refreshProjectTemplates");
        }
        
        for (let product of this.getTemplates) {
          if (product.product_name === params.title) {
            findPro = true;
            if (product.product_feature && product.product_feature.develop_habit === 'yaml') {
              params.routerList.splice(2);
              break;
            }
          }
        }
        this.changeTitle(params);
        if (!findPro) {
          throw ('Not find the product from getTemplates1');
        }
      } else {
        this.changeTitle(params);
      }
    });
  },
}
</script>

<style lang="less">
.sub-side-bar-wrapper {
  @import url("~@assets/css/common/scroll-bar.less");
  height: 100%;
  position: relative;
  background: transparent;
  left: 0;
  transition: width 0.2s;
  &.sub-side-bar-wrapper-opened {
    width: 130px;
    left: 0;
  }
  &.sub-side-bar-wrapper-closed {
    position: absolute;
    appearance: none;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    left: -100%;
  }
  .sub-side-bar {
    position: absolute;
    height: 100%;
    top: 0;
    left: 0;
    right: 0;
    transition: opacity, 400ms linear, -webkit-transform;
    transition: transform, opacity, 400ms linear;
    transition: transform, opacity, 400ms linear, -webkit-transform;
    z-index: 1010;
    .sub-side-bar-inner {
      height: 100%;
      width: 100%;
      transition: -webkit-transform 0.2s;
      transition: transform 0.2s;
      transition: transform 0.2s, -webkit-transform 0.2s;
      .pipelines-sub-side-bar__wrap {
        display: flex;
        flex: 1;
        height: 100%;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        flex-direction: column;
        background-color: #f5f7fa;
        box-shadow: 10px 0px 20px rgba(0, 0, 0, 0.12);
        .pipelines-sub-side-bar__header {
          flex-basis: 60px;
          .pipelines-sub-side-bar__section-label {
            color: #888;
            text-transform: uppercase;
            font-size: 10px;
            font-weight: bold;
            padding-left: 12px;
            padding-bottom: 12px;
          }
          .cf-registry-selector__preview {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 5px 10px;
            font-size: 14px;
          }
          .cf-registry-selector__preview {
            position: relative;
            height: 50px;
            font-weight: bold;
            padding: 5px 30px 5px 12px;
            text-transform: uppercase;
            color: #bebebe;
            .cf-registry-selector__text {
              display: inline-block;
              overflow: hidden;
              white-space: nowrap;
              text-overflow: ellipsis;
              color: #888;
              &.url {
                a {
                  color: #888;
                  &:hover {
                    color: #1989fa;
                  }
                }
              }
            }
          }
        }
        .pipelines-sub-side-bar__content {
          flex-grow: 1;
          overflow-y: auto;
          scrollbar-color: #c0c0c0;
          scrollbar-width: thin;
          .pipelines-sub-side-bar__list {
            list-style-type: none;
            padding: 0;
            margin: 0;
            .selected {
              .pipelines-sub-side-bar__item {
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
              }
            }
            .pipelines-sub-side-bar__item {
              display: flex;
              align-items: center;
              -webkit-box-pack: justify;
              -ms-flex-pack: justify;
              justify-content: space-between;
              color: #eeeeee;
              font-size: 12px;
              padding: 5px 20px;
              height: 35px;
              transition: background-color 150ms ease, color 150ms ease;
              &.selected {
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
              }
              .pipeline-name {
                min-width: 0;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
                flex: 1;
                color: #434548;
                font-weight: 500;
                line-height: 16px;
              }
              &:hover {
                cursor: pointer;
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
                padding-right: 2px;
              }
            }
          }
        }
        .pipelines-sub-side-bar__footer {
          flex-basis: 60px;
          background-color: #fff;
          box-shadow: 0px -6px 4px rgba(0, 0, 0, 0.04);
        }
        .pipelines-sub-side-bar__footer {
          display: flex;
          justify-content: flex-start;
          align-items: center;
          padding-left: 12px;
          -ms-flex-negative: 0;
          flex-shrink: 0;
        }
      }
    }
  }
}
</style>