<template>
  <div class="create-project">
    <el-dialog :fullscreen="true"
               custom-class="create-project"
               :before-close="handleClose"
               :visible.sync="dialogVisible">
      <div class="project-contexts-modal">
        <header class="project-contexts-modal__header">
        </header>
        <div class="project-contexts-modal__content">
          <h1 class="project-contexts-modal__content-title">{{isEdit?'修改项目信息':'开始新建项目'}}</h1>
          <div class="project-contexts-modal__content-container">
            <div class="project-settings__inputs-container">
              <el-form :model="projectForm"
                       :rules="rules"
                       label-position="top"
                       ref="projectForm"
                       label-width="100px"
                       class="demo-projectForm">
                <el-form-item label="项目名称"
                              prop="project_name">
                  <el-input v-model="projectForm.project_name"></el-input>
                </el-form-item>

                <el-form-item label="项目主键"
                              prop="product_name">
                  <span slot="label">项目主键
                    <el-tooltip effect="dark"
                                content="项目主键是该项目资源的全局唯一标识符，用于该项目下所有资源的引用与更新，默认自动生成，同时支持手动指定，创建后不可更改"
                                placement="top">
                      <i class="el-icon-question"></i>
                    </el-tooltip>
                    <el-button v-if="!isEdit&&!editProductName"
                               @click="editProductName=true"
                               type="text">编辑</el-button>
                    <el-button v-if="!isEdit&&editProductName"
                               @click="editProductName=false"
                               type="text">完成</el-button>
                  </span>
                  <el-input :disabled="!showProductName"
                            v-model="projectForm.product_name"></el-input>
                </el-form-item>

                <el-form-item v-if="isEdit"
                              label="服务部署超时（分钟）"
                              prop="timeout">
                  <el-input v-model.number="projectForm.timeout"></el-input>
                </el-form-item>
                <el-form-item label="描述信息"
                              prop="desc">
                  <el-input type="textarea"
                            :rows="2"
                            placeholder="请输入描述信息"
                            v-model="projectForm.desc">
                  </el-input>

                </el-form-item>
                <el-form-item v-if="!isEdit"
                              prop="desc">
                  <el-row v-if="projectForm.product_feature.basic_facility==='kubernetes'"
                          :gutter="5">
                    <el-col :span="4">
                      <span>部署方式
                        <el-tooltip placement="top">
                          <div slot="content">
                            K8s YAML 部署：使用 K8s 原生的 YAML配置方式部署服务<br />
                          </div>
                          <i class="icon el-icon-question"></i>
                        </el-tooltip>
                      </span>
                    </el-col>
                    <el-col :span="12">
                      <el-radio-group size="mini"
                                      v-model="projectForm.product_feature.deploy_type">
                        <el-radio border
                                  label="k8s">K8s YAML 部署</el-radio>
                      </el-radio-group>
                    </el-col>
                  </el-row>
                </el-form-item>
                <el-row v-if="isEdit"
                        :gutter="5">
                  <el-col :span="24">
                    <el-form-item label="项目管理员"
                                  prop="user_ids">
                      <el-select v-model="projectForm.user_ids"
                                 style="width:100%"
                                 filterable
                                 multiple
                                 remote
                                 :remote-method="remoteMethod"
                                 :loading="loading"
                                 placeholder="请输入用户名搜索用户">
                        <el-option v-for="(user,index) in users"
                                   :key="index"
                                   :label="user.name"
                                   :value="user.id">
                        </el-option>
                      </el-select>
                    </el-form-item>
                  </el-col>
                </el-row>

              </el-form>
            </div>

          </div>
        </div>
        <footer class="project-contexts-modal__footer">
          <el-button class="create-btn"
                     type="primary"
                     plain
                     @click="submitForm('projectForm')">{{isEdit?'确认修改':'立即创建'}}
          </el-button>
        </footer>

      </div>
    </el-dialog>
  </div>
