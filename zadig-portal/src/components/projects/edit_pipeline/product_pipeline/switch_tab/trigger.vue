<template>
  <div class="trigger">
    <!-- start of edit webhook dialog -->
    <el-dialog width="40%"
               :title="webhookEditMode?'修改触发器配置':'添加触发器'"
               :visible.sync="showWebhookDialog"
               :close-on-click-modal="false"
               @close="closeWebhookDialog"
               custom-class="add-trigger-dialog"
               center>
      <el-form :model="webhook"
               label-position="left"
               label-width="80px">
        <el-form-item label="代码库">
          <el-select v-model="webhookSwap.repo"
                     size="small"
                     @change="repoChange(webhookSwap.repo)"
                     filterable
                     clearable
                     value-key="key"
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
        <el-form-item label="部署环境">
          <el-select v-model="webhookSwap.namespace"
                     multiple
                     filterable
                     @change="changeNamespace"
                     size="small"
                     placeholder="请选择">
            <el-option v-for="pro of matchedProducts"
                       :key="`${pro.product_name} / ${pro.env_name}`"
                       :label="`${pro.product_name} / ${pro.env_name}（${pro.is_prod?'生产':'测试'}）`"
                       :value="`${pro.env_name}`">
              <span>{{`${pro.product_name} / ${pro.env_name}`}}
                <el-tag v-if="pro.is_prod"
                        type="danger"
                        size="mini"
                        effect="dark">
                  生产
                </el-tag>
              </span>
            </el-option>
          </el-select>
          <el-button @click="showEnvUpdatePolicy = !showEnvUpdatePolicy"
                     class="env-open-button"
                     size="mini"
                     plain>
            环境更新策略
            <i class="el-icon-arrow-left"></i>
          </el-button>
        </el-form-item>
        <div class="env-update-list"
             v-show="showEnvUpdatePolicy">
          <p>环境更新策略</p>
          <el-radio-group v-model="webhookSwap.env_update_policy">
            <el-tooltip content="目前一个触发任务仅支持更新单个环境，部署环境指定单个环境时可选"
                        placement="right">
              <el-radio label="all"
                        :disabled="!(webhookSwap.namespace.length===1)">
                更新指定环境
              </el-radio>
            </el-tooltip>
            <el-tooltip content="动态选择一套“没有工作流任务正在更新”的环境进行验证"
                        placement="right">
              <el-radio label="single">
                动态选择空闲环境更新
              </el-radio>
            </el-tooltip>
            <el-tooltip v-if="showPrEnv && webhookSwap.repo.source==='gitlab'"
                        content="基于基准环境版本生成一套临时测试环境做 PR 级验证"
                        placement="right">
              <el-radio label="base"
                        :disabled="!(webhookSwap.namespace.length===1 && showPrEnv && webhookSwap.repo.source==='gitlab')">
                设置指定环境为基准环境
              </el-radio>
            </el-tooltip>
          </el-radio-group>
        </div>
        <el-form-item v-if="webhookSwap.env_update_policy === 'base' && showPrEnv && webhookSwap.repo.source==='gitlab' && showEnvUpdatePolicy"
                      label="销毁策略">
          <el-select v-model="webhookSwap.env_recycle_policy"
                     size="small"
                     placeholder="请选择销毁策略">
            <el-option label="工作流成功之后销毁"
                       value="success">
            </el-option>
            <el-option label="每次销毁"
                       value="always">
            </el-option>
            <el-option label="每次保留"
                       value="never">
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="部署服务">
          <el-select v-model="webhookSwap.targets"
                     multiple
                     filterable
                     value-key="key"
                     size="small"
                     placeholder="请选择">
            <el-option v-for="(target,index) in webhookTargets"
                       :key="index"
                       :label="`${target.name}(${target.service_name})`"
                       :value="target">
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item v-if="webhookSwap.repo.source==='gerrit'"
                      label="触发事件">
          <el-checkbox-group v-model="webhookSwap.events">
            <el-checkbox style="display: block;"
                         label="change-merged"></el-checkbox>
            <el-checkbox style="display: block;"
                         label="patchset-created">
              <template v-if="webhookSwap.events.includes('patchset-created')">
                <span>patchset-created</span>
                <span style="color: #606266;">评分标签</span>
                <el-input size="mini"
                          style="width: 250px;"
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
        <el-form-item label="触发策略">
          <el-checkbox v-model="webhookSwap.auto_cancel">
            <span>自动取消</span>
            <el-tooltip effect="dark"
                        content="如果你希望只构建最新的提交，则使用这个选项会自动取消队列中的任务"
                        placement="top">
              <i class="el-icon-question"></i>
            </el-tooltip>
          </el-checkbox>
          <el-checkbox v-if="webhookSwap.repo.source==='gerrit'"
                       v-model="webhookSwap.check_patch_set_change">
            <span>代码无变化时不触发工作流</span>
            <el-tooltip effect="dark"
                        content="例外情况说明：当目标代码仓配置为 Gerrit 的情况下，受限于其 API 的能力，当单行代码有变化时也不被触发"
                        placement="top">
              <i class="el-icon-question"></i>
            </el-tooltip>
          </el-checkbox>
        </el-form-item>
        <el-form-item v-if="webhookSwap.repo.source!=='gerrit'"
                      label="文件目录">
          <el-input :autosize="{ minRows: 4, maxRows: 10}"
                    type="textarea"
                    v-model="webhookSwap.match_folders"
                    placeholder="输入目录时，多个目录请用回车换行分隔"></el-input>
        </el-form-item>
        <ul v-if="webhookSwap.repo.source!=='gerrit'"
            style="padding-left: 80px;">
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

    <el-card class="box-card">
      <div class="content dashed-container">
        <div>
          <span class="title">定时器</span>
          <el-switch v-model="schedules.enabled">
          </el-switch>
        </div>
        <div class="trigger dashed-container">
          <el-button v-if="schedules.enabled"
                     @click="addTimerBtn"
                     type="text">添加配置</el-button>
          <div class="add-border"
               v-if="schedules.enabled">
            <test-timer ref="timer"
                        timerType="project"
                        dialogWidth="65%"
                        dialogLeft
                        whichSave="outside"
                        :projectName="productTmlName"
                        :schedules="schedules">
              <!-- 添加参数 确定是产品工作流 -->
              <template v-slot:content="{ orgsObject, indexWork }">
                <div class="underline"></div>
                <div class="pipeline-header">工作流参数</div>
                <workflow-args :key="indexWork*(testInfos.length+1)"
                               :workflowName="workflowToRun.name"
                               :workflowMeta="workflowToRun"
                               :targetProduct="workflowToRun.product_tmpl_name"
                               :forcedUserInput="orgsObject || {}"
                               :testInfos="testInfos"
                               whichSave="outside"
                               ref="pipelineConfig"></workflow-args>
                <div class="timer-dialog-footer">
                  <el-button @click="addTimerSchedule"
                             size="small"
                             type="primary">确定</el-button>
                </div>
              </template>
            </test-timer>
          </div>
        </div>
      </div>
      <div class="content dashed-container webhook-container">
        <div>
          <span class="title"> Webhook
            <a href="https://docs.koderover.com/zadig/project/workflow/#git-webhook"
               target="_blank"
               rel="noopener noreferrer"> <i class="el-icon-question help-link"></i></a>
          </span>
          <el-switch v-model="webhook.enabled">
          </el-switch>
        </div>
        <div class="trigger-container">
          <div v-if="webhook.enabled"
               class="trigger-list">
            <el-button @click="addWebhookBtn"
                       type="text">添加配置</el-button>
            <el-table class="add-border"
                      :data="webhook.items"
                      style="width: 100%;">
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
              <el-table-column label="部署环境">
                <template slot-scope="scope">
                  <span>{{ scope.row.workflow_args.namespace }}</span>
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
    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus'
