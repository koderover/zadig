<template>
  <el-dialog
    :title="(isEdit ? '编辑': '新增') + 'Jenkins 集成'"
    custom-class="edit-form-dialog"
    :visible.sync="dialogVisible"
    center
  >
    <el-alert
      style="margin-bottom: 10px"
      v-if="checkRes === 'fail'"
      :title="errorMessage"
      type="error"
      :closable="false"
      show-icon>
    </el-alert>
    <el-form
      :model="addForm"
      ref="addForm"
      :rules="formRules"
      class="form"
      label-position="left"
      label-width="90px"
    >
      <el-form-item label="服务地址" prop="url">
        <el-input
          v-model="addForm.url"
          @change="validate(checkPassword)"
          placeholder="Jenkins 服务地址"
        ></el-input>
      </el-form-item>
      <el-form-item label="用户名" prop="username">
        <el-input
          @change="validate(checkPassword)"
          v-model="addForm.username"
          placeholder="Jenkins 用户名"
        ></el-input>
      </el-form-item>
      <el-form-item label="API Token" prop="password">
        <el-input
          @change="validate(checkPassword)"
          v-model="addForm.password"
          :suffix-icon="showCheckIcon"
          placeholder="Jenkins API Token"
          class="input"
        ></el-input>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :disabled="checkRes!=='pass'" size="small" @click="validate(save)">保存</el-button>
      </el-form-item>
    </el-form>
  </el-dialog>
</template>
<script>
import { addJenkins, jenkinsConnection, editJenkins } from '@/api'
export default {
  name: 'addJenkins',
  props: {
    getJenkins: Function,
  },
  data() {
    return {
      dialogVisible: false,
      checkRes: null,
      isEdit: false,
      errorMessage: '',
      addForm: {
        url: null,
        username: null,
        password: null,
      },
      formRules: {
        url: [
          { required: true, message: '服务地址不能为空', trigger: 'blur' },
          {
            pattern: /(http|ftp|https):\/\/[\w\-_]+(\.[\w\-_]+)+([\w\-\.,@?^=%&:/~\+#]*[\w\-\@?^=%&/~\+#])?/,
            message: '请输入正确的服务地址',
            trigger: 'blur',
          },
        ],
        username: [
          { required: true, message: '用户名不能为空', trigger: 'blur' },
        ],
        password: [
          { required: true, message: 'API token不能为空', trigger: 'blur' },
        ],
      },
    }
  },
  methods: {
    openDialog(data) {
      this.dialogVisible = true
      if (data) {
        this.isEdit = true
        this.addForm = data
      } else {
        this.isEdit = false
      }
    },
    async save() {
      if (this.isEdit) {
        this.edit()
      } else {
        let res = await addJenkins(this.addForm).catch((error) =>
          console.log(error)
        )
        if (res && res.message === 'success') {
          this.$message.success('新增成功')
          this.dialogVisible = false
          this.getJenkins()
        }
      }
    },
    async edit() {
      let res = await editJenkins(this.addForm).catch((error) =>
        console.log(error)
      )
      if (res && res.message === 'success') {
        this.$message.success('保存成功')
        this.dialogVisible = false
        this.getJenkins()
      }
    },
    async checkPassword() {
      this.check = true
      let res = await jenkinsConnection(this.addForm).catch(error => {
        this.checkRes = 'fail'
        this.errorMessage = error.response.data.message
       })
      if (res && res.message === 'success') {
        this.checkRes = 'pass'
      }
    },
    validate(fn) {
      this.$refs.addForm.validate((valid) => {
        if (valid) {
          fn()
        } else {
          return false
        }
      })
    },
    resetForm() {
      this.addForm = {
        url: '',
        username: '',
        password: '',
      }
    },
  },
  computed: {
    showCheckIcon() {
      if (this.checkRes === 'pass') {
        return 'el-icon-success'
      } else if (this.checkRes === 'fail') {
        return 'el-icon-error'
      } else {
        return ''
      }
    },
  },
  watch: {
    dialogVisible(value) {
      if (!value) {
        this.$refs.addForm.resetFields()
        this.checkRes = null
        this.resetForm()
      }
    },
  },
}
</script>
<style lang="less" scoped>
/deep/ .edit-form-dialog {
    width: 550px;
    .el-dialog__header {
      text-align: center;
      border-bottom: 1px solid #e4e4e4;
      padding: 15px;
      .el-dialog__close {
        font-size: 10px;
      }
    }
    .el-dialog__body {
      padding-bottom: 0px;
    }
    .el-dialog__body {
      padding: 30px 20px;
      color: #606266;
      font-size: 14px;
      .el-form-item {
        margin-bottom: 15px;
      }
    }

    .el-select {
      width: 100%;
    }
    .el-input {
      display: inline-block;
    }
  }
/deep/ .el-icon-success {
  color: #67c23a;
  font-size: 20px;
  margin-left: 10px;
}
/deep/ .el-icon-error {
  color: red;
  font-size: 20px;
  margin-left: 10px;
}
/deep/ .el-input {
  width: 400px;
}
</style>