</template>
<script>
import { usersAPI, createProjectAPI, getSingleProjectAPI, updateSingleProjectAPI } from '@api';
let pinyin = require("pinyin");
import { mapGetters } from 'vuex';
let validateProductName = (rule, value, callback) => {
  if (typeof value === 'undefined' || value == '') {
    callback(new Error('填写项目主键'));
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('项目主键只支持小写字母和数字，特殊字符只支持中划线'));
    } else {
      callback();
    }
  }
};
let validateDeployTimeout = (rule, value, callback) => {
  const reg = /^[0-9]+.?[0-9]*/;
  if (!reg.test(value)) {
    callback(new Error('时间应为数字'));
  } else {
    if (value > 0) {
      callback();
    } else {
      callback(new Error('请输入正确的时间范围'));
    }
  }
};
export default {
  data() {
    return {
      dialogVisible: true,
      users: [],
      loading: false,
      editProductName: false,
      radio: true,
      projectForm: {
        project_name: '',
        product_name: '',
        user_ids: [],
        team_id: null,
        timeout: null,
        desc: '',
        visibility: 'public',
        enabled: true,
        product_feature: {
          basic_facility: "kubernetes",
          deploy_type: "k8s",
        }
      },
      rules: {
        project_name: [
          { required: true, message: '请输入项目名称', trigger: 'blur' },
        ],
        product_name: [
          { required: true, trigger: 'change', validator: validateProductName }
        ],
        user_ids: [
          { type: 'array', required: true, message: '请选择项目管理员', trigger: 'change' }
        ],
        visibility: [
          { type: 'string', required: true, message: '请选择项目可见范围', trigger: 'change' }
        ],
        enabled: [
          { type: 'boolean', required: true, message: '请选择项目是否启用项目', trigger: 'change' }
        ],
        timeout: [
          { required: true, trigger: 'change', validator: validateDeployTimeout }
        ],
      }
    };
  },
  methods: {
    getUsers() {
      const orgId = this.currentOrganizationId;
      usersAPI(orgId).then((res) => {
        this.users = this.$utils.deepSortOn(res.data, 'name');
      });
    },
    remoteMethod(query) {
      if (query !== '') {
        this.loading = true;
        const orgId = this.currentOrganizationId;
        usersAPI(orgId, '', 0, 0, query).then((res) => {
          this.loading = false;
          this.users = this.$utils.deepSortOn(res.data, 'name');
        });
      } else {
        this.users = [];
      }
    },
    handleClose() {
      if (this.isEdit) {
        this.$router.push(`/v1/projects/detail/${this.projectName}`);
      }
      else {
        this.$router.push(`/v1/projects`);
      }
    },
    createProject(payload) {
      createProjectAPI(payload).then((res) => {
        this.$message({
          type: 'success',
          message: '新建项目成功'
        });
        this.$store.dispatch('refreshProjectTemplates');
        if (payload.product_feature.basic_facility === 'kubernetes') {
          this.$router.push(`/v1/projects/create/${payload.product_name}/basic/info?rightbar=step`);
        }
      });
    },
    updateSingleProject(projectName, payload) {
      updateSingleProjectAPI(projectName, payload).then((res) => {
        this.$message({
          type: 'success',
          message: '更新项目成功'
        });
        this.$store.dispatch('refreshProjectTemplates');
        this.$router.push(`/v1/projects`)
      });
    },
    getProject(projectName) {
      getSingleProjectAPI(projectName).then((res) => {
        this.projectForm = res;
        if (res.team_id === 0) {
          this.projectForm.team_id = null;
        }
        if (!res.timeout) {
          this.$set(this.projectForm, 'timeout', 10)
        }
      });
    },
    submitForm(formName) {
      this.$refs[formName].validate((valid) => {
        if (valid) {
          if (this.isEdit) {
            this.updateSingleProject(this.projectForm.product_name, this.projectForm);
          }
          else {
            this.projectForm.timeout = 10;
            this.projectForm.user_ids.push(this.currentUserId);
            this.createProject(this.projectForm);
          }
        } else {
          return false;
        }
      });
    },
    resetForm(formName) {
      this.$refs[formName].resetFields();
    }
  },
  watch: {
    'projectForm.project_name': {
      handler(val, old_val) {
        if (!this.isEdit) {
          this.projectForm.product_name = pinyin(val, {
            style: pinyin.STYLE_NORMAL,
          }).join('')
        }
      }
    },
  },
  computed: {
    ...mapGetters([
      'signupStatus'
    ]),
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    currentUserId() {
      return this.$store.state.login.userinfo.info.id;
    },
    isEdit() {
      return this.$route.path.includes('/projects/edit');
    },
    showProductName() {
      return !this.isEdit && this.editProductName;
    },
    projectName() {
      if (this.isEdit) {
        return this.$route.params.project_name;
      }
      else {
        return false;
      }
    }
  },
  mounted() {
    this.$store.dispatch('getSignupStatus');
    if (this.isEdit) {
      this.getUsers();
      this.getProject(this.projectName);
    }
  }
};
</script>

<style lang="less" >
.create-project {
  .icon {
    cursor: pointer;
  }
  .el-dialog__headerbtn {
    font-size: 40px;
  }
  .el-dialog__body {
    padding: 5px 20px;
  }
  .create-btn {
    color: #1989fa;
    background: #fff;
    border-color: #1989fa;
    &:hover {
      color: #fff;
      background: #1989fa;
      border-color: #1989fa;
    }
  }
  .project-contexts-modal {
    height: 100%;
    .project-contexts-modal__header {
      display: flex;
      align-items: flex-end;
      justify-content: space-between;
      padding: 0 50px;
    }
    .project-contexts-modal__footer {
      display: flex;
      justify-content: center;
      align-items: center;
      height: 60px;
    }
    .project-contexts-modal__content {
      display: flex;
      flex-direction: column;
      flex: 1;
      justify-content: center;
      align-items: center;
      .project-contexts-modal__content-title {
        text-align: center;
        color: #000;
        font-weight: bold;
        font-size: 27px;
        margin: 0;
        margin-bottom: 20px;
      }
      .project-settings__inputs-container {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: flex-start;
        width: 800px;
        .el-form {
          width: 100%;
          .el-form-item {
            margin-bottom: 5px;
          }
        }
        .small-title {
          font-size: 12px;
          color: #ccc;
        }
        .el-radio--mini {
          &.is-bordered {
            width: 135px;
            margin-right: 0;
          }
        }
      }
    }
  }
}
</style>