import workflowArgs from '../container/workflow_args.vue'
import { mapGetters } from 'vuex'
import { listProductAPI, getBranchInfoByIdAPI, singleTestAPI, getAllBranchInfoAPI } from '@api'
import { uniqBy, get } from 'lodash'
export default {
  data () {
    return {
      testInfos: [],
      gotScheduleRepo: false,
      currentForcedUserInput: {},
      products: [],
      webhookBranches: {},
      webhookSwap: {
        repo: {},
        events: [],
        targets: [],
        namespace: '',
        env_update_policy: 'all',
        auto_cancel: false,
        check_patch_set_change: false,
        base_namespace: '',
        env_recycle_policy: 'success',
        match_folders: '/\n!.md'
      },
      currenteditWebhookIndex: null,
      webhookEditMode: false,
      webhookAddMode: false,
      showEnvUpdatePolicy: false,
      firstShowPolicy: false
    }
  },
  props: {
    webhook: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
    productTmlName: {
      required: true,
      type: String
    },
    workflowToRun: {
      required: true,
      type: Object
    },
    schedules: {
      required: true,
      type: Object
    },
    presets: {
      required: true,
      type: Array
    }
  },
  methods: {
    getTestReposForQuery (testInfos) {
      const testRepos = testInfos.length > 0 ? this.$utils.flattenArray(testInfos.map(t => t.builds)) : []
      let testReposForQuery = {}
      const testReposDeduped = this.$utils.deduplicateArray(
        testRepos,
        re => `${re.repo_owner}/${re.repo_name}`
      )
      testReposForQuery = testReposDeduped.map(re => ({
        repo_owner: re.repo_owner,
        repo: re.repo_name,
        default_branch: re.branch,
        codehost_id: re.codehost_id
      }))
      return new Promise((resolve, reject) => {
        getAllBranchInfoAPI({ infos: testReposForQuery }).then(res => {
          // make these repo info more friendly
          for (const repo of res) {
            repo.prs.forEach(element => {
              element.pr = element.id
            })
            repo.branchPRsMap = this.$utils.arrayToMapOfArrays(repo.prs, 'targetBranch')
            repo.branchNames = repo.branches.map(b => b.name)
          }
          // and make a map
          const repoInfoMap = this.$utils.arrayToMap(res, re => `${re.repo_owner}/${re.repo}`)
          // prepare build/test repo for view
          // see watcher for allRepos
          for (const repo of testRepos) {
            this.$set(repo, '_id_', `${repo.repo_owner}/${repo.repo_name}`)
            const repoInfo = repoInfoMap[repo._id_]
            this.$set(repo, 'branchNames', repoInfo && repoInfo.branchNames)
            this.$set(repo, 'branchPRsMap', repoInfo && repoInfo.branchPRsMap)
            this.$set(repo, 'tags', repoInfo && repoInfo.tags)
            this.$set(repo, 'prNumberPropName', 'pr')
            this.$set(repo, 'releaseMethod', 'branch')
            // make sure branch/pr/tag is reactive
            this.$set(repo, 'branch', repo.branch || '')
            this.$set(repo, repo.prNumberPropName, repo[repo.prNumberPropName] || undefined)
            this.$set(repo, 'tag', repo.tag || '')
          }
          resolve(testInfos)
        }).catch(err => {
          console.log('Get test repo error', err)
          reject(testInfos)
        })
      })
    },
    getTestReposForSchedules () {
      const allPromise = []
      this.schedules.items.forEach((item) => {
        allPromise.push(this.getTestReposForQuery(item.workflow_args.tests || []))
      })
      Promise.all(allPromise).then(() => {
        const params = (this.workflowToRun.test_stage && this.workflowToRun.test_stage.tests && this.workflowToRun.test_stage.tests.map(t => { return t.test_name })) || []
        this.getTestInfos(params)
      })
    },
    getTestInfos (test_names = []) {
      const allPro = []
      test_names.forEach((test_name) => {
        allPro.push(new Promise((resolve, reject) => {
          singleTestAPI(test_name, this.product_name).then((res) => {
            const test = {}
            test.namespace = this.workflowToRun.env_name
            test.test_module_name = test_name
            test.envs = res.pre_test.envs
            test.builds = res.repos
            resolve(test)
          }).catch((err) => {
            reject(err)
          })
        }))
      })
      Promise.all(allPro).then(this.getTestReposForQuery).then((res) => {
        this.testInfos = res
        this.schedules.items.forEach((item) => {
          const savedTests = []
          const savedTestNames = []
          item.workflow_args.tests && item.workflow_args.tests.forEach((test) => {
            const test_names = get(this.workflowToRun, 'test_stage.tests', []).map(t => { return t.test_name })
            if (test_names.includes(test.test_module_name)) {
              savedTests.push(test)
              savedTestNames.push(test.test_module_name)
            }
          })
          this.testInfos.forEach((info) => {
            if (!savedTestNames.includes(info.test_module_name)) {
              savedTests.push(info)
            }
          })
          item.workflow_args.tests = this.workflowToRun.test_stage.enabled ? savedTests : []
        })
      }).catch((err) => {
        console.log('ERROR:  ', err)
      })
    },
    deleteTestBuildData () {
      const repoKeysToDelete = [
        '_id_', 'branchNames', 'branchPRsMap', 'tags', 'isGithub', 'prNumberPropName', 'id',
        'releaseMethod', 'showBranch', 'showTag', 'showSwitch', 'showPR', 'pr', 'branch'
      ]
      this.testInfos.forEach((test) => {
        test.builds && test.builds.forEach(build => {
          for (const key of repoKeysToDelete) {
            delete build[key]
          }
        })
      })
    },
    addTimerBtn () {
      this.$refs.timer.addTimerBtn()
      this.currentForcedUserInput = {}
    },
    addTimerSchedule () {
      const pipelineConfigValue = this.$refs.pipelineConfig.submit()
      if (pipelineConfigValue) {
        this.$refs.timer.addSchedule(pipelineConfigValue, 'workflow_args')
        this.currentForcedUserInput = {}
        this.deleteTestBuildData()
      }
    },
    closeWebhookDialog () {
      this.firstShowPolicy = false
      this.showEnvUpdatePolicy = false
    },
    editWebhook (index) {
      this.webhookEditMode = true
      this.showEnvUpdatePolicy = true
      this.currenteditWebhookIndex = index
      const webhookSwap = this.$utils.cloneObj(this.webhook.items[index])
      this.getBranchInfoById(webhookSwap.main_repo.codehost_id, webhookSwap.main_repo.repo_owner, webhookSwap.main_repo.repo_name)
      this.webhookSwap = {
        repo: Object.assign({ key: `${webhookSwap.main_repo.repo_owner}/${webhookSwap.main_repo.repo_name}` }, webhookSwap.main_repo),
        namespace: webhookSwap.workflow_args.namespace.split(','),
        env_update_policy: webhookSwap.workflow_args.env_update_policy ? webhookSwap.workflow_args.env_update_policy : (webhookSwap.workflow_args.base_namespace ? 'base' : 'all'),
        base_namespace: webhookSwap.workflow_args.base_namespace,
        env_recycle_policy: webhookSwap.workflow_args.env_recycle_policy,
        events: webhookSwap.main_repo.events,
        auto_cancel: webhookSwap.auto_cancel,
        check_patch_set_change: webhookSwap.check_patch_set_change,
        targets: webhookSwap.workflow_args.targets.map(element => {
          element.key = element.name + '/' + element.service_name
          return element
        }),
        match_folders: webhookSwap.main_repo.match_folders.join('\n')
      }
    },
    addWebhookBtn () {
      this.webhookAddMode = true
      this.webhookSwap = {
        repo: {},
        events: [],
        targets: [],
        namespace: [],
        env_update_policy: 'all',
        auto_cancel: false,
        check_patch_set_change: false,
        base_namespace: '',
        env_recycle_policy: 'success',
        match_folders: '/\n!.md'
      }
    },
    addWebhook () {
      const webhookSwap = this.$utils.cloneObj(this.webhookSwap)
      webhookSwap.repo.match_folders = webhookSwap.match_folders.split('\n')
      webhookSwap.repo.events = webhookSwap.events
      this.webhook.items.push({
        main_repo: webhookSwap.repo,
        auto_cancel: webhookSwap.auto_cancel,
        check_patch_set_change: webhookSwap.check_patch_set_change,
        workflow_args: {
          namespace: webhookSwap.namespace.toString(),
          base_namespace: webhookSwap.env_update_policy === 'base' ? webhookSwap.namespace.toString() : '',
          env_update_policy: webhookSwap.env_update_policy,
          env_recycle_policy: webhookSwap.env_recycle_policy,
          targets: webhookSwap.targets
        }
      })
      this.webhookSwap = {
        repo: {},
        events: [],
        targets: [],
        namespace: '',
        env_update_policy: 'all',
        auto_cancel: false,
        check_patch_set_change: false,
        base_namespace: '',
        env_recycle_policy: 'success',
        match_folders: '/\n!.md'
      }
      this.webhookAddMode = false
    },
    saveWebhook () {
      const index = this.currenteditWebhookIndex
      const webhookSwap = this.$utils.cloneObj(this.webhookSwap)
      webhookSwap.repo.match_folders = webhookSwap.match_folders.split('\n')
      webhookSwap.repo.events = webhookSwap.events
      this.$set(this.webhook.items, index, {
        main_repo: webhookSwap.repo,
        auto_cancel: webhookSwap.auto_cancel,
        check_patch_set_change: webhookSwap.check_patch_set_change,
        workflow_args: {
          namespace: webhookSwap.namespace.toString(),
          base_namespace: webhookSwap.env_update_policy === 'base' ? webhookSwap.namespace.toString() : '',
          env_update_policy: webhookSwap.env_update_policy,
          env_recycle_policy: webhookSwap.env_recycle_policy,
          targets: webhookSwap.targets
        }
      })
      this.webhookSwap = {
        repo: {},
        events: [],
        targets: [],
        namespace: '',
        env_update_policy: 'all',
        auto_cancel: false,
        check_patch_set_change: false,
        base_namespace: '',
        env_recycle_policy: 'success',
        match_folders: '/\n!.md'
      }
      this.webhookEditMode = false
    },
    deleteWebhook (index) {
      this.webhook.items.splice(index, 1)
    },
    changeNamespace () {
      this.webhookSwap.base_namespace = ''
      if (!this.firstShowPolicy && this.webhookSwap.namespace.length === 2) {
        this.showEnvUpdatePolicy = true
        this.firstShowPolicy = true
      }
      if (this.webhookSwap.namespace.length >= 2) {
        this.webhookSwap.env_update_policy = 'single'
      }
    },
    getProducts () {
      const projectName = this.productTmlName
      listProductAPI('test', projectName).then(res => {
        this.products = res
      })
    },
    getBranchInfoById (id, repo_owner, repo_name) {
      getBranchInfoByIdAPI(id, repo_owner, repo_name).then((res) => {
        this.$set(this.webhookBranches, repo_name, res)
      })
    },
    repoChange (currentRepo) {
      this.webhookSwap.events = []
      this.getBranchInfoById(currentRepo.codehost_id, currentRepo.repo_owner, currentRepo.repo_name)
    }
  },
  computed: {
    ...mapGetters(['signupStatus']),
    showPrEnv: {
      get: function () {
        if (this.signupStatus && this.signupStatus.features && this.signupStatus.features.length > 0) {
          if (this.signupStatus.features.includes('pr_create_env')) {
            return true
          } else {
            return false
          }
        } else {
          return false
        }
      }
    },
    showWebhookDialog: {
      get: function () {
        return this.webhookAddMode ? this.webhookAddMode : this.webhookEditMode
      },
      set: function (newValue) {
        this.webhookAddMode ? this.webhookAddMode = newValue : this.webhookEditMode = newValue
      }
    },
    webhookRepos: {
      get: function () {
        let repos = []
        this.presets.forEach(element => {
          repos = repos.concat(element.repos)
        })
        repos = uniqBy(repos, value => value.repo_owner + '/' + value.repo_name)
        repos.forEach(repo => {
          repo.key = `${repo.repo_owner}/${repo.repo_name}`
        })
        return repos
      }
    },
    webhookTargets: {
      get: function () {
        const targets = []
        this.presets.forEach(element => {
          targets.push({
            name: element.target.service_module,
            service_name: element.target.service_name,
            key: element.target.service_module + '/' + element.target.service_name
          })
        })
        return targets
      }
    },
    matchedProducts () {
      return this.products.filter(p => p.product_name === this.productTmlName)
    }
  },
  watch: {
    'workflowToRun.test_stage.tests' (newVal) {
      if (this.workflowToRun.test_stage.enabled) {
        const test_names = newVal.map(t => { return t.test_name })
        this.getTestInfos(test_names)
      }
    },
    'workflowToRun.test_stage.enabled' (newVal) {
      this.getTestReposForSchedules()
    },
    'workflowToRun.schedules.enabled' (newVal) {
      if (newVal && !this.gotScheduleRepo) {
        this.getTestReposForSchedules()
        this.gotScheduleRepo = true
      }
    }
  },
  created () {
    bus.$on('check-tab:trigger', () => {
      bus.$emit('receive-tab-check:trigger', true)
    })
    this.getProducts()
  },
  beforeDestroy () {
    bus.$off('check-tab:trigger')
  },
  components: {
    workflowArgs
  }
}
</script>

