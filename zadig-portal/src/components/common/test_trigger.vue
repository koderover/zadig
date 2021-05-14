<template>
  <div class="test-trigger">
    <!-- start of edit webhook dialog -->
    <el-dialog width="40%"
                :title="webhookEditMode?'修改触发器配置':'添加触发器'"
               :visible.sync="showWebhookDialog"
               :close-on-click-modal="false"
               custom-class="add-trigger-dialog"
               center>
      <el-form :model="webhook"
               label-position="left"
               label-width="90px">
        <el-form-item label="代码库">
          <el-select v-model="webhookSwap.repo"
                     size="small"
                     @change="repoChange(webhookSwap.repo)"
                     filterable
                     clearable
                     value-key="repo_name"
                     placeholder="请选择">
            <el-option v-for="(repo,index) in webhookRepos"
                       :key="index"
                       :label="repo.repo_owner+'/'+repo.repo_name"
                       :value="repo">
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="目标分支">
          <el-select v-model="webhookSwap.repo.branch"
                     size="small"
                     filterable
                     clearable
                     placeholder="请选择">
            <el-option v-for="(branch,index) in webhookBranches[webhookSwap.repo.repo_name]"
                       :key="index"
                       :label="branch.name"
                       :value="branch.name">
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item v-if="webhookSwap.repo.source==='gerrit'"
                      label="触发事件">
          <el-checkbox-group v-model="webhookSwap.events">
            <el-checkbox style="display:block"
                         label="change-merged"></el-checkbox>
            <el-checkbox style="display:block"
                         label="patchset-created">
              <template v-if="webhookSwap.events.includes('patchset-created')">
                <span>patchset-created</span>
                <span style="color:#606266">评分标签</span>
                <el-input size="mini"
                          style="width:250px"
                          v-model="webhookSwap.repo.label"
                          placeholder="Code-Review"></el-input>
              </template>
            </el-checkbox>

          </el-checkbox-group>

        </el-form-item>
        <el-form-item v-else-if="webhookSwap.repo.source!=='gerrit'"
                      label="触发事件">
          <el-checkbox-group v-model="webhookSwap.events">
            <el-checkbox label="push"></el-checkbox>
            <el-checkbox label="pull_request"></el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="自动取消">
          <span slot="label">
            <span>自动取消</span>
            <el-tooltip effect="dark"
                        content="如果你希望只构建最新的提交，则使用这个选项会自动取消队列中的任务"
                        placement="right">
              <i class="el-icon-question"></i>
            </el-tooltip>
          </span>
          <el-checkbox v-model="webhookSwap.auto_cancel"></el-checkbox>
        </el-form-item>
        <el-form-item v-if="webhookSwap.repo.source!=='gerrit'"
                      label="文件目录">
          <el-input :autosize="{ minRows: 4, maxRows: 10}"
                    type="textarea"
                    v-model="webhookSwap.match_folders"
                    placeholder="输入目录时，多个目录请用回车换行分隔"></el-input>
        </el-form-item>
        <ul v-if="webhookSwap.repo.source!=='gerrit'"
            style="padding-left:80px">
          <li> "/" 表示代码库中的所有文件</li>
          <li> 用 "!" 符号开头可以排除相应的文件</li>
        </ul>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button size="small"
                   round
                   @click="webhookAddMode?webhookAddMode=false:webhookEditMode=false">取 消
        </el-button>
        <el-button size="small"
                   round
                   type="primary"
                   @click="webhookAddMode?addWebhook():saveWebhook()">确定</el-button>
      </div>
    </el-dialog>
    <!--end of edit webhook dialog -->

    <div class="content dashed-container">
      <div class="trigger-container">
        <div v-if="webhook.enabled && webhook.items.length >0"
             class="trigger-list">
          <el-table :data="webhook.items"
                    style="width: 100%">
            <el-table-column label="代码库拥有者">
              <template slot-scope="scope">
                <span>{{ scope.row.main_repo.repo_owner }}</span>
              </template>
            </el-table-column>
            <el-table-column label="代码库">
              <template slot-scope="scope">
                <span>{{ scope.row.main_repo.repo_name }}</span>
              </template>
            </el-table-column>
            <el-table-column label="目标分支">
              <template slot-scope="scope">
                <span>{{ scope.row.main_repo.branch }}</span>
              </template>
            </el-table-column>
            <el-table-column label="触发方式">
              <template slot-scope="scope">
                <span>{{ scope.row.main_repo.events.join() }}</span>
              </template>
            </el-table-column>
            <el-table-column label="文件目录">
              <template slot-scope="scope">
                <span
                      v-if="scope.row.main_repo.source!=='gerrit'">{{ scope.row.main_repo.match_folders.join() }}</span>
                <span v-else-if="scope.row.main_repo.source==='gerrit'"> N/A </span>
              </template>
            </el-table-column>
            <el-table-column label="操作">
              <template slot-scope="scope">
                <el-button @click.native.prevent="editWebhook(scope.$index)"
                           type="text"
                           size="small">
                  编辑
                </el-button>
                <el-button @click.native.prevent="deleteWebhook(scope.$index)"
                           type="text"
                           size="small">
                  移除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </div>
  </div>
