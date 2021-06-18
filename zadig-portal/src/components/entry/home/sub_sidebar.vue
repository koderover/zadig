<template>
  <div class="sub-side-bar-wrapper"
       :class="showSubSidebar?'sub-side-bar-wrapper-opened':'sub-side-bar-wrapper-closed'">
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
                <li class="pipelines-sub-side-bar__item">
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
import bus from '@utils/event_bus'
import { mapGetters } from 'vuex'
export default {
  data () {
    return {
      content: {
        title: '',
        routerList: []
      }
    }
  },
  methods: {
    changeTitle (params) {
      this.content = params
      if (this.content.title !== '' && this.content.routerList.length > 0) {
        bus.$emit('show-sidebar', false)
        bus.$emit('sub-sidebar-opened', true)
      } else if (this.content.routerList.length === 0) {
        bus.$emit('show-sidebar', true)
        bus.$emit('sub-sidebar-opened', false)
      }
    }
  },
  computed: {
    showSubSidebar: {
      get: function () {
        if (this.content.title !== '' && this.content.routerList.length > 0) {
          bus.$emit('show-sidebar', false)
          bus.$emit('sub-sidebar-opened', true)
          return true
        } else if (this.content.routerList.length === 0) {
          bus.$emit('show-sidebar', true)
          bus.$emit('sub-sidebar-opened', false)
          return false
        }
        return false
      }
    },
    ...mapGetters([
      'getTemplates'
    ])
  },
  beforeDestroy () {
    bus.$emit('sub-sidebar-opened', false)
  },
  created () {
    bus.$on('set-sub-sidebar-title', async (params) => {
      let isProjectList = false
      if (params.routerList && params.routerList.length > 2 && params.routerList[1].name === '集成环境') {
        isProjectList = true
      }
      if (isProjectList) {
        let findPro = false

        const findTem = this.getTemplates.filter(tem => {
          return tem.product_name === params.title
        })
        if (findTem.length === 0) {
          await this.$store.dispatch('refreshProjectTemplates')
        }

        for (const product of this.getTemplates) {
          if (product.product_name === params.title) {
            findPro = true
            if (product.product_feature && product.product_feature.develop_habit === 'yaml') {
              params.routerList.splice(2)
              break
            }
          }
        }
        this.changeTitle(params)
        if (!findPro) {
          // eslint-disable-next-line no-throw-literal
          throw ('Not find the product from getTemplates')
        }
      } else {
        this.changeTitle(params)
      }
    })
  }
}
</script>

<style lang="less">
// @import url("~@assets/css/common/scroll-bar.less");

.sub-side-bar-wrapper {
  position: relative;
  left: 0;
  height: 100%;
  background: transparent;
  transition: width 0.2s;

  &.sub-side-bar-wrapper-opened {
    left: 0;
    width: 130px;
  }

  &.sub-side-bar-wrapper-closed {
    position: absolute;
    left: -100%;
    width: 1px;
    height: 1px;
    overflow: hidden;
    appearance: none;
    clip: rect(0 0 0 0);
  }

  .sub-side-bar {
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    z-index: 1010;
    height: 100%;
    transition: opacity, 400ms linear, -webkit-transform;
    transition: transform, opacity, 400ms linear;
    transition: transform, opacity, 400ms linear, -webkit-transform;

    .sub-side-bar-inner {
      width: 100%;
      height: 100%;
      transition: -webkit-transform 0.2s;
      transition: transform 0.2s;
      transition: transform 0.2s, -webkit-transform 0.2s;

      .pipelines-sub-side-bar__wrap {
        display: flex;
        flex: 1;
        flex-direction: column;
        height: 100%;
        background-color: #f5f7fa;
        box-shadow: 10px 0 20px rgba(0, 0, 0, 0.12);
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;

        .pipelines-sub-side-bar__header {
          flex-basis: 60px;

          .pipelines-sub-side-bar__section-label {
            padding-bottom: 12px;
            padding-left: 12px;
            color: #888;
            font-weight: bold;
            font-size: 10px;
            text-transform: uppercase;
          }

          .cf-registry-selector__preview {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            height: 50px;
            padding: 5px 30px 5px 12px;
            color: #bebebe;
            font-weight: bold;
            font-size: 14px;
            text-transform: uppercase;

            .cf-registry-selector__text {
              display: inline-block;
              overflow: hidden;
              color: #888;
              white-space: nowrap;
              text-overflow: ellipsis;

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
            margin: 0;
            padding: 0;
            list-style-type: none;

            .pipelines-sub-side-bar__item {
              display: flex;
              align-items: center;
              justify-content: space-between;
              height: 35px;
              padding: 5px 20px;
              color: #eee;
              font-size: 12px;
              transition: background-color 150ms ease, color 150ms ease;
              -webkit-box-pack: justify;
              -ms-flex-pack: justify;

              &.selected {
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
              }

              .pipeline-name {
                flex: 1;
                min-width: 0;
                overflow: hidden;
                color: #434548;
                font-weight: 500;
                line-height: 16px;
                white-space: nowrap;
                text-overflow: ellipsis;
              }

              &:hover {
                padding-right: 2px;
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
                cursor: pointer;
              }
            }

            .selected {
              .pipelines-sub-side-bar__item {
                background-color: #e1edfa;
                box-shadow: inset 4px 0 0 #1989fa;
              }
            }
          }
        }

        .pipelines-sub-side-bar__footer {
          display: flex;
          flex-basis: 60px;
          flex-shrink: 0;
          align-items: center;
          justify-content: flex-start;
          padding-left: 12px;
          background-color: #fff;
          box-shadow: 0 -6px 4px rgba(0, 0, 0, 0.04);
          -ms-flex-negative: 0;
        }
      }
    }
  }
}
</style>