<style lang="less">
.add-trigger-dialog {
  .el-form {
    .el-form-item {
      margin-bottom: 8px;
    }

    .env-open-button {
      padding: 7px 7px 7px 10px;
      color: #409eff;
      border-color: #409eff;
    }

    .env-update-list {
      .el-radio-group {
        margin-top: -1rem;
        margin-left: 80px;
        padding: 5px 0;

        .el-radio {
          display: block;
          line-height: 2;
        }
      }
    }
  }
}

.trigger {
  .box-card {
    .add-border {
      padding: 10px;
      border: 1px solid #ebeef5;
      box-shadow: 0 0 5px #ebeef5;
    }

    .webhook-container {
      margin-top: 20px;
    }

    .underline {
      height: 1px;
      margin: 10px 0;
      background: #ccc;
    }

    .pipeline-header {
      height: 40px;
      font-weight: 500;
      font-size: 15px;
      line-height: 2;
    }

    .timer-dialog-footer {
      margin-top: 25px;
    }

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
      width: 100%;
      height: 1px;
      margin: 13px 0;
      background-color: #dfe0e6;
    }

    .help-link {
      color: #1989fa;
    }

    .script {
      padding: 5px 0;

      .title {
        display: inline-block;
        width: 100px;
        padding-top: 6px;
        color: #606266;
        font-size: 14px;
      }

      .item-title {
        margin-left: 5px;
        color: #909399;
      }
    }
  }
}
</style>
