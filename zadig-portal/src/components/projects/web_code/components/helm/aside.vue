<template>
  <div class="helm-aside-container">
    <div class="pipelines__aside-right--resizable">
    </div>
    <div class="aside__inner">
      <div class="aside-bar">
        <div class="tabs__wrap tabs__wrap_vertical">
          <div class="tabs__item"
               :class="{'selected': $route.query.rightbar === 'var'}"
               @click="changeRoute('var')">
            <span class="step-name">镜像更新</span>
          </div>
          <div class="tabs__item"
               :class="{'selected': $route.query.rightbar === 'help'}"
               @click="changeRoute('help')">
            <span class="step-name">帮助</span>
          </div>
        </div>
      </div>
      <div class="aside__content">
        <div v-if="$route.query.rightbar === 'var'"
             class="pipelines__aside--variables">
          <header class="pipeline-workflow-box__header">
            <div class="pipeline-workflow-box__title">镜像更新</div>
          </header>
         <div class="pipeline-workflow-box__content">
          <section>
              <h4>
                <span><i class="iconfont iconfuwu"></i></span> 检测到的服务组件
                <el-tooltip effect="dark"
                            placement="top">
                  <div slot="content">构建镜像标签生成规则 ：<br />选择 Tag 进行构建 ： 构建时间戳 -
                    Tag<br />只选择分支进行构建：构建时间戳
                    - 任务 ID - 分支名称<br />选择分支和 PR 进行构建：构建时间戳 - 任务 ID - 分支名称 - PR ID<br />只选择 PR
                    进行构建：构建时间戳 - 任务 ID - PR ID</div>
                  <span><i class="el-icon-question"></i></span>
                </el-tooltip>
              </h4>
              <!-- <div v-if="allRegistry.length === 0"
                   class="registry-alert">
                <el-alert title="私有镜像仓库未集成，请联系系统管理员前往「系统设置 -> 镜像仓库」进行集成"
                          type="warning">
                </el-alert>
              </div> -->
              <el-table :data="serviceModules"
                        stripe
                        style="width: 100%;">
                <el-table-column prop="name"
                                 label="服务组件">
                </el-table-column>
                <el-table-column prop="image"
                                 label="当前镜像版本">
                </el-table-column>
                <el-table-column label="构建信息/操作">
                  <template slot-scope="scope">
                    <!-- <router-link v-if="scope.row.build_name"
                                 :to="`${buildBaseUrl}?rightbar=build&service_name=${scope.row.name}&build_name=${scope.row.build_name}`"> -->
                      <el-button size="small"  v-if="scope.row.build_name"  @click="editBuild(scope.row.name, scope.row.build_name)"
                                 type="text">{{scope.row.build_name}}</el-button>
                    <!-- </router-link> -->
                    <!-- <router-link v-else
                                 :to="`${buildBaseUrl}?rightbar=build&service_name=${scope.row.name}&build_add=true`"> -->
                      <el-button size="small"  v-else
                                 type="text" @click="addBuild(scope.row.name)">添加构建</el-button>
                    <!-- </router-link> -->

                  </template>

                </el-table-column>
              </el-table>
            </section>
         </div>
          <!-- <div class="pipeline-workflow-box__content" v-if="showBuild">
            <build ref="buildRef"
                   :serviceName="serviceName"
                   :name="name"
                   :buildName="buildName"
                   :isEdit="isEdit"
                   :getServiceModules="getServiceModules"
                   ></build>
          </div> -->
        </div>
        <div v-if="$route.query.rightbar === 'help'"
             class="pipelines__aside--variables">
          <header class="pipeline-workflow-box__header">
            <div class="pipeline-workflow-box__title">帮助</div>
          </header>
          <div class="pipelines-aside-help__content">
            <help></help>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import qs from 'qs'
