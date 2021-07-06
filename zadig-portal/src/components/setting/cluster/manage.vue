<template>
    <div v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconjiqun"
         class="setting-cluster-container">
      <!--Cluster-access-dialog-->
      <el-dialog :title='`接入集群 ${accessCluster.name}`'
                 :visible.sync="dialogClusterAccessVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="75%">
        <p>运行下面的 kubectl 命令，将其导入到 Zadig 系统中</p>
        <div class="highlighter-rouge">
          <div class="highlight">
            <span class="code-line">
              {{`$ kubectl apply -f "${$utils.getOrigin()}/api/aslan/cluster/agent/${accessCluster.id}/agent.yaml${deployType==='Deployment'?'?type=deploy':''}"`}}
              <span v-clipboard:copy="getYamlUrl()"
                    v-clipboard:success="copyCommandSuccess"
                    v-clipboard:error="copyCommandError"
                    class="el-icon-document-copy copy"></span>
            </span>
          </div>
        </div>
        <el-form>
          <el-form-item label="Agent 部署方式">
            <el-radio-group v-model="deployType">
              <el-radio label="DaemonSet">DaemonSet</el-radio>
              <el-radio label="Deployment">Deployment</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button :plain="true"
                     size="small"
                     type="primary"
                     @click="dialogClusterAccessVisible=false">确定</el-button>
        </div>
      </el-dialog>
      <!--Cluster-access-dialog-->

      <!--Cluster-create-dialog-->
      <el-dialog title='添加集群'
                 :visible.sync="dialogClusterCreateFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="45%">
        <el-alert title="注意:"
                  type="warning"
                  style="margin-bottom: 15px;"
                  :closable="false">
          <slot>
            <span class="tip-item">- 如果指定生产集群为“否”，有集成环境创建权限的用户，可以指定使用哪个集群资源。</span>
            <span class="tip-item">-
              如果指定生产集群为“是”，超级管理员可以通过权限控制集群资源的使用，以实现业务与资源的严格隔离和安全生产管控。</span>
            <span class="tip-item">- 接入新的集群后，如需该集群支持泛域名访问，需安装 Ingress Controller，请参阅
              <el-link style="font-size: 13px;"
                       type="primary"
                       :href="`/zadig/pages/c325d8/`"
                       :underline="false"
                       target="_blank">帮助</el-link> 查看 Agent 部署样例</span>
          </slot>
        </el-alert>
        <el-form ref="cluster"
                 :rules="rules"
                 label-width="120px"
                 label-position="top"
                 :model="cluster">
          <el-form-item label="名称"
                        prop="name">
            <el-input size="small"
                      v-model="cluster.name"
                      placeholder="请输入集群名称"></el-input>
          </el-form-item>
          <el-form-item label="命名空间"
                        prop="namespace">
            <el-input size="small"
                      v-model="cluster.namespace"
                      placeholder="请输入命名空间"></el-input>
          </el-form-item>
          <el-form-item label="描述"
                        prop="description">
            <el-input size="small"
                      v-model="cluster.description"
                      placeholder="请输入描述"></el-input>
          </el-form-item>
          <el-form-item label="生产集群"
                        prop="production">
            <el-radio-group v-model="cluster.production">
              <el-radio :label="true">是</el-radio>
              <el-radio :label="false">否</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogClusterCreateFormVisible = false">取 消</el-button>
          <el-button :plain="true"
                     size="small"
                     type="success"
                     @click="clusterOperation('add')">保存</el-button>
        </div>
      </el-dialog>
      <!--Cluster-create-dialog-->

      <!--Cluster-edit-dialog-->
      <el-dialog title='修改'
                 :visible.sync="dialogClusterEditFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="35%">
        <el-form ref="swapCluster"
                 :rules="rules"
                 label-width="120px"
                 label-position="top"
                 :model="swapCluster">
          <el-form-item label="名称"
                        prop="name">
            <el-input size="small"
                      v-model="swapCluster.name"
                      placeholder="请输入集群名称"></el-input>
          </el-form-item>
          <el-form-item label="命名空间"
                        prop="namespace">
            <el-input size="small"
                      v-model="swapCluster.namespace"
                      placeholder="请输入命名空间"></el-input>
          </el-form-item>
          <el-form-item label="描述"
                        prop="description">
            <el-input size="small"
                      v-model="swapCluster.description"
                      placeholder="请输入描述"></el-input>
          </el-form-item>
          <el-form-item label="生产集群"
                        prop="production">
            <el-radio-group v-model="swapCluster.production">
              <el-radio :label="true">是</el-radio>
              <el-radio :label="false">否</el-radio>
            </el-radio-group>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogClusterEditFormVisible = false">取 消</el-button>
          <el-button size="small"
                     :plain="true"
                     type="success"
                     @click="clusterOperation('update')">保存</el-button>
        </div>
      </el-dialog>
      <!--Cluster-edit-dialog-->
      <div class="section">
        <div class="sync-container">
          <el-button size="small"
                     :plain="true"
                     @click="dialogClusterCreateFormVisible=true"
                     type="success">新建</el-button>
        </div>
        <div class="Cluster-list">
          <template>
            <el-table :data="allCluster"
                      style="width: 100%;">
              <el-table-column label="名称">
                <template slot-scope="scope">
                  <span>{{scope.row.name}}</span>
                </template>
              </el-table-column>
              <el-table-column width="120"
                               label="状态">
                <template slot-scope="scope">
                  <el-tag size="small"
                          effect="dark"
                          :type="statusIndicator[scope.row.status]">
                    {{myTranslate(scope.row.status)}}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="生产集群">
                <template slot-scope="scope">
                  <span>{{scope.row.production?'是':'否'}}</span>
                </template>
              </el-table-column>
              <el-table-column label="描述">
                <template slot-scope="scope">
                  <span>{{scope.row.description}}</span>
                </template>
              </el-table-column>
              <el-table-column label="创建时间">
                <template slot-scope="scope">
                  <span>{{$utils.convertTimestamp(scope.row.createdAt)}}</span>
                </template>
              </el-table-column>
              <el-table-column label="创建人">
                <template slot-scope="scope">
                  <span>{{scope.row.createdBy}}</span>
                </template>
              </el-table-column>

              <el-table-column width="240"
                               label="操作">
                <template slot-scope="scope">
                  <el-button v-if="scope.row.status==='pending'||scope.row.status==='abnormal'"
                             @click="clusterOperation('access',scope.row)"
                             size="mini">接入</el-button>
                  <el-button v-if="scope.row.status==='normal'"
                             @click="clusterOperation('disconnect',scope.row)"
                             size="mini">断开</el-button>
                  <el-button v-if="scope.row.status==='disconnected'"
                             @click="clusterOperation('recover',scope.row)"
                             size="mini">恢复</el-button>
                  <el-button @click="clusterOperation('edit',scope.row)"
                             size="mini">编辑</el-button>
                  <el-button @click="clusterOperation('delete',scope.row)"
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
import { getClusterListAPI, createClusterAPI, updateClusterAPI, deleteClusterAPI, recoverClusterAPI, disconnectClusterAPI } from '@api'
import { wordTranslate } from '@utils/word_translate'
import bus from '@utils/event_bus'
const validateClusterName = (rule, value, callback) => {
  if (value === '') {
    callback(new Error('请输入集群名称'))
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('名称只支持小写字母和数字，特殊字符只支持中划线'))
    } else {
      callback()
    }
  }
}
export default {
  data () {
    return {
      statusIndicator: {
        normal: 'success',
        abnormal: 'danger',
        pending: 'info',
        disconnected: 'warning'
      },
      allCluster: [],
      deployType: 'DaemonSet',
      cluster: {
        name: '',
        production: false,
        description: '',
        namespace: ''
      },
      swapCluster: {
        name: '',
        production: false,
        description: '',
        namespace: ''
      },
      accessCluster: {
        yaml: 'init',
        name: 'test'
      },
      dialogClusterCreateFormVisible: false,
      dialogClusterEditFormVisible: false,
      dialogClusterAccessVisible: false,
      loading: false,
      rules: {
        name: [{
          type: 'string',
          required: true,
          validator: validateClusterName,
          trigger: 'change'
        }],
        production: [{
          type: 'boolean',
          required: true,
          message: '请选择是否为生产集群'
        }]
      }
    }
  },
  methods: {
    getYamlUrl () {
      return `kubectl apply -f "${this.$utils.getOrigin()}/api/aslan/cluster/agent/${this.accessCluster.id}/agent.yaml${this.deployType === 'Deployment' ? '?type=deploy' : ''}"`
    },
    clusterOperation (operate, current_cluster) {
      if (operate === 'add') {
        this.$refs.cluster.validate(valid => {
          if (valid) {
            const payload = this.cluster
            this.dialogClusterCreateFormVisible = false
            this.addCluster(payload)
          } else {
            return false
          }
        })
      } else if (operate === 'access') {
        this.accessCluster = this.$utils.cloneObj(current_cluster)
        this.dialogClusterAccessVisible = true
      } else if (operate === 'disconnect') {
        this.$confirm(`确定要断开 ${current_cluster.name} 的连接?`, '确认', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(({ value }) => {
          this.disconnectCluster(current_cluster.id)
        })
      } else if (operate === 'recover') {
        this.recoverCluster(current_cluster.id)
      } else if (operate === 'edit') {
        this.swapCluster = this.$utils.cloneObj(current_cluster)
        this.dialogClusterEditFormVisible = true
      } else if (operate === 'update') {
        this.$refs.swapCluster.validate(valid => {
          if (valid) {
            const id = this.swapCluster.id
            const payload = this.swapCluster
            this.dialogClusterEditFormVisible = false
            this.updateCluster(id, payload)
          } else {
            return false
          }
        })
      } else if (operate === 'delete') {
        const id = current_cluster.id
        this.$confirm(`确定要删除 ${current_cluster.name} ?`, '确认', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(({ value }) => {
          deleteClusterAPI(id).then((res) => {
            this.getCluster()
            this.$message({
              message: '删除集群成功',
              type: 'success'
            })
          })
        })
      }
    },
    addCluster (payload) {
      createClusterAPI(payload).then((res) => {
        this.$refs.cluster.resetFields()
        this.getCluster()
        this.accessCluster = res
        this.dialogClusterAccessVisible = true
        this.$message({
          type: 'success',
          message: '新增成功'
        })
      })
    },
    disconnectCluster (id) {
      disconnectClusterAPI(id).then((res) => {
        this.getCluster()
        this.$message({
          type: 'success',
          message: '断开集群连接成功'
        })
      })
    },
    recoverCluster (id) {
      recoverClusterAPI(id).then((res) => {
        this.getCluster()
        this.$message({
          type: 'success',
          message: '恢复集群连接成功'
        })
      })
    },
    updateCluster (id, payload) {
      updateClusterAPI(id, payload).then((res) => {
        this.$refs.swapCluster.resetFields()
        this.getCluster()
        this.$message({
          type: 'success',
          message: '更新成功'
        })
      })
    },
    getCluster () {
      this.loading = true
      getClusterListAPI().then((res) => {
        this.loading = false
        this.allCluster = res
      })
    },
    copyCommandSuccess (event) {
      this.$message({
        message: '命令已成功复制到剪贴板',
        type: 'success'
      })
    },
    copyCommandError (event) {
      this.$message({
        message: '命令复制失败',
        type: 'error'
      })
    },
    myTranslate (word) {
      return wordTranslate(word, 'setting', 'cluster')
    }
  },
  created () {
    this.getCluster()
    bus.$emit(`set-topbar-title`, { title: '集群管理', breadcrumb: [] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
  }
}
</script>

<style lang="less">
.setting-cluster-container {
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

    .Cluster-list {
      padding-bottom: 30px;
    }
  }

  .dialog-style {
    .el-dialog__body {
      padding: 0 20px;
    }

    .el-form-item {
      margin-bottom: 10px;
    }

    .tip-item {
      display: block;
      font-size: 14px;
      line-height: 17px;

      &.code {
        display: block;
        margin: 2px 0;
        padding: 0 2px;
        color: #ecf0f1;
        font-size: 13px;
        font-family: monospace;
        word-wrap: break-word;
        word-break: break-all;
        background-color: #3d3d3d;
        background-color: #334851;
        border: 1px solid #0a141a;
        border-radius: 4px;
      }
    }

    .highlighter-rouge {
      .code-line {
        display: block;
        margin: 0;
        padding: 8px 0;
        color: #ecf0f1;
        font-size: 14px;
        font-family: monospace;
        word-wrap: break-word;
        word-break: break-all;
        background-color: #3d3d3d;
        background-color: #334851;
        border: 1px solid #0a141a;
        border-radius: 4px;
      }

      .copy {
        font-size: 16px;
        cursor: pointer;

        &:hover {
          color: #13ce66;
        }
      }
    }

    .command {
      color: #fff;
      font-size: 13px;
      line-height: 20px;
      background: #303133;
    }
  }
}
</style>
