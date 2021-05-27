<template>
  <div class="inte-registry">
    <el-alert type="warning">
      <div>
        镜像仓库集成可参考
        <el-link type="warning"
                 style="font-weight: 600;"
                 :href="`https://docs.koderover.com/zadig/settings/image-registry/`"
                 :underline="false"
                 target="_blank">
          这里
        </el-link>
        ！
      </div>
    </el-alert>
    <div class="content">
      <el-form ref="registry"
               :rules="rules"
               label-width="110px"
               label-position="top"
               :model="registry">
        <el-form-item label="默认使用"
                      prop="is_default">
          <el-checkbox v-model="registry.is_default"></el-checkbox>
        </el-form-item>
        <el-form-item label="地址"
                      prop="reg_addr">
          <el-input v-model="registry.reg_addr"></el-input>
        </el-form-item>
        <el-form-item label="Namespace"
                      prop="namespace">
          <el-input v-model="registry.namespace"></el-input>
        </el-form-item>
        <el-form-item :rules="{required: false}"
                      label="Docker 用户名"
                      prop="access_key">
          <el-input v-model="registry.access_key"></el-input>
        </el-form-item>
        <el-form-item :rules="{required: false}"
                      label="Docker 密码"
                      prop="secret_key">
          <el-input type="passsword"
                    v-model="registry.secret_key"></el-input>
        </el-form-item>
      </el-form>
      <div class="footer">
        <el-button :plain="true"
                   type="success"
                   size="small"
                   @click="registryOperation">保存</el-button>
        <el-button @click="$emit('cancel')"
                   size="small">取 消</el-button>
      </div>
    </div>
  </div>
</template>

<script>
import { createRegistryAPI } from '@api';
export default {
  name: 'IntegrationRegistry',
  data() {
    return {
      registry: {
        namespace: '',
        reg_addr: '',
        access_key: '',
        secret_key: '',
        reg_provider: 'native',
        is_default: false,
      },
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
    }
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    }
  },
  methods: {
    registryOperation() {
      this.$refs['registry'].validate(valid => {
        if (valid) {
          let payload = this.registry;
          payload.org_id = this.currentOrganizationId;
          createRegistryAPI(payload).then((res) => {
            this.$message({
              type: 'success',
              message: '新增成功'
            });
            this.$emit('cancel')
            this.$emit('createSuccess')
          })
        } else {
          return false;
        }
      });
    }
  }
}
</script>

<style lang="less" scoped>
.inte-registry {
  padding: 10px 15px;
  font-size: 13px;
  .content {
    padding: 30px 20px;
    /deep/ .el-form-item{
      margin-bottom: 10px;
      &:first-child .el-form-item__content{
        display: inline-block;
        top: -4px;
        padding-left: 30px;
      }
    }
  }
  .footer {
    padding-top: 20px;
  }
}
</style>