<template>
  <el-dialog
    class="env-recycle-dialog"
    title="环境回收-策略设定"
    :visible.sync="recycleEnvDialogVisible"
    width="400px"
  >
    <el-form
      :model="recycleEnv"
      :rules="recycleEnvRules"
      ref="recycleEnvForm"
      hide-required-asterisk
    >
      <el-form-item label="环境未更新，则">
        <el-input-number
          size="mini"
          v-model="recycleEnv.recycleDay"
          :min="0"
          label="环境回收时间"
        ></el-input-number>
        <span>天后自动回收</span>
        <p v-if="recycleEnv.recycleDay === 0" class="tips">
          <el-link :underline="false" type="primary">环境永不回收</el-link>
        </p>
        <p v-if="recycleEnv.recycleDay > 0" class="tips">
          <el-link :underline="false" type="primary"
            >环境将在
            {{
              $utils.convertTimestamp(
                productInfo.update_time + recycleEnv.recycleDay * 86400
              )
            }}
            被回收
          </el-link>
        </p>
      </el-form-item>
    </el-form>
    <span slot="footer" class="dialog-footer">
      <el-button size="small" type="primary" @click="submitEnvRecycleInfo"
        >确 定</el-button
      >
      <el-button size="small" @click="cancelEnvRecyleInfo">取 消</el-button>
    </span>
  </el-dialog>
</template>
<script>
import { recycleEnvAPI } from '@/api'
export default {
  name: 'recycleDialog',
  props: {
    recycleDay: Number,
    initPageInfo: Function,
    getProductEnv: Function,
    productInfo: Object
  },
  data () {
    return {
      recycleEnvDialogVisible: false,
      recycleEnv: {
        recycleDay: null
      },
      recycleEnvRules: {
        recycleDay: [
          {
            type: 'number',
            required: true,
            message: '请填写回收时间',
            trigger: 'hover'
          }
        ]
      }
    }
  },
  methods: {
    openDialog () {
      this.recycleEnvDialogVisible = true
    },
    submitEnvRecycleInfo () {
      this.$refs.recycleEnvForm.validate((valid) => {
        if (valid) {
          this.recycleEnvUpdate(
            this.productInfo.product_name,
            this.productInfo.env_name,
            this.recycleEnv.recycleDay ? this.recycleEnv.recycleDay : 0
          )
        }
      })
    },
    cancelEnvRecyleInfo () {
      this.recycleEnvDialogVisible = false
    },
    recycleEnvUpdate (product_name, env_name, recycle_day) {
      recycleEnvAPI(product_name, env_name, recycle_day).then((res) => {
        this.$message.success('设置环境回收策略成功')
        this.initPageInfo()
        this.getProductEnv()
        this.recycleEnvDialogVisible = false
      })
    }
  },
  watch: {
    recycleEnvDialogVisible (value) {
      if (value) {
        this.recycleEnv.recycleDay = this.recycleDay
      }
    }
  }
}
</script>
