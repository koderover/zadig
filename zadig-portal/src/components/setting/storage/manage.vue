<template>
    <div v-loading="loading"
         element-loading-text="加载中..."
         element-loading-spinner="iconfont iconfont-loading iconduixiangcunchu"
         class="setting-storage-container">
      <!--storage-create-dialog-->
      <el-dialog title='添加'
                 :visible.sync="dialogStorageCreateFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="35%">
        <el-form ref="storage"
                 :rules="rules"
                 label-width="120px"
                 tab-position="left"
                 :model="storage">
          <el-form-item label="接入点地址"
                        prop="endpoint">
            <el-input size="small"
                      v-model="storage.endpoint"
                      placeholder="请输入接入点地址"></el-input>
          </el-form-item>
          <el-form-item label="AK"
                        prop="ak">
            <el-input size="small"
                      v-model="storage.ak"
                      placeholder="请输入 Access Key"></el-input>
          </el-form-item>
          <el-form-item label="SK"
                        prop="sk">
            <el-input size="small"
                      v-model="storage.sk"
                      placeholder="请输入 Secret Key"></el-input>
          </el-form-item>
          <el-form-item label="Bucket"
                        prop="bucket">
            <el-input size="small"
                      v-model="storage.bucket"
                      placeholder="请输入 Bucket"></el-input>
          </el-form-item>
          <el-form-item label="存储相对路径"
                        prop="subfolder">
            <el-input size="small"
                      v-model="storage.subfolder"
                      placeholder="请输入存储相对路径"></el-input>
          </el-form-item>
          <el-form-item label="协议"
                        prop="insecure">
            <el-radio v-model="storage.insecure"
                      :label="true">HTTP</el-radio>
            <el-radio v-model="storage.insecure"
                      :label="false">HTTPS</el-radio>
          </el-form-item>
          <el-form-item label="默认使用"
                        prop="is_default">
            <el-checkbox size="small"
                         v-model="storage.is_default"></el-checkbox>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogStorageCreateFormVisible = false">取 消</el-button>
          <el-button size="small"
                     :plain="true"
                     type="success"
                     @click="storageOperation('add')">保存</el-button>
        </div>
      </el-dialog>
      <!--storage-create-dialog-->

      <!--storage-edit-dialog-->
      <el-dialog title='修改'
                 :visible.sync="dialogStorageEditFormVisible"
                 custom-class="dialog-style"
                 :close-on-click-modal="false"
                 width="35%">
        <el-form ref="swapStorage"
                 :rules="rules"
                 label-width="120px"
                 tab-position="left"
                 :model="swapStorage">
          <el-form-item label="接入点地址"
                        prop="endpoint">
            <el-input size="small"
                      v-model="swapStorage.endpoint"
                      placeholder="请输入接入点地址"></el-input>
          </el-form-item>
          <el-form-item label="AK"
                        prop="ak">
            <el-input size="small"
                      v-model="swapStorage.ak"
                      placeholder="请输入 Access Key"></el-input>
          </el-form-item>
          <el-form-item label="SK"
                        prop="sk">
            <el-input size="small"
                      v-model="swapStorage.sk"
                      placeholder="请输入 Secret Key"></el-input>
          </el-form-item>
          <el-form-item label="Bucket"
                        prop="bucket">
            <el-input size="small"
                      v-model="swapStorage.bucket"
                      placeholder="请输入 Bucket"></el-input>
          </el-form-item>
          <el-form-item label="存储相对路径"
                        prop="subfolder">
            <el-input size="small"
                      v-model="swapStorage.subfolder"
                      placeholder="请输入存储相对路径"></el-input>
          </el-form-item>
          <el-form-item label="协议"
                        prop="insecure">
            <el-radio v-model="swapStorage.insecure"
                      :label="true">HTTP</el-radio>
            <el-radio v-model="swapStorage.insecure"
                      :label="false">HTTPS</el-radio>
          </el-form-item>
          <el-form-item label="默认使用"
                        prop="is_default">
            <el-checkbox size="small"
                         v-model="swapStorage.is_default"></el-checkbox>
          </el-form-item>
        </el-form>
        <div slot="footer"
             class="dialog-footer">
          <el-button size="small"
                     @click="dialogStorageEditFormVisible = false">取 消</el-button>
          <el-button size="small"
                     :plain="true"
                     type="success"
                     @click="storageOperation('update')">保存</el-button>
        </div>
      </el-dialog>
      <!--storage-edit-dialog-->
      <div class="section">
        <div class="sync-container">
          <el-button :plain="true"
                     size="small"
                     @click="dialogStorageCreateFormVisible=true"
                     type="success">新建</el-button>
        </div>
        <div class="storage-list">
          <template>
            <el-table :data="allStorage"
                      style="width: 100%">
              <el-table-column label="接入点地址">
                <template slot-scope="scope">
                  <span>{{scope.row.endpoint}}</span>
                </template>
              </el-table-column>
              <el-table-column label="Bucket">
                <template slot-scope="scope">
                  <span>{{scope.row.bucket}}</span>
                </template>
              </el-table-column>
              <el-table-column label="相对路径">
                <template slot-scope="scope">
                  <span v-if="scope.row.subfolder">{{scope.row.subfolder}}</span>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column width="80"
                               label="HTTPS">
                <template slot-scope="scope">
                  <span>{{!scope.row.insecure?'是':'否'}}</span>
                </template>
              </el-table-column>

              <el-table-column width="100" label="默认使用">
                <template slot-scope="scope">
                  <el-tag v-if="scope.row.is_default">默认使用</el-tag>
                  <span v-else>-</span>
                </template>
              </el-table-column>
              <el-table-column label="创建时间">
                <template slot-scope="scope">
                  <i class="el-icon-time"></i>
                  <span>{{ $utils.convertTimestamp(scope.row.update_time) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="最后修改">
                <template slot-scope="scope">
                  <span>{{ scope.row.updated_by}}</span>
                </template>
              </el-table-column>

              <el-table-column label="操作">
                <template slot-scope="scope">
                  <el-button @click="storageOperation('edit',scope.row)"
                             size="mini">编辑</el-button>
                  <el-button @click="storageOperation('delete',scope.row)"
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

import { getStorageListAPI, createStorageAPI, updateStorageAPI, deleteStorageAPI } from '@api';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
      allStorage: [],
      storage: {
        "ak": "",
        "sk": "",
        "endpoint": "",
        "bucket": "",
        "subfolder": "",
        "insecure": true,
        "is_default": true
      },
      swapStorage: {
        "ak": "",
        "sk": "",
        "endpoint": "",
        "bucket": "",
        "subfolder": "",
        "insecure": true,
        "is_default": true
      },
      dialogStorageCreateFormVisible: false,
      dialogStorageEditFormVisible: false,
      loading: true,
      rules: {
        ak: [{ required: true, message: '请输入 Access Key', trigger: 'blur' }],
        sk: [{ required: true, message: '请输入 Secret Key', trigger: 'blur' }],
        endpoint: [{
          required: true,
          message: '请输入接入点地址',
          trigger: 'blur'
        }],
        bucket: [{ required: true, message: '请输入 Bucket', trigger: 'blur' }],
      }
    };
  },
  methods: {
    storageOperation(operate, current_storage) {
      if (operate === 'add') {
        this.$refs['storage'].validate(valid => {
          if (valid) {
            let payload = this.storage;
            this.dialogStorageCreateFormVisible = false;
            this.addStorage(payload);
          } else {
            return false;
          }
        });
      } else if (operate === 'edit') {
        this.swapStorage = this.$utils.cloneObj(current_storage);
        this.dialogStorageEditFormVisible = true;
      } else if (operate === 'update') {
        this.$refs['swapStorage'].validate(valid => {
          if (valid) {
            const id = this.swapStorage.id;
            const payload = this.swapStorage;
            this.dialogStorageEditFormVisible = false;
            this.updateStorage(id, payload);
          } else {
            return false;
          }
        });
      } else if (operate === 'delete') {
        const id = current_storage.id;
        this.$confirm(`确定要删除 ${current_storage.endpoint} ?`, '确认', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(({ value }) => {
          deleteStorageAPI(id).then((res) => {
            this.getStorage();
            this.$message({
              message: '删除成功',
              type: 'success'
            })
          })
        })
      }
    },
    addStorage(payload) {
      createStorageAPI(payload).then((res) => {
        this.$refs['storage'].resetFields();
        this.getStorage();
        this.storage = {
          "ak": "",
          "sk": "",
          "endpoint": "",
          "bucket": "",
          "subfolder": "",
          "insecure": true,
          "is_default": true
        }
        this.$message({
          type: 'success',
          message: '新增成功'
        });
      })
    },
    updateStorage(id, payload) {
      updateStorageAPI(id, payload).then((res) => {
        this.$refs['swapStorage'].resetFields();
        this.getStorage();
        this.$message({
          type: 'success',
          message: '更新成功'
        });
      })
    },
    getStorage() {
      this.loading = true;
      getStorageListAPI().then((res) => {
        this.loading = false;
        this.allStorage = res;
      })
    }
  },
  computed: {

  },
  created() {
    bus.$emit(`set-topbar-title`, { title: '对象存储', breadcrumb: [] });
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    });
    this.getStorage();
  }
};
</script>


<style lang="less">
.setting-storage-container {
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
    .storage-list {
      padding-bottom: 30px;
    }
  }
  .dialog-style {
    .el-dialog__body {
      padding: 0px 20px;
    }
  }
}
</style>