</template>

<script type="text/javascript">
import { getBranchInfoByIdAPI } from '@api';
export default {
  data() {
    return {
      showTriggerParamsDialog: false,
      webhookBranches: {},
      webhookSwap: {
        auto_cancel: false,
        repo: {},
        events: [],
        match_folders: '/\n!.md'
      },
      currenteditWebhookIndex: null,
      webhookEditMode: false,
      webhookAddMode: false,
    };
  },
  props: {
    webhook: {
      required: true,
      type: Object
    },
    testName: {
      required: true,
      type: String
    },
    editMode: {
      required: false,
      default: false,
      type: Boolean
    },
    projectName: {
      required: true,
      type: String
    },
    avaliableRepos: {
      required: true,
      type: Array
    }
  },
  methods: {
    editWebhook(index) {
      this.webhookEditMode = true;
      this.currenteditWebhookIndex = index;
      let webhookSwap = this.$utils.cloneObj(this.webhook.items[index]);
      this.getBranchInfoById(webhookSwap.main_repo.codehost_id, webhookSwap.main_repo.repo_owner, webhookSwap.main_repo.repo_name);
      this.webhookSwap = {
        auto_cancel: webhookSwap.auto_cancel,
        repo: webhookSwap.main_repo,
        events: webhookSwap.main_repo.events,
        match_folders: webhookSwap.main_repo.match_folders.join('\n'),
      }
    },
    addWebhookBtn() {
      this.webhookAddMode = true;
      this.webhook.enabled = true;
      this.webhookSwap = {
        auto_cancel: false,
        repo: {},
        events: [],
        match_folders: '/\n!.md'
      };
    },
    addWebhook() {
      let webhookSwap = this.$utils.cloneObj(this.webhookSwap);
      webhookSwap.repo.match_folders = webhookSwap.match_folders.split('\n');
      webhookSwap.repo.events = webhookSwap.events;
      this.webhook.items.push({
        main_repo: webhookSwap.repo,
        auto_cancel: webhookSwap.auto_cancel,
        test_args: {
          test_name: this.testName,
          product_name: this.projectName
        }
      });
      this.webhookSwap = {
        auto_cancel: false,
        repo: {},
        events: [],
        match_folders: '/\n!.md',
      };
      this.webhookAddMode = false;
    },
    saveWebhook() {
      const index = this.currenteditWebhookIndex;
      let webhookSwap = this.$utils.cloneObj(this.webhookSwap);
      webhookSwap.repo.match_folders = webhookSwap.match_folders.split('\n');
      webhookSwap.repo.events = webhookSwap.events;
      this.$set(this.webhook.items, index, {
        auto_cancel: webhookSwap.auto_cancel,
        main_repo: webhookSwap.repo,
        test_args: {
          test_name: this.testName,
          product_name: this.projectName
        }
      });
      this.webhookSwap = {
        auto_cancel: false,
        repo: {},
        events: [],
        match_folders: '/\n!.md',
      };
      this.webhookEditMode = false;
    },
    deleteWebhook(index) {
      this.webhook.items.splice(index, 1);
      if (this.webhook.items.length === 0) {
        this.webhook.enabled = false;
      }
    },
    getBranchInfoById(id, repo_owner, repo_name) {
      getBranchInfoByIdAPI(id, repo_owner, repo_name).then((res) => {
        this.$set(this.webhookBranches, repo_name, res);
      });
    },
    repoChange(currentRepo) {
      this.webhookSwap.events = [];
      this.getBranchInfoById(currentRepo.codehost_id, currentRepo.repo_owner, currentRepo.repo_name);
    }
  },
  computed: {
    showWebhookDialog: {
      get: function () {
        return this.webhookAddMode ? this.webhookAddMode : this.webhookEditMode;
      },
      set: function (newValue) {
        this.webhookAddMode ? this.webhookAddMode = newValue : this.webhookEditMode = newValue;
      }
    },
    webhookRepos: {
      get: function () {
        return this.avaliableRepos;
      }
    },
  },
  created() {

  },

  components: {
  }
};
</script>

<style lang="less">
.test-trigger {
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
    .el-card__body {
      padding: 0px;
    }
    .divider {
      height: 1px;
      background-color: #dfe0e6;
      margin: 13px 0;
      width: 100%;
    }
    .help-link {
      color: #1989fa;
    }

    .trigger-container {
      margin: 10px 0;
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
