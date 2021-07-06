<template>
    <div v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconzhuji"
         class="setting-host-container">

      <!--Host-create-dialog-->
      <el-dialog title='创建主机资源'
                 :visible.sync="dialogHostCreateFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="45%">
        <el-form ref="host"
                 :rules="rules"
                 label-width="120px"
                 label-position="left"
                 :model="host">
          <el-form-item label="主机名称"
                        prop="name">
            <el-input size="small"
                      v-model="host.name"
                      placeholder="请输入主机名称"></el-input>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="user_name">
            <el-input size="small"
                      v-model="host.user_name"
                      placeholder="请输入用户名"></el-input>
          </el-form-item>
          <el-form-item label="IP 地址"
                        prop="ip">
            <el-input size="small"
                      v-model="host.ip"
                      placeholder="请输入主机 IP"></el-input>
          </el-form-item>
          <el-form-item label="标签"
                        prop="label">
            <el-input size="small"
                      v-model="host.label"
                      placeholder="请输入标签"></el-input>
          </el-form-item>
          <el-form-item label="是否生产"
                        prop="is_prod">
            <el-radio-group v-model="host.is_prod">
              <el-radio :label="true">是</el-radio>
              <el-radio :label="false">否</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="私钥"
                        prop="private_key">
            <el-input size="small"
                      type="textarea"
                      v-model="host.private_key"
                      placeholder="请输入私钥"></el-input>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogHostCreateFormVisible = false">取 消</el-button>
          <el-button :plain="true"
                     size="small"
                     type="success"
                     @click="hostOperation('add')">保存</el-button>
        </div>
      </el-dialog>
      <!--Host-create-dialog-->

      <!--Host-edit-dialog-->
      <el-dialog title='修改主机资源'
                 :visible.sync="dialogHostEditFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="35%">
        <el-form ref="swapHost"
                 :rules="rules"
                 label-width="120px"
                 label-position="left"
                 :model="swapHost">
          <el-form-item label="主机名称"
                        prop="name">
            <el-input size="small"
                      v-model="swapHost.name"
                      placeholder="请输入主机名称"></el-input>
          </el-form-item>
          <el-form-item label="用户名"
                        prop="user_name">
            <el-input size="small"
                      v-model="swapHost.user_name"
                      placeholder="请输入用户名"></el-input>
          </el-form-item>
          <el-form-item label="IP"
                        prop="ip">
            <el-input size="small"
                      v-model="swapHost.ip"
                      placeholder="请输入主机 IP"></el-input>
          </el-form-item>

          <el-form-item label="标签"
                        prop="label">
            <el-input size="small"
                      v-model="swapHost.label"
                      placeholder="请输入主机标签"></el-input>
          </el-form-item>
          <el-form-item label="是否生产"
                        prop="is_prod">
            <el-radio-group v-model="swapHost.is_prod">
              <el-radio :label="true">是</el-radio>
              <el-radio :label="false">否</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="私钥"
                        prop="private_key">
            <el-input size="small"
                      type="textarea"
                      v-model="swapHost.origin_private_key"
                      placeholder="请输入私钥"></el-input>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogHostEditFormVisible = false">取 消</el-button>
          <el-button size="small"
                     :plain="true"
                     type="success"
                     @click="hostOperation('update')">保存</el-button>
        </div>
      </el-dialog>
      <!--Host-edit-dialog-->
      <div class="section">
        <div class="sync-container">
          <el-button size="small"
                     :plain="true"
                     @click=" dialogHostCreateFormVisible=true"
                     type="success">新建</el-button>
        </div>
        <div class="cluster-list">
          <template>
            <el-table :data="allHost"
                      style="width: 100%;">
              <el-table-column label="主机名称">
                <template slot-scope="scope">
                  <span>{{scope.row.name}}</span>
                </template>
              </el-table-column>
              <el-table-column label="标签">
                <template slot-scope="scope">
                  <el-tag v-if="scope.row.label"
                          size="small">{{scope.row.label}}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="IP">
                <template slot-scope="scope">
                  <span>{{scope.row.ip}}</span>
                </template>
              </el-table-column>
              <el-table-column label="用户名">
                <template slot-scope="scope">
                  <span>{{scope.row.user_name}}</span>
                </template>
              </el-table-column>
              <el-table-column width="240"
                               label="操作">
                <template slot-scope="scope">
                  <el-button @click="hostOperation('edit',scope.row)"
                             size="mini">编辑</el-button>
                  <el-button @click="hostOperation('delete',scope.row)"
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
import { getHostListAPI, createHostAPI, updateHostAPI, deleteHostAPI } from '@api'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
      allHost: [],
      host: {
        name: '',
        label: '',
        ip: '',
        user_name: '',
        private_key: ''
      },
      swapHost: {
        name: '',
        label: '',
        ip: '',
        user_name: '',
        private_key: '',
        origin_private_key: ''
      },
      dialogHostCreateFormVisible: false,
      dialogHostEditFormVisible: false,
      loading: false,
      rules: {
        name: [{
          type: 'string',
          required: true,
          message: '请输入主机名称',
          trigger: 'change'
        }],
        label: [{
          type: 'string',
          required: false,
          message: '请输入主机标签',
          trigger: 'change'
        }],
        user_name: [{
          type: 'string',
          required: true,
          message: '请输入用户名'
        }],
        ip: [{
          type: 'string',
          required: true,
          message: '请输入主机 IP'
        }],
        private_key: [{
          type: 'string',
          required: true,
          message: '请输入私钥'
        }]
      }
    }
  },
  methods: {
    hostOperation (operate, current_host) {
      if (operate === 'add') {
        this.$refs.host.validate(valid => {
          if (valid) {
            const payload = this.host
            payload.private_key = window.btoa(payload.private_key)
            this.dialogHostCreateFormVisible = false
            this.addHost(payload)
          } else {
            return false
          }
        })
      } else if (operate === 'edit') {
        this.swapHost = this.$utils.cloneObj(current_host)
        this.dialogHostEditFormVisible = true
      } else if (operate === 'update') {
        this.$refs.swapHost.validate(valid => {
          if (valid) {
            const id = this.swapHost.id
            const payload = this.swapHost
            payload.private_key = window.btoa(payload.origin_private_key)
            delete payload.origin_private_key
            this.dialogHostEditFormVisible = false
            this.updateHost(id, payload)
          } else {
            return false
          }
        })
      } else if (operate === 'delete') {
        const id = current_host.id
        this.$confirm(`确定要删除 ${current_host.label} ?`, '确认', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(({ value }) => {
          deleteHostAPI(id).then((res) => {
            this.getHost()
            this.$message({
              message: '删除主机成功',
              type: 'success'
            })
          })
        })
      }
    },
    addHost (payload) {
      createHostAPI(payload).then((res) => {
        this.$refs.host.resetFields()
        this.getHost()
        this.accessCluster = res
        this.dialogClusterAccessVisible = true
        this.$message({
          type: 'success',
          message: '新增主机信息成功'
        })
      })
    },
    updateHost (id, payload) {
      updateHostAPI(id, payload).then((res) => {
        this.$refs.swapHost.resetFields()
        this.getHost()
        this.$message({
          type: 'success',
          message: '更新主机信息成功'
        })
      })
    },
    getHost () {
      this.loading = true
      getHostListAPI().then((res) => {
        this.loading = false
        res.forEach(element => {
          element.origin_private_key = window.atob(element.private_key)
        })
        this.allHost = res
      })
    }
  },
  computed: {
  },
  created () {
    this.getHost()
    bus.$emit(`set-topbar-title`, { title: '主机管理', breadcrumb: [] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
  }
}
</script>

<style lang="less">
.setting-host-container {
  position: relative;
  flex: 1;
  padding: 15px 30px;
  overflow: auto;
  font-size: 13px;

  .module-title h1 {
    margin-bottom: 1.5rem;
    font-weight: 200;
    font-size: 2rem;
  }

  .section {
    margin-bottom: 56px;

    .sync-container {
      padding-top: 15px;
      padding-bottom: 15px;
      overflow: hidden;

      .el-button--success.is-plain {
        color: #13ce66;
        background: #fff;
        border-color: #13ce66;
      }

      .el-button--success.is-plain:hover {
        color: #13ce66;
        background: #fff;
        border-color: #13ce66;
      }
    }

    .cluster-list {
      padding-bottom: 30px;
    }
  }

  .dialog-style {
    .el-dialog__body {
      padding: 0 20px;
    }

    .el-form-item {
      margin-bottom: 15px;
    }
  }
}
</style>
