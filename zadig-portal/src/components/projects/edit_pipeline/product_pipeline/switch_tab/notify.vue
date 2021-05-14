<template>
  <div class="notify">
    <el-card class="box-card">
      <div>
        <div class="script dashed-container">
          <span class="title">通知</span>
        </div>
        <div class="notify dashed-container">
          <div class="notify-container">
            <el-form :model="notify"
                     :rules="notifyRules"
                     label-position="top"
                     ref="notify">
              <el-form-item prop="webhook_type">
                <span slot="label">
                  <span>Webhook 类型：</span>
                </span>
                <el-select v-model="notify.webhook_type"
                           @change="clearForm"
                           style="width:350px"
                           size="small"
                           placeholder="请选择类型">
                  <el-option label="钉钉"
                             value="dingding">
                  </el-option>
                  <el-option label="企业微信"
                             value="wechat">
                  </el-option>
                  <el-option label="飞书"
                             value="feishu">
                  </el-option>
                </el-select>
              </el-form-item>
              <el-form-item v-if="notify.webhook_type==='feishu'"
                            prop="feishu_webhook">
                <span slot="label">
                  <span>Webhook 地址：
                    <el-tooltip class="item"
                                effect="dark"
                                content="点击查看飞书 webhook 配置文档"
                                placement="top">
                      <a class="help-link"
                         href="https://docs.koderover.com/zadig/project/workflow/#%E7%8A%B6%E6%80%81%E9%80%9A%E7%9F%A5"
                         target="_blank"><i class="el-icon-question"></i></a>
                    </el-tooltip></span>
                </span>
                <el-input style="width:350px"
                          size="small"
                          v-model="notify.feishu_webhook"></el-input>
              </el-form-item>
              <el-form-item v-if="notify.webhook_type==='wechat'"
                            prop="weChat_webHook">
                <span slot="label">
                  <span>Webhook 地址：
                    <el-tooltip class="item"
                                effect="dark"
                                content="点击查看企业微信 webhook 配置文档"
                                placement="top">
                      <a class="help-link"
                         href="https://docs.koderover.com/zadig/project/workflow/#%E7%8A%B6%E6%80%81%E9%80%9A%E7%9F%A5"
                         target="_blank"><i class="el-icon-question"></i></a>
                    </el-tooltip></span>
                </span>
                <el-input style="width:350px"
                          size="small"
                          v-model="notify.weChat_webHook"></el-input>
              </el-form-item>
              <el-form-item v-if="notify.webhook_type==='dingding'"
                            prop="dingding_webhook">
                <span slot="label">
                  <span>Webhook 地址：
                    <el-tooltip class="item"
                                effect="dark"
                                content="点击查看钉钉 webhook 配置文档"
                                placement="top">
                      <a class="help-link"
                         href="https://docs.koderover.com/zadig/project/workflow/#%E7%8A%B6%E6%80%81%E9%80%9A%E7%9F%A5"
                         target="_blank"><i class="el-icon-question"></i></a>
                    </el-tooltip></span>
                </span>
                <el-input style="width:350px"
                          size="small"
                          v-model="notify.dingding_webhook"></el-input>
              </el-form-item>
              <el-form-item v-if="notify.webhook_type==='dingding'"
                            prop="at_mobiles">
                <span slot="label">
                  <span>@指定成员（输入指定通知接收人的手机号码，使用 ; 分割，为空则全员通知）：</span>
                </span>
                <el-input style="width:350px"
                          type="textarea"
                          :rows="3"
                          placeholder="输入指定通知接收人的手机号码，使用用 ; 分割"
                          v-model="mobileStr">
                </el-input>
              </el-form-item>

              <el-form-item prop="notify_type"
                            label="通知事件：">
                <el-checkbox :indeterminate="isIndeterminate"
                             v-model="checkAll"
                             @change="handleCheckAllChange">全选</el-checkbox>
                <el-checkbox-group @change="handleCheckedValueChange"
                                   v-model="notify.notify_type">
                  <el-checkbox label="passed">任务成功</el-checkbox>
                  <el-checkbox label="failed">任务失败</el-checkbox>
                  <el-checkbox label="timeout">任务超时</el-checkbox>
                  <el-checkbox label="cancelled">任务取消</el-checkbox>
                </el-checkbox-group>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus';

export default {
  data() {
    return {
      isIndeterminate: true,
      notifyRules: {
        webhook_type: [
          {
            type: 'string',
            required: true,
            message: '选择通知类型',
            trigger: 'blur'
          }
        ],
        weChat_webHook: [
          {
            type: 'string',
            required: true,
            message: '请填写企业微信 Webhook',
            trigger: 'blur'
          }
        ],
        dingding_webhook: [
          {
            type: 'string',
            required: true,
            message: '请填写钉钉 Webhook',
            trigger: 'blur'
          }
        ],
        feishu_webhook: [
          {
            type: 'string',
            required: true,
            message: '请填写飞书 Webhook',
            trigger: 'blur'
          }
        ],
        notify_type: [
          {
            type: 'array',
            required: true,
            message: '请选择通知类型',
            trigger: 'blur'
          }
        ]
      }
    };
  },
  computed: {
    checkAll: {
      get: function () {
        return this.notify.notify_type.length === 4;
      },
      set: function (newValue) {
      }
    },
    'notify.at_mobiles': {
      get: function () {
        return this.mobileStr.split(';');
      }
    },
    'mobileStr': {
      get: function () {
        if (this.notify.at_mobiles) {
          return this.notify.at_mobiles.join(';');
        }
        else {
          return '';
        }
      },
      set: function (newValue) {
        if (newValue === '') {
          this.$set(this.notify, 'is_at_all', true);
          this.$set(this.notify, 'at_mobiles', []);
        }
        else {
          this.$set(this.notify, 'is_at_all', false);
          this.$set(this.notify, 'at_mobiles', newValue.split(';'));
        }
      }
    }

  },
  methods: {
    handleCheckAllChange(val) {
      this.notify.notify_type = val ? ['timeout', 'passed', 'failed', 'cancelled'] : [];
      this.isIndeterminate = false;
    },
    handleCheckedValueChange(value) {
      let checkedCount = value.length;
      this.checkAll = (checkedCount === 4);
      this.isIndeterminate = (checkedCount > 0 && checkedCount < 4);
    },
    clearForm() {
      this.$refs['notify'].clearValidate();
    }
  },
  props: {
    notify: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
  },

  created() {
    bus.$on('check-tab:notify', () => {
      this.$refs.notify.validate(valid => {
        bus.$emit('receive-tab-check:notify', valid);
      });
    });
  },
  beforeDestroy() {
    bus.$off('check-tab:notify');
  },

};
</script>

<style lang="less">
.notify {
  .box-card {
    .el-card__header {
      text-align: center;
    }
    .el-form {
      .el-form-item {
        margin-bottom: 5px;
        .env-form-inline {
          width: 100%;
        }
      }
    }
    .divider {
      height: 1px;
      background-color: #dfe0e6;
      margin: 13px 0;
      width: 100%;
    }

    .notify-container {
      margin: 10px 0;
    }
    .help-link {
      color: #303133;
    }
    .script {
      padding: 5px 0px;
      .title {
        display: inline-block;
        color: #606266;
        font-size: 14px;
        padding-top: 6px;
        width: 100px;
      }
      .item-title {
        color: #909399;
        margin-left: 5px;
      }
    }
  }
}
</style>