import bus from '@utils/event_bus'
import { mapState } from 'vuex'
import help from './help.vue'
import aceEditor from 'vue2-ace-bind'
import 'brace/mode/yaml'
import 'brace/theme/xcode'
import 'brace/ext/searchbox'
export default {
  props: {
    changeExpandFileList: Function
  },
  data () {
    return {
      editorOption: {
        showLineNumbers: false,
        showFoldWidgets: false,
        showGutter: false,
        showPrintMargin: false,
        readOnly: true,
        tabSize: 2,
        maxLines: Infinity,
        highlightActiveLine: false,
        highlightGutterLine: false,
        displayIndentGuides: false
      },
      allRegistry: [],
      chartValues: [],
      detectedServices: [],
      showBuild: false,
      serviceName: null,
      name: null,
      buildName: null,
      isEdit: false
    }
  },
  methods: {
    addBuild (name) {
      const item = {
        id: name,
        type: 'components',
        componentsName: 'Build',
        label: '新增构建',
        name: name,
        isEdit: false
      }
      this.changeExpandFileList('add', item)
    },
    editBuild (name, buildName) {
      const item = {
        id: name,
        type: 'components',
        componentsName: 'Build',
        label: '修改构建',
        name: name,
        isEdit: true,
        buildName: buildName
      }
      this.changeExpandFileList('add', item)
    },
    changeRoute (step) {
      this.$router.replace({
        query: Object.assign(
          {},
          qs.parse(window.location.search, { ignoreQueryPrefix: true }),
          {
            rightbar: step
          })
      })
    },
    editorInit (e) {
      e.renderer.$cursorLayer.element.style.opacity = 0
    }
  },
  beforeDestroy () {
    bus.$off('save-var')
  },
  computed: {
    projectName () {
      return this.$route.params.project_name
    },
    ...mapState({
      serviceModules: (state) => state.service_manage.serviceModules
    }),
    serviceType () {
      return this.service.type
    }
  },
  components: {
    help,
    editor: aceEditor
  }
}
</script>
<style lang="less">
.helm-aside-container {
  position: relative;
  display: flex;
  -webkit-box-flex: 1;
  -ms-flex: 1;
  flex: 1;
  height: 100%;

  .kv-container {
    .el-table {
      .unused {
        background: #e6effb;
      }

      .present {
        background: #fff;
      }

      .new {
        background: oldlace;
      }
    }

    .el-table__row {
      .cell {
        span {
          font-weight: 400;
        }

        .operate {
          font-size: 1.12rem;

          .delete {
            color: #ff1949;
          }

          .edit {
            color: #1989fa;
          }
        }
      }
    }

    .render-value {
      display: block;
      max-width: 100%;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }

    .add-key-container {
      .el-form-item__label {
        display: none;
      }

      .el-form-item {
        margin-bottom: 15px;
      }
    }

    .add-kv-btn {
      margin-top: 10px;
    }
  }

  .pipelines__aside-right--resizable {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 99;
    width: 5px;
    height: 100%;
    border-left: 1px solid transparent;
    -webkit-transition: border-color ease-in-out 200ms;
    transition: border-color ease-in-out 200ms;

    .capture-area__component {
      position: relative;
      top: 50%;
      left: -6px;
      display: inline-block;
      height: 38px;
      -webkit-transform: translateY(-50%);
      transform: translateY(-50%);

      .capture-area {
        position: absolute;
        width: 10px;
        height: 38px;
        background-color: #fff;
        border: 1px solid #dbdbdb;
        border-radius: 5px;
      }
    }
  }

  .aside__inner {
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    -ms-flex: 1;
    flex: 1;
    -ms-flex-direction: row-reverse;
    flex-direction: row-reverse;
    box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
    -webkit-box-orient: horizontal;
    -webkit-box-direction: reverse;
    -webkit-box-flex: 1;

    .aside__content {
      -ms-flex: 1;
      flex: 1;
      width: 200px;
      overflow-x: hidden;
      background-color: #fff;
      -webkit-box-flex: 1;

      .pipelines__aside--variables {
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        -ms-flex-direction: column;
        flex-direction: column;
        flex-grow: 1;
        height: 100%;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        -webkit-box-flex: 1;
        -ms-flex-positive: 1;

        .pipeline-workflow-box__header {
          display: flex;
          flex-shrink: 0;
          align-items: center;
          justify-content: space-between;
          width: 100%;
          height: 35px;
          padding: 10px 7px 10px 20px;
          -webkit-box-pack: justify;
          -ms-flex-pack: justify;
          -webkit-box-align: center;
          -ms-flex-align: center;
          -ms-flex-negative: 0;

          .pipeline-workflow-box__title {
            margin-right: 20px;
            margin-bottom: 0;
            color: #000;
            font-weight: bold;
            font-size: 16px;
            text-transform: uppercase;
          }
        }

        .pipeline-workflow-box__content {
          flex-grow: 1;
          -webkit-box-flex: 1;
          -ms-flex-positive: 1;

          section {
            position: relative;
            padding: 12px 16px;

            h4 {
              margin: 0;
              padding: 0;
              color: #909399;
              font-weight: 300;
            }

            .el-table td,
            .el-table th {
              padding: 6px 0;
            }

            .yaml-container {
              margin: 5px 0;

              .info {
                margin: 8px 0;

                .name,
                .version {
                  color: #303133;
                  font-size: 14px;
                }
              }

              .editor-container {
                border: 1px solid #ccc;
                border-radius: 4px;
              }
            }
          }
        }

        .pipelines-aside-help__content {
          display: flex;
          -ms-flex: 1;
          flex: 1;
          flex-direction: column;
          height: 100%;
          padding: 0 20px 10px 20px;
          overflow-y: auto;
          -webkit-box-flex: 1;
        }
      }

      .btn-container {
        padding: 0 10px 10px 10px;
        box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);

        .save-btn {
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
      }
    }

    .aside-bar {
      .tabs__wrap_vertical {
        -ms-flex-direction: column;
        flex-direction: column;
        width: 47px;
        height: 100%;
        background-color: #f5f5f5;
        border: none;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;

        .tabs__item {
          position: relative;
          display: -webkit-box;
          display: -ms-flexbox;
          display: flex;
          align-items: center;
          margin-bottom: -1px;
          padding: 17px 30px;
          padding: 20px 17px;
          color: #4c4c4c;
          color: #000;
          font-size: 13px;
          text-transform: uppercase;
          text-orientation: mixed;
          background-color: transparent;
          background-color: #f5f5f5;
          border: none;
          border-top: 1px solid transparent;
          cursor: pointer;
          -webkit-transition: background-color 150ms ease, color 150ms ease;
          transition: background-color 150ms ease, color 150ms ease;
          -webkit-box-align: center;
          -ms-flex-align: center;

          &.selected {
            z-index: 1;
            background-color: #fff;
            border: none;
            -webkit-box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
            box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
          }

          &:hover {
            z-index: 2;
            color: #000;
            background-color: #fff;
            border: none;
            border-top: 1px solid #f5f5f5;
            -webkit-box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
            box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
          }

          .step-name {
            font-weight: 500;
            font-size: 14px;
          }
        }
      }

      .tabs__wrap {
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        justify-content: flex-start;
        height: 56px;
        -webkit-box-pack: start;
        -ms-flex-pack: start;
      }
    }
  }
}
</style>
