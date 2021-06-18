<template>
  <div class="project-home-container">
    <div class="project-header">
      <div class="header-start">
        <div class="container">
          <div class="display-mode">
            <div class="btn-container">
              <button type="button"
                      @click="currentTab = 'grid'"
                      :class="{'active':currentTab==='grid'?true:false}"
                      class="display-btn">
                <i class="el-icon-s-grid"></i>
                <span class="add-filter-value-title">网格模式</span>
              </button>
              <button type="button"
                      @click="currentTab = 'list'"
                      :class="{'active':currentTab==='list'?true:false}"
                      class="display-btn">
                <i class="el-icon-s-fold"></i>
                <span class="add-filter-value-title">列表模式</span>
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="header-end">
        <router-link to="/v1/projects/create">
          <button type="button"
                  class="add-project-btn">
            <i class="el-icon-plus"></i>
            新建项目
          </button>
        </router-link>
      </div>
    </div>
    <div v-if="currentTab==='grid'"
         v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconxiangmuloading"
         class="projects-grid">
      <el-row :gutter="12">
        <el-col v-for="(project,index) in projects"
                :key="index"
                :span="6">
          <el-card shadow="hover"
                   class="project-card">
            <span class="operations">
              <el-dropdown @command="handleCommand"
                           trigger="click">
                <span class="el-dropdown-link">
                  <i class="el-icon-more"></i></span>
                <el-dropdown-menu slot="dropdown">
                  <el-dropdown-item :command="{action:'edit',project_name:project.product_name}">
                    修改
                  </el-dropdown-item>
                  <el-dropdown-item :command="{action:'delete',project_name:project.product_name}">
                    删除</el-dropdown-item>
                </el-dropdown-menu>
              </el-dropdown>
            </span>
            <div @click="toProject(project)"
                 class="content-container">
              <div class="content">
                <div class="card-header">
                  <div class="quickstart-icon">
                    <span>{{project.product_name.slice(0, 1).toUpperCase()}}</span>
                  </div>
                  <div class="card-text">
                    <h4 class="project-name">
                      {{project.project_name?project.project_name:project.product_name}}
                    </h4>
                  </div>
                  <div class="info">
                    <span class="project-desc">{{project.desc}}</span>
                  </div>
                </div>
              </div>
            </div>
            <div class="footer">
              <div class="module">
                <el-tooltip effect="dark"
                            content="工作流"
                            placement="top">
                  <router-link :to="`/v1/projects/detail/${project.product_name}/pipelines`">
                    <span class="icon iconfont icongongzuoliucheng"></span>
                  </router-link>
                </el-tooltip>
                <el-tooltip effect="dark"
                            content="构建管理"
                            placement="top">
                  <router-link :to="`/v1/projects/detail/${project.product_name}/builds`">
                    <span class="icon iconfont icongoujianzhong"></span>
                  </router-link>
                </el-tooltip>
                <el-tooltip effect="dark"
                            content="测试管理"
                            placement="top">
                  <router-link :to="`/v1/projects/detail/${project.product_name}/test`">
                    <span class="icon iconfont icontest"></span>
                  </router-link>
                </el-tooltip>
                <el-tooltip effect="dark"
                            content="查看服务"
                            placement="top">
                  <router-link :to="`/v1/projects/detail/${project.product_name}/services`">
                    <span class="icon iconfont iconrongqifuwu"></span>
                  </router-link>
                </el-tooltip>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
      <div v-if="projects.length === 0"
           class="no-product">
        <img src="@assets/icons/illustration/product.svg"
             alt="">
        <p>暂无可展示的项目，请手动添加项目</p>
      </div>
    </div>
    <div v-if="currentTab==='list'"
         v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconxiangmuloading"
         class="projects-list">
      <el-table v-if="projects.length > 0"
                :data="projects"
                stripe
                style="width: 100%;">
        <el-table-column label="项目名称">
          <template slot-scope="scope">
            <router-link :to="`/v1/projects/detail/${scope.row.product_name}`"
                         class="project-name">
              {{scope.row.project_name?scope.row.project_name:scope.row.product_name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column prop="total_service_num"
                         label="服务数量">
        </el-table-column>
        <el-table-column prop="total_env_num"
                         label="集成环境">
        </el-table-column>
        <el-table-column label="更新信息">
          <template slot-scope="scope">
            <div><i class="el-icon-time"></i> {{ $utils.convertTimestamp(scope.row.update_time) }}
            </div>
            <div><i class="el-icon-user"></i> {{ scope.row.update_by }}</div>
          </template>
        </el-table-column>
        <el-table-column label="">
          <template slot-scope="scope">
            <router-link :to="`/v1/projects/detail/${scope.row.product_name}`">
              <el-button class="operation"
                         type="text">配置</el-button>
            </router-link>
            <el-button @click="deleteProject(scope.row.product_name)"
                       class="operation"
                       type="text">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="projects.length === 0"
           class="no-product">
        <img src="@assets/icons/illustration/product.svg"
             alt="">
        <p>暂无可展示的项目，请手动添加项目</p>
      </div>
    </div>
  </div>
</template>
<script>
import bus from '@utils/event_bus'
import { getProjectsAPI, getBuildConfigsAPI, getSingleProjectAPI, deleteProjectAPI } from '@api'
import _ from 'lodash'
import { mapGetters } from 'vuex'
export default {
  data () {
    return {
      projects: [],
      loading: false,
      currentTab: 'grid',
      currentProjectName: ''
    }
  },
  methods: {
    getProjects () {
      this.loading = true
      getProjectsAPI().then(
        response => {
          this.projects = this.$utils.deepSortOn(response, 'product_name')
          this.loading = false
        }
      )
    },
    toProject (project) {
      this.$router.push(`/v1/projects/detail/${project.product_name}`)
    },
    exitGuideModal () {
      this.$intro().exit()
    },
    handleCommand (command) {
      if (command.action === 'delete') {
        this.deleteProject(command.project_name)
      } else if (command.action === 'edit') {
        this.$router.push(`/v1/projects/edit/${command.project_name}`)
      }
    },
    deleteProject (projectName) {
      let services; let buildConfigs; let allWorkflows = []
      const workflows = (this.workflowList.filter(w => w.product_tmpl_name === projectName)).map((element) => { return element.name })
      const envNames = (this.productList.filter(p => p.product_name === projectName)).map((element) => { return element.env_name })
      allWorkflows = workflows
      getSingleProjectAPI(projectName).then((res) => {
        services = _.flattenDeep(res.services)
      }).then(() => {
        getBuildConfigsAPI(projectName).then((res) => {
          buildConfigs = res.map((element) => { return element.name })
          const htmlTemplate = `
      <span><b>服务：</b>${services.length > 0 ? services.join(', ') : '无'}</span><br>
      <span><b>构建：</b>${buildConfigs.length > 0 ? buildConfigs.join(', ') : '无'}</span><br>
      <span><b>环境：</b>${envNames.length > 0 ? envNames.join(', ') : '无'}</span><br>
      <span><b>工作流：</b>${allWorkflows.length > 0 ? allWorkflows.join(', ') : '无'}</span>
      `
          this.$prompt(`该项目下的资源会同时被删除<span style="color:red">请谨慎操作！！</span><br> ${htmlTemplate}`, `请输入项目名 ${projectName} 确认删除`, {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            dangerouslyUseHTMLString: true,
            customClass: 'product-prompt',
            confirmButtonClass: 'el-button el-button--danger',
            inputValidator: project_name => {
              if (project_name === projectName) {
                return true
              } else if (project_name === '') {
                return '请输入项目名'
              } else {
                return '项目名不相符'
              }
            }
          })
            .then(({ value }) => {
              deleteProjectAPI(projectName).then(
                response => {
                  this.$message({

                    type: 'success',
                    message: '项目删除成功'
                  })
                  this.getProjects()
                }
              )
            })
            .catch(() => {
              this.$message({
                type: 'info',
                message: '取消删除'
              })
            })
        })
      })
    }
  },
  computed: {
    ...mapGetters([
      'productList', 'workflowList'
    ])
  },
  beforeDestroy () {
    this.exitGuideModal()
  },
  mounted () {
    this.$store.dispatch('getProductListSSE').closeWhenDestroy(this)
    this.$store.dispatch('refreshWorkflowList')
    this.getProjects()
    bus.$emit('show-sidebar', true)
    bus.$emit('set-topbar-title', { title: '项目', breadcrumb: [] })
    bus.$emit('set-sub-sidebar-title', {
      title: '',
      routerList: []
    })
  }
}
</script>

<style lang="less" >
.show-guide-class {
  min-width: 200px !important;

  span {
    padding: 3px;
    color: #303133;
    font-size: 14px;
  }

  .introjs-donebutton {
    color: #333;

    &:focus {
      border: 2px solid #1989fa;
    }
  }
}

.project-home-container {
  position: relative;
  flex: 1;
  overflow: auto;
  background-color: #f5f7f7;

  .no-product {
    display: flex;
    flex-direction: column;
    align-content: center;
    align-items: center;
    justify-content: center;
    height: 70%;

    img {
      width: 400px;
      height: 400px;
    }

    p {
      color: #606266;
      font-size: 15px;
    }
  }

  .project-header {
    display: flex;
    align-items: stretch;
    justify-content: flex-start;

    .header-start {
      flex: 1;

      .container {
        min-height: 50px;
        margin: 0;
        padding: 10px 20px;
        font-size: 13px;

        .display-mode {
          display: flex;
          flex-wrap: wrap;
          align-items: baseline;
          justify-content: flex-start;
          min-height: 46px;

          .btn-container {
            position: relative;
            height: 44px;
            margin-top: 1px;
            margin-right: 5px;

            .display-btn {
              padding: 13px 17px;
              color: #1989fa;
              font-size: 13px;
              text-decoration: none;
              background-color: #fff;
              border: none;
              border-color: #fff;
              border-style: none;
              border-radius: 2px;
              box-shadow: 0 4px 4px rgba(0, 0, 0, 0.05);
              cursor: pointer;

              &:hover {
                color: #1989fa;
                background-color: #fff;
                border-color: #1989fa;
              }

              &.active {
                color: #fff;
                background-color: #1989fa;
                border-color: #1989fa;
              }

              &.round {
                margin-left: 20px;
                border-radius: 20px;
              }
            }
          }
        }
      }
    }

    .header-end {
      .add-project-btn {
        width: 165px;
        height: 100%;
        padding: 10px 17px;
        color: #fff;
        font-size: 13px;
        text-decoration: none;
        background-color: #1989fa;
        border: 1px solid #1989fa;
        cursor: pointer;
      }
    }
  }

  .projects-list {
    height: 100%;
    padding: 0 20px;

    .el-table {
      tr {
        height: 71px;
      }

      .project-name {
        color: #4c4c4c;
        font-weight: 400;
        font-size: 16px;
        text-align: left;
      }

      .operation {
        margin-right: 10px;
        color: #606266;

        &:hover {
          color: #1989fa;
        }
      }
    }
  }

  .projects-grid {
    height: 100%;
    padding: 0 20px;

    .project-card {
      height: 135px;
      margin-bottom: 15px;
      border: 2px solid #fff;
      border-radius: 3px;
      box-shadow: 0 2px 7px 0 rgba(0, 0, 0, 0.1);

      &:hover {
        border-color: #1989fa;
        cursor: pointer;

        .quickstart-icon span {
          background-color: #1989fa !important;
        }
      }

      .el-card__body {
        position: relative;
        display: flex;
        flex-direction: column;
        height: 100%;
        padding: 0;

        &.add {
          font-size: 30px;
          text-align: center;

          .text {
            margin: auto 0;
            padding: 40px;

            a {
              color: #7a8599;
            }

            span {
              cursor: pointer;
            }
          }
        }

        .operations {
          position: absolute;
          top: 8px;
          right: 15px;
          display: flex;
          cursor: pointer;

          i {
            font-size: 20px;
            line-height: 25px;
          }
        }

        .footer {
          display: flex;
          flex-direction: row;
          align-self: flex-end;
          justify-content: flex-end;
          width: 100%;
          height: 35px;
          border-top: 1px solid #ebeef5;

          .icon {
            margin: 0 5px;
            color: #606266;
            font-size: 25px;
            line-height: 35px;
            cursor: pointer;

            &:hover {
              color: #1989fa;
            }
          }

          .operation {
            border-left: 2px solid #ebeef5;
          }
        }

        .content-container {
          flex: 1;
          height: calc(~"100% - 55px");
          padding: 15px 15px 0 15px;

          .content {
            display: flex;
            flex-direction: row;
            height: 100%;

            .card-header {
              .quickstart-icon {
                display: inline-block;
                margin-bottom: 15px;

                span {
                  display: inline-block;
                  width: 20px;
                  height: 20px;
                  color: #fff;
                  font-size: 18px;
                  line-height: 20px;
                  text-align: center;
                  background-color: #999;
                  border-radius: 50%;
                }
              }

              .card-text {
                display: inline-block;
                margin-left: 4px;
              }

              .divider {
                width: 278px;
                height: 1px;
                margin-top: 14px;
                margin-bottom: 8px;
                background-color: #ccc;
              }

              .project-name {
                margin: 0;
                padding: 0;
                color: #4c4c4c;
                font-weight: 400;
                font-size: 20px;
                text-overflow: ellipsis;
                cursor: pointer;
              }

              .project-desc {
                display: inline-block;
                margin-top: 12px;
                font-size: 14px;
              }
            }

            .icon {
              margin-right: 15px;
            }

            .info {
              .project-name {
                margin: 0;
                padding: 0;
                color: #1989fa;
                font-size: 18px;
                text-overflow: ellipsis;
                cursor: pointer;
              }

              .project-desc {
                display: inline-block;
                margin-top: 12px;
                font-size: 14px;
              }
            }
          }
        }
      }
    }
  }

  .projects-grid,
  .projects-list {
    .show-tag {
      color: #1989fa;
      font-size: 12px;
      white-space: nowrap;
      vertical-align: top;
    }
  }
}

.el-message-box.product-prompt {
  width: 40%;

  .el-message-box__content {
    max-height: 300px;
    overflow-y: auto;
  }
}
</style>
