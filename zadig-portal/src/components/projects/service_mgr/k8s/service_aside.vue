<template>
  <div class="aside__wrap">
    <el-drawer title="代码源集成"
               :visible.sync="addCodeDrawer"
               direction="rtl">
      <add-code @cancel="addCodeDrawer = false"></add-code>
    </el-drawer>
    <div class="aside__inner">
      <div class="aside-bar">
        <div class="tabs__wrap tabs__wrap_vertical">
          <div class="tabs__item"
               :class="{'selected': $route.query.rightbar === 'build'}"
               @click="changeRoute('build')"
               style="">
            <span class="step-name">构建</span>
          </div>
          <div class="tabs__item"
               :class="{'selected': $route.query.rightbar === 'var'}"
               @click="changeRoute('var')"
               style="">
            <span class="step-name">变量</span>
          </div>
          <div class="tabs__item"
               :class="{'selected': $route.query.rightbar === 'help'}"
               @click="changeRoute('help')"
               style="">
            <span class="step-name">帮助</span>
          </div>
        </div>
      </div>
      <div class="aside__content">
        <div v-if="$route.query.rightbar === 'build'"
             class="pipelines__aside--variables">
          <header class="pipeline-workflow-box__header">
            <div class="pipeline-workflow-box__title">构建</div>
          </header>
          <div class="pipeline-workflow-box__content">
            <build ref="buildRef"
                   @getServiceModules="getServiceModules"
                   :detectedServices="detectedServices"></build>
          </div>
            <div class="btn-container">
              <el-button type="primary"
                         size="small"
                         @click="saveBuildConfig"
                         class="save-btn"
                         plain>保存构建
              </el-button>
            </div>
        </div>
        <div v-if="$route.query.rightbar === 'var'"
             class="pipelines__aside--variables">
          <header class="pipeline-workflow-box__header">
            <div class="pipeline-workflow-box__title">变量</div>
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
              <div v-if="allRegistry.length === 0"
                   class="registry-alert">
                <el-alert title="私有镜像仓库未集成，请联系系统管理员前往系统设置-> Registry 管理  进行集成"
                          type="warning">
                </el-alert>
              </div>
              <el-table :data="serviceModules"
                        stripe
                        style="width: 100%">
                <el-table-column prop="name"
                                 label="服务组件">
                </el-table-column>
                <el-table-column prop="image"
                                 label="当前镜像版本">
                </el-table-column>
                <el-table-column label="构建信息/操作">
                  <template slot-scope="scope">
                    <router-link v-if="scope.row.build_name"
                                 :to="`${buildBaseUrl}?rightbar=build&service_name=${scope.row.name}&build_name=${scope.row.build_name}`">
                      <el-button size="small"
                                 type="text">{{scope.row.build_name}}</el-button>
                    </router-link>
                    <el-button v-else
                               size="small"
                               @click="addBuild(scope.row)"
                               type="text">添加构建</el-button>

                  </template>

                </el-table-column>
              </el-table>
            </section>
            <section>
              <h4>
                <span><i class="iconfont icongongjuxiang"></i></span> 系统内置变量
                <el-tooltip effect="dark"
                            content="在服务配置中使用 $Namespace$，$Product$，$Service$，$EnvName$ 方式引用"
                            placement="top">
                  <span><i class="el-icon-question"></i></span>
                </el-tooltip>

              </h4>
              <el-table :data="sysEnvs"
                        stripe
                        style="width: 100%">
                <el-table-column prop="key"
                                 label="变量">
                </el-table-column>
                <el-table-column prop="value"
                                 label="当前值">
                  <template slot-scope="scope">
                    <span v-if="scope.row.value">{{scope.row.value}}</span>
                    <span v-else>空</span>
                  </template>
                </el-table-column>
              </el-table>
            </section>
            <section>
              <h4>
                <span><i class="iconfont icontanhao"></i></span> 自定义变量
                <el-tooltip effect="dark"
                            :content="'自定义变量通过'+' {{'+'.key}} ' +' 声明'"
                            placement="top">
                  <span><i class="el-icon-question"></i></span>
                </el-tooltip>
              </h4>
              <div class="kv-container">
                <el-table :data="customEnvs"
                          style="width: 100%">
                  <el-table-column label="Key">
                    <template slot-scope="scope">
                      <span>{{ scope.row.key }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="Value">
                    <template slot-scope="scope">
                      <el-input size="small"
                                :disabled="!editEnvIndex[scope.$index]"
                                v-model="scope.row.value"
                                type="textarea"
                                :autosize="{ minRows: 1, maxRows: 4}"
                                placeholder="请输入内容"></el-input>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作"
                                   width="150">
                    <template slot-scope="scope">
                        <span class="operate">
                              <el-button v-if="!editEnvIndex[scope.$index]"
                                         type="text"
                                         @click="editRenderKey(scope.$index,scope.row.state)"
                                         class="edit">编辑</el-button>
                              <el-button v-if="editEnvIndex[scope.$index]"
                                         type="text"
                                         @click="saveRenderKey(scope.$index,scope.row.state)"
                                         class="edit">保存</el-button>
                              <el-button v-if="scope.row.state === 'unused'"
                                         type="text"
                                         @click="deleteRenderKey(scope.$index,scope.row.state)"
                                         class="delete">移除</el-button>
                            <el-tooltip v-if="scope.row.state === 'present'||scope.row.state === 'new'"
                                        effect="dark"
                                        content="服务中已经用到的 Key 无法被删除"
                                        placement="top">
                              <span class="el-icon-question"></span>
                            </el-tooltip>
                          </span>
                    </template>
                  </el-table-column>
                </el-table>
                <div v-if="addKeyInputVisable"
                     class="add-key-container">
                  <el-table :data="addKeyData"
                            :show-header="false"
                            style="width: 100%">
                    <el-table-column>
                      <template slot-scope="scope">
                        <el-form :model="addKeyData[0]"
                                 :rules="keyCheckRule"
                                 ref="addKeyForm"
                                 hide-required-asterisk>
                          <el-form-item label="Key"
                                        prop="key"
                                        inline-message>
                            <el-input size="small"
                                      type="textarea"
                                      :autosize="{ minRows: 1, maxRows: 4}"
                                      v-model="addKeyData[0].key"
                                      placeholder="Key">
                            </el-input>
                          </el-form-item>
                        </el-form>
                      </template>
                    </el-table-column>
                    <el-table-column>
                      <template slot-scope="scope">
                        <el-form :model="addKeyData[0]"
                                 :rules="keyCheckRule"
                                 ref="addValueForm"
                                 hide-required-asterisk>
                          <el-form-item label="Value"
                                        prop="value"
                                        inline-message>
                            <el-input size="small"
                                      type="textarea"
                                      :autosize="{ minRows: 1, maxRows: 4}"
                                      v-model="addKeyData[0].value"
                                      placeholder="Value">
                            </el-input>
                          </el-form-item>
                        </el-form>
                      </template>
                    </el-table-column>
                    <el-table-column width="140">
                      <template slot-scope="scope">
                        <span style="display: inline-block;margin-bottom:15px">
                          <el-button @click="addRenderKey()"
                                     type="text">确认</el-button>
                          <el-button @click="addKeyInputVisable=false"
                                     type="text">取消</el-button>
                        </span>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>
                  <div>
                    <el-button size="medium"
                               class="add-kv-btn"
                               @click="addKeyInputVisable=true"
                               type="text">
                      <i class="el-icon-circle-plus-outline"></i>添加
                    </el-button>
                  </div>
              </div>
            </section>
          </div>
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
import qs from 'qs';
import bus from '@utils/event_bus';
import { serviceTemplateWithConfigAPI, getSingleProjectAPI, updateEnvTemplateAPI, getRegistryWhenBuildAPI, getCodeSourceByAdminAPI } from '@api';
import build from '../common/build.vue';
import help from './container/help.vue';
import addCode from '../common/add_code.vue';
let validateKey = (rule, value, callback) => {
  if (typeof value === 'undefined' || value == '') {
    callback(new Error('请输入 Key'));
  } else {
    if (!/^[a-zA-Z0-9_]+$/.test(value)) {
      callback(new Error('Key 只支持字母大小写和数字，特殊字符只支持下划线'));
    } else {
      callback();
    }
  }
};
export default {
  data() {
    return {
      allRegistry: [],
      serviceModules: this.detectedServices,
      sysEnvs: this.systemEnvs,
      customEnvs: this.detectedEnvs,
      addKeyInputVisable: false,
      addCodeDrawer: false,
      editEnvIndex: {},
      projectForm: {},
      addKeyData: [
        {
          key: '',
          value: '',
          state: 'unused'
        }
      ],
      keyCheckRule: {
        key: [
          {
            type: 'string',
            required: true,
            validator: validateKey,
            trigger: 'blur'
          }
        ],
        value: [
          {
            type: 'string',
            required: false,
            message: 'value',
            trigger: 'blur'
          }
        ]
      }
    }
  },
  methods: {
    async addBuild(item) {
      const res = await getCodeSourceByAdminAPI(1);
      if (res && res.length > 0) {
        this.$router.push(`${this.buildBaseUrl}?rightbar=build&service_name=${item.name}&build_add=true`);
      } else {
        this.addCodeDrawer = true;
      }
    },
    saveBuildConfig() {
      this.$refs.buildRef.updateBuildConfig();
    },
    getServiceModules() {
      this.$emit('getServiceModules');
    },
    getProject() {
      const projectName = this.projectName;
      getSingleProjectAPI(projectName).then((res) => {
        this.projectForm = res;
        if (res.team_id === 0) {
          this.projectForm.team_id = null;
        }
      });
    },
    getServiceTemplateWithConfig() {
      if (this.service && this.service.type === 'k8s' && this.service.status === 'added' && this.service.product_name === this.projectName) {
        this.changeRoute('var')
        serviceTemplateWithConfigAPI(this.service.service_name, this.projectName).then(res => {
          this.serviceModules = res.service_module;
          this.sysEnvs = res.system_variable;
          this.customEnvs = res.custom_variable;
        })
      }
    },
    changeRoute(step) {
      this.$router.replace({
        query: Object.assign(
          {},
          qs.parse(window.location.search, { ignoreQueryPrefix: true }),
          {
            rightbar: step
          })
      });
    },

    checkExistVars() {
      return new Promise((resolve, reject) => {
        const isDuplicate = this.detectedEnvs.map((item) => { return item.key }).some((item, idx) => {
          return this.detectedEnvs.map((item) => { return item.key }).indexOf(item) != idx
        });
        if (isDuplicate) {
          this.$message({
            message: '变量列表中存在相同的 Key 请检查后再保存',
            type: 'warning'
          });
          reject(new Error('cancel save'));
        }
        else {
          resolve();
        }
      });
    },
    updateEnvTemplate(projectName, payload, verbose) {
      updateEnvTemplateAPI(projectName, payload).then((res) => {
        bus.$emit('refresh-service');
        if (verbose) {
          this.$notify({
            title: '保存成功',
            message: '变量列表保存成功',
            type: 'success'
          });
        }
      });
    },
    addRenderKey() {
      if (this.addKeyData[0].key !== '') {
        this.$refs['addKeyForm'].validate(valid => {
          if (valid) {
            this.customEnvs.push(this.$utils.cloneObj(this.addKeyData[0]));
            this.projectForm.vars = this.customEnvs;
            this.checkExistVars()
              .then(() => {
                this.updateEnvTemplate(this.projectName, this.projectForm);
                this.addKeyData[0].key = '';
                this.addKeyData[0].value = '';

              })
              .catch(err => {
                this.addKeyData[0].key = '';
                this.addKeyData[0].value = '';
                this.$refs['addKeyForm'].resetFields();
                this.$refs['addValueForm'].resetFields();
                this.addKeyInputVisable = false;
                console.log('error');
              });

          } else {
            return false;
          }
        });
      }
    },
    editRenderKey(index, state) {
      this.$set(this.editEnvIndex, index, true);
    },
    saveRenderKey(index, state) {
      this.$set(this.editEnvIndex, index, false);
      this.projectForm.vars = this.customEnvs;
      this.updateEnvTemplate(this.projectName, this.projectForm);
    },
    deleteRenderKey(index, state) {
      if (state === 'present') {
        this.$confirm('该 Key 被产品引用，确定删除', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.customEnvs.splice(index, 1);
          this.projectForm.vars = this.customEnvs;
          this.updateEnvTemplate(this.projectName, this.projectForm);
        }).catch(() => {
          this.$message({
            type: 'info',
            message: '已取消删除'
          });
        });
      }
      else {
        this.customEnvs.splice(index, 1);
        this.projectForm.vars = this.customEnvs;
        this.updateEnvTemplate(this.projectName, this.projectForm);
      }
    },

  },
  created() {
    this.getProject();
    this.getServiceTemplateWithConfig();
    bus.$on(`save-var`, () => {
      this.projectForm.vars = this.detectedEnvs;
      this.updateEnvTemplate(this.projectName, this.projectForm);
    });
    getRegistryWhenBuildAPI().then((res) => {
      this.allRegistry = res;
    });
  },
  beforeDestroy() {
    bus.$off('save-var');
  },
  props: {
    detectedEnvs: {
      required: false,
      type: Array
    },
    detectedServices: {
      required: false,
      type: Array
    },
    systemEnvs: {
      required: false,
      type: Array
    },
    service: {
      required: false,
      type: Object
    },
    buildBaseUrl: {
      required: true,
      type: String
    }

  },
  watch: {
    detectedServices(val) {
      this.serviceModules = val;
    },
    systemEnvs(val) {
      this.sysEnvs = val;
    },
    detectedEnvs(val) {
      this.customEnvs = val;
    },
    service(val) {
      if (val) {
        this.getServiceTemplateWithConfig();
      }

    }
  },
  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
    serviceType() {
      return "k8s";
    }
  },
  components: {
    build, help,
    'add-code': addCode
  },
}
</script>
<style lang="less">
.aside__wrap {
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
      max-width: 100%;
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
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
    left: 0;
    top: 0;
    height: 100%;
    width: 5px;
    z-index: 99;
    border-left: 1px solid transparent;
    -webkit-transition: border-color ease-in-out 200ms;
    transition: border-color ease-in-out 200ms;
    .capture-area__component {
      top: 50%;
      left: -6px;
      -webkit-transform: translateY(-50%);
      transform: translateY(-50%);
      display: inline-block;
      height: 38px;
      position: relative;
      .capture-area {
        border: 1px solid #dbdbdb;
        background-color: #fff;
        width: 10px;
        height: 38px;
        border-radius: 5px;
        position: absolute;
      }
    }
  }
  .aside__inner {
    display: -webkit-box;
    display: -ms-flexbox;
    display: flex;
    -webkit-box-orient: horizontal;
    -webkit-box-direction: reverse;
    -ms-flex-direction: row-reverse;
    flex-direction: row-reverse;
    -webkit-box-flex: 1;
    -ms-flex: 1;
    flex: 1;
    box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
    .aside__content {
      -webkit-box-flex: 1;
      -ms-flex: 1;
      flex: 1;
      background-color: #fff;
      width: 200px;
      overflow-x: hidden;
      .pipelines__aside--variables {
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        -ms-flex-direction: column;
        flex-direction: column;
        -webkit-box-flex: 1;
        -ms-flex-positive: 1;
        flex-grow: 1;
        height: 100%;
        .pipeline-workflow-box__header {
          display: flex;
          -webkit-box-pack: justify;
          -ms-flex-pack: justify;
          justify-content: space-between;
          -webkit-box-align: center;
          -ms-flex-align: center;
          align-items: center;
          width: 100%;
          height: 35px;
          padding: 10px 7px 10px 20px;
          -ms-flex-negative: 0;
          flex-shrink: 0;
          .pipeline-workflow-box__title {
            color: #000000;
            font-weight: bold;
            font-size: 16px;
            text-transform: uppercase;
            margin-right: 20px;
            margin-bottom: 0;
          }
        }
        .pipeline-workflow-box__content {
          -webkit-box-flex: 1;
          -ms-flex-positive: 1;
          flex-grow: 1;
          overflow-y: auto;
          overflow-x: hidden;
          section {
            position: relative;
            padding: 12px 16px;
            h4 {
              margin: 0;
              padding: 0;
              font-weight: 300;
              color: #909399;
            }
            .el-table td,
            .el-table th {
              padding: 6px 0;
            }
          }
        }
        .pipelines-aside-help__content {
          display: -webkit-box;
          display: -ms-flexbox;
          display: flex;
          height: 100%;
          -webkit-box-flex: 1;
          -ms-flex: 1;
          flex: 1;
          overflow-y: auto;
          -webkit-box-orient: vertical;
          -webkit-box-direction: normal;
          -ms-flex-direction: column;
          flex-direction: column;
          padding: 0 20px 10px 20px;
          overflow-y: auto;
        }
      }
      .btn-container {
        padding: 0px 10px 10px 10px;
        box-shadow: 0 4px 4px 0 rgba(0, 0, 0, 0.05);
        .save-btn {
          text-decoration: none;
          background-color: #1989fa;
          color: #fff;
          padding: 10px 17px;
          border: 1px solid #1989fa;
          font-size: 13px;
          font-weight: bold;
          transition: background-color 300ms, color 300ms, border 300ms;
          cursor: pointer;
        }
      }
    }
    .aside-bar {
      .tabs__wrap_vertical {
        -webkit-box-orient: vertical;
        -webkit-box-direction: normal;
        -ms-flex-direction: column;
        flex-direction: column;
        border: none;
        width: 47px;
        background-color: #f5f5f5;
        height: 100%;
        .tabs__item {
          background-color: transparent;
          font-size: 13px;
          text-transform: uppercase;
          cursor: pointer;
          padding: 17px 30px;
          color: #4c4c4c;
          margin-bottom: -1px;
          &.selected {
            z-index: 1;
            border: none;
            background-color: #fff;
            -webkit-box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
            box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
          }
          &:hover {
            border: none;
            color: #000000;
            background-color: #fff;
            -webkit-box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
            box-shadow: 0px 4px 4px rgba(0, 0, 0, 0.05);
            z-index: 2;
            border-top: 1px solid #f5f5f5;
          }
          .step-name {
            font-weight: 500;
            font-size: 14px;
          }
        }
        .tabs__item {
          position: relative;
          display: -webkit-box;
          display: -ms-flexbox;
          display: flex;
          -webkit-box-align: center;
          -ms-flex-align: center;
          align-items: center;
          text-orientation: mixed;
          border: none;
          padding: 20px 17px;
          background-color: #f5f5f5;
          color: #000000;
          border-top: 1px solid transparent;
          -webkit-transition: background-color 150ms ease, color 150ms ease;
          transition: background-color 150ms ease, color 150ms ease;
        }
      }
      .tabs__wrap {
        display: -webkit-box;
        display: -ms-flexbox;
        display: flex;
        -webkit-box-pack: start;
        -ms-flex-pack: start;
        justify-content: flex-start;
        height: 56px;
      }
    }
  }
}
</style>