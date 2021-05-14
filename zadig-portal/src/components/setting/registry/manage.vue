<template>
    <div v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading icondocker"
         class="setting-registry-container">
      <!--registry-create-dialog-->
      <el-dialog title='添加'
                 :visible.sync="dialogRegistryCreateFormVisible"
                 :close-on-click-modal="false"
                 custom-class="dialog-style"
                 width="35%">
        <el-form ref="registry"
                 :rules="rules"
                 label-width="120px"
                 tab-position="left"
                 :model="registry">
          <el-form-item label="默认使用"
                        prop="is_default">
            <el-checkbox v-model="registry.is_default"></el-checkbox>
          </el-form-item>
          <el-form-item label="地址"
                        prop="reg_addr">
            <el-input size="small"
                      v-model="registry.reg_addr"></el-input>
          </el-form-item>
          <el-form-item label="Namespace"
                        prop="namespace">
            <el-input size="small"
                      v-model="registry.namespace"></el-input>
          </el-form-item>
          <el-form-item :rules="{required: false}"
                        label="Docker 用户名"
                        prop="access_key">
            <el-input size="small"
                      v-model="registry.access_key"></el-input>
          </el-form-item>
          <el-form-item :rules="{required: false}"
                        label="Docker 密码"
                        prop="secret_key">
            <el-input size="small"
                      type="passsword"
                      v-model="registry.secret_key"></el-input>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button @click="dialogRegistryCreateFormVisible = false">取 消</el-button>
          <el-button :plain="true"
                     type="success"
                     @click="registryOperation('add')">保存</el-button>
        </div>
      </el-dialog>
      <!--registry-create-dialog-->

      <!--registry-edit-dialog-->
      <el-dialog title='修改'
                 :visible.sync="dialogRegistryEditFormVisible"
                 :close-on-click-modal="false"
                 custom-class="dialog-style"
                 width="35%">
        <el-form ref="swapRegistry"
                 :rules="rules"
                 label-width="120px"
                 tab-position="left"
                 :model="swapRegistry">
          <el-form-item label="默认使用"
                        prop="is_default">
            <el-checkbox v-model="swapRegistry.is_default"></el-checkbox>
          </el-form-item>
          <el-form-item label="地址"
                        prop="reg_addr">
            <el-input size="small"
                      v-model="swapRegistry.reg_addr"></el-input>
          </el-form-item>
          <el-form-item label="Namespace"
                        prop="namespace">
            <el-input size="small"
                      v-model="swapRegistry.namespace"></el-input>
          </el-form-item>
          <el-form-item :rules="{required: false}"
                        label="Docker 用户名"
                        prop="access_key">
            <el-input size="small"
                      v-model="swapRegistry.access_key"></el-input>
          </el-form-item>
          <el-form-item :rules="{required: false}"
                        label="Docker 密码"
                        prop="secret_key">
            <el-input size="small"
                      v-model="swapRegistry.secret_key"></el-input>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button @click="dialogRegistryEditFormVisible = false">取 消</el-button>
          <el-button :plain="true"
                     type="success"
                     @click="registryOperation('update')">保存</el-button>
        </div>
      </el-dialog>
      <!--registry-edit-dialog-->
      <div class="section">
        <div class="sync-container">
          <el-button :plain="true"
                     @click="dialogRegistryCreateFormVisible=true"
                     size="small"
                     type="success">新建</el-button>
        </div>
        <div class="registry-list">
          <template>
            <el-table :data="allRegistry"
                      style="width: 100%">
              <el-table-column label="地址/Namespace">
                <template slot-scope="scope">
                  <span>{{`${scope.row.reg_addr.split('://')[1]}/${scope.row.namespace}`}}</span>
                </template>
              </el-table-column>
              <el-table-column width="150px"
                               label="默认使用">
                <template slot-scope="scope">
                  <el-tag v-if="scope.row.is_default">默认使用</el-tag>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column width="220px"
                               label="创建时间">
                <template slot-scope="scope">
                  <i class="el-icon-time"></i>
                  <span
                        style="margin-left: 5px">{{ $utils.convertTimestamp(scope.row.update_time) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="最后修改">
                <template slot-scope="scope">
                  <span>{{ scope.row.update_by}}</span>
                </template>
              </el-table-column>

              <el-table-column label="操作"
                               width="180px">
                <template slot-scope="scope">
                  <el-button @click="registryOperation('edit',scope.row)"
                             size="mini">编辑</el-button>
                  <el-button @click="registryOperation('delete',scope.row)"
                             size="mini"
                             type="danger">删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </template>
        </div>
      </div>

    </div>
</template>

<script>

import { getRegistryListAPI, createRegistryAPI, updateRegistryAPI, deleteRegistryAPI } from '@api';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
      allRegistry: [],
      registry: {
        namespace: '',
        reg_addr: '',
        access_key: '',
        secret_key: '',
        reg_provider: 'native',
        is_default: false,
      },
      swapRegistry: {
        namespace: '',
        reg_addr: '',
        reg_provider: 'native',
        access_key: '',
        secret_key: '',
        is_default: false,
      },
      dialogRegistryCreateFormVisible: false,
      dialogRegistryEditFormVisible: false,
      loading: true,
      rules: {
        reg_addr: [{
          required: true,
          message: '请输入 URL',
          trigger: 'blur'
        },
        {
          type: 'url',
          message: '请输入正确的 URL，包含协议',
          trigger: ['blur', 'change']
        }],
        namespace: [{ required: true, message: '请输入 Namespace', trigger: 'blur' }],
        access_key: [{ required: true, message: '请输入 Access Key', trigger: 'blur' }],
        secret_key: [{ required: true, message: '请输入 Secret Key', trigger: 'blur' }],
      }
    };
  },
  methods: {
    registryOperation(operate, current_registry) {
      if (operate === 'add') {
        this.$refs['registry'].validate(valid => {
          if (valid) {
            let payload = this.registry;
            payload.org_id = this.currentOrganizationId;
            this.dialogRegistryCreateFormVisible = false;
            this.addRegistry(payload);
          } else {
            return false;
          }
        });
      } else if (operate === 'edit') {
        this.swapRegistry = this.$utils.cloneObj(current_registry);
        this.dialogRegistryEditFormVisible = true;
      } else if (operate === 'update') {
        this.$refs['swapRegistry'].validate(valid => {
          if (valid) {
            const id = this.swapRegistry.id;
            const payload = this.swapRegistry;
            this.dialogRegistryEditFormVisible = false;
            this.updateRegistry(id, payload);
          } else {
            return false;
          }
        });
      } else if (operate === 'delete') {
        const id = current_registry.id;
        this.$confirm(`确定要删除 ${current_registry.namespace} ?`, '确认', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(({ value }) => {
          deleteRegistryAPI(id).then((res) => {
            this.getRegistry();
            this.$message({
              message: '删除成功',
              type: 'success'
            })
          })
        })
      }
    },
    addRegistry(payload) {
      createRegistryAPI(payload).then((res) => {
        this.$refs['registry'].resetFields();
        this.getRegistry();
        this.$message({
          type: 'success',
          message: '新增成功'
        });
      })
    },
    updateRegistry(id, payload) {
      updateRegistryAPI(id, payload).then((res) => {
        this.$refs['swapRegistry'].resetFields();
        this.getRegistry();
        this.$message({
          type: 'success',
          message: '更新成功'
        });
      })
    },
    getRegistry() {
      this.loading = true;
      const organizationId = this.currentOrganizationId;
      getRegistryListAPI(organizationId).then((res) => {
        this.loading = false;
        this.allRegistry = res;
      })
    }
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    }
  },
  created() {
    bus.$emit(`set-topbar-title`, { title: 'REGISTRY 管理', breadcrumb: [] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
    this.getRegistry();
  }
};
</script>


<style lang="less">
.setting-registry-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 30px;
  font-size: 13px;
  .module-title h1 {
    font-weight: 200;
    font-size: 2rem;
    margin-bottom: 1.5rem;
  }
  .section {
    margin-bottom: 56px;
    .sync-container {
      overflow: hidden;
      padding-top: 15px;
      padding-bottom: 15px;
      .el-button--success.is-plain {
        background: #fff;
        border-color: #13ce66;
        color: #13ce66;
      }
      .el-button--success.is-plain:hover {
        background: #fff;
        border-color: #13ce66;
        color: #13ce66;
      }
    }
    .registry-list {
      padding-bottom: 30px;
    }
    .dialog-style {
      .el-dialog__body {
        padding: 0px 20px;
      }
    }
  }
}
</style>
