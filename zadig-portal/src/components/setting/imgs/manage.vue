<template>
  <div v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading iconjingxiang"
       class="setting-img-container">
    <!--imgs-create-dialog-->
    <el-dialog title='添加自定义镜像'
               width="40%"
               :close-on-click-modal="false"
               custom-class="create-img-dialog"
               :visible.sync="dialogImgCreateFormVisible">
      <el-form ref="createImg"
               :rules="rules"
               :model="createImg"
               label-width="125px">
        <el-form-item label="标签"
                      prop="label">
          <el-input size="small"
                    v-model="createImg.label"></el-input>
        </el-form-item>
        <el-form-item label="自定义镜像名称"
                      prop="value">
          <el-input size="small"
                    placeholder="仓库地址/命名空间/镜像名:标签"
                    v-model="createImg.value"></el-input>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button size="small"
                   @click="dialogImgCreateFormVisible = false">取 消</el-button>
        <el-button :plain="true"
                   type="success"
                   size="small"
                   @click="addImg">保存</el-button>
      </div>
    </el-dialog>
    <!--imgs-create-dialog-->

    <!--imgs-edit-dialog-->
    <el-dialog title='修改自定义镜像'
               custom-class="create-img-dialog"
               :close-on-click-modal="false"
               :visible.sync="dialogImgEditFormVisible">
      <el-form ref="updateImg"
               :rules="rules"
               :model="swapImg"
               label-width="125px">
        <el-form-item label="标签"
                      prop="label">
          <el-input size="small"
                    disabled
                    v-model="swapImg.label"></el-input>
        </el-form-item>
        <el-form-item label="自定义镜像名称"
                      prop="value">
          <el-input size="small"
                    v-model="swapImg.value"></el-input>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button size="small"
                   @click="dialogImgEditFormVisible = false">取 消</el-button>
        <el-button size="small"
                   :plain="true"
                   @click="updateImg"
                   type="success">保存</el-button>
      </div>
    </el-dialog>
    <!--imgs-edit-dialog-->
    <div class="section">
      <el-alert type="info"
                :closable="false">
        <template slot>
          <span>项目的构建和测试可以使用自定义构建镜像作为脚本执行的基础环境</span><br>
          <span>自定义镜像需要添加 Zadig 系统所需的一些必要组件，详细可参考
            <el-link style="vertical-align: baseline;"
                     type="primary"
                     href="https://docs.koderover.com/zadig/settings/custom-image/"
                     :underline="false"
                     target="_blank">帮助文档</el-link>
          </span><br>
        </template>
      </el-alert>
      <div class="sync-container">
        <el-button :plain="true"
                   @click="dialogImgCreateFormVisible=true"
                   size="small"
                   type="success">添加自定义镜像</el-button>
      </div>
      <div class="img-list">
        <template>
          <el-table :data="imgs"
                    style="width: 100%;">
            <el-table-column label="标签">
              <template slot-scope="scope">
                <span>{{scope.row.label}}</span>
              </template>
            </el-table-column>
            <el-table-column label="自定义镜像名称">
              <template slot-scope="scope">
                <span>{{scope.row.value}}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作">
              <template slot-scope="scope">
                <el-button @click="editImg(scope.row)"
                           size="mini">编辑</el-button>
                <el-button size="mini"
                           @click="deleteImg(scope.row)"
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

import { getImgListAPI, addImgAPI, deleteImgAPI, updateImgAPI } from '@api'
import bus from '@utils/event_bus'
export default {
  data () {
    return {
      dialogImgCreateFormVisible: false,
      dialogImgEditFormVisible: false,
      loading: false,
      createImg: {
        label: '',
        value: '',
        image_from: 'custom'
      },
      swapImg: {
        label: '',
        value: '',
        image_from: 'custom'
      },
      imgs: [],
      rules: {
        label: [{ required: true, message: '请填写镜像标签', trigger: 'blur' }],
        value: [{ required: true, message: '请填写镜像名称', trigger: 'blur' }]
      }
    }
  },
  methods: {
    editImg (data) {
      this.dialogImgEditFormVisible = true
      this.swapImg = this.$utils.cloneObj(data)
    },
    deleteImg (data) {
      this.$confirm(`确定要删除 ${data.label} 这个镜像吗？`, '删除镜像确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteImgAPI(data.id).then(
          res => {
            this.getImgList()
            this.$message({
              message: '删除镜像成功',
              type: 'success'
            })
          }
        )
      })
    },
    addImg () {
      this.$refs.createImg.validate((valid) => {
        if (valid) {
          const payload = this.$utils.cloneObj(this.createImg)
          addImgAPI(payload).then(
            res => {
              this.dialogImgCreateFormVisible = false
              this.$refs.createImg.resetFields()
              this.getImgList()
              this.$message({
                message: '新增镜像成功',
                type: 'success'
              })
            }
          )
        } else {
          return false
        }
      })
    },
    updateImg () {
      this.$refs.updateImg.validate((valid) => {
        if (valid) {
          const payload = this.$utils.cloneObj(this.swapImg)
          const id = payload.id
          updateImgAPI(id, payload).then(
            res => {
              this.dialogImgEditFormVisible = false
              this.$refs.updateImg.resetFields()
              this.getImgList()
              this.$message({
                message: '更新镜像成功',
                type: 'success'
              })
            }
          )
        } else {
          return false
        }
      })
    },
    getImgList () {
      this.loading = true
      getImgListAPI('custom').then(
        res => {
          this.loading = false
          this.imgs = res
        }
      )
    }
  },
  computed: {

  },
  created () {
    bus.$emit(`set-topbar-title`, { title: '构建镜像管理', breadcrumb: [] })
    bus.$emit(`set-sub-sidebar-title`, {
      title: '',
      routerList: []
    })
    this.getImgList()
  },
  components: {
  }
}
</script>

<style lang="less">
.setting-img-container {
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

    .img-list {
      padding-bottom: 30px;
    }

    .create-img-dialog {
      .el-dialog__body {
        padding: 0 20px;
      }

      .el-form-item {
        margin-bottom: 10px;
      }
    }
  }
}
</style>
