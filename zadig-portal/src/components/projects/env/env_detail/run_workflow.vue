<template>
  <el-form class="run-workflow"
           label-width="80px">
    <el-form-item prop="pipelines"
                  label="工作流">
      <el-select v-model="workflowName"
                 size="small"
                 class="full-width"
                 @change="getWorkflow"
                 placeholder="请选择">
        <el-option v-for="(item,index) in workflows"
                   :key="index"
                   :label="item.name"
                   :value="item.name">
        </el-option>
      </el-select>
    </el-form-item>
    <template>
      <div v-if="workflowType==='workflow'">
        <div v-if="workflowMeta.build_stage && workflowMeta.build_stage.enabled"
             v-loading="precreateLoading">
          <div v-if="pickedTargets.length > 0">
            <workflow-build-rows :pickedTargets="pickedTargets"></workflow-build-rows>
          </div>
        </div>
        <div class="advanced-setting">
          <el-collapse v-model="activeNames">
            <el-collapse-item title="高级设置"
                              name="1">
              <el-checkbox v-model="runner.reset_cache">不使用工作空间缓存
                <el-tooltip effect="dark"
                            content="可能会增加任务时长。如果构建中不使用工作空间缓存，该设置会被忽略"
                            placement="top">
                  <span><i style="color:#909399"
                       class="el-icon-question"></i></span>
                </el-tooltip>
              </el-checkbox>
              <br>
              <el-checkbox v-model="runner.ignore_cache">不使用 Docker 缓存
                <el-tooltip effect="dark"
                            content="只对配置了镜像构建步骤的构建生效"
                            placement="top">
                  <span><i style="color:#909399"
                       class="el-icon-question"></i></span>
                </el-tooltip>
              </el-checkbox>
            </el-collapse-item>
          </el-collapse>
        </div>
        <div class="start-task">
          <el-button @click="submit"
                     :loading="startTaskLoading"
                     :disabled="pickedTargets.length === 0"
                     type="primary"
                     size="small"
                     plain>
            {{ startTaskLoading?'启动中':'启动任务' }}
          </el-button>
          <span style="color:#ff1949" v-if="pickedTargets.length === 0">该服务尚未配置构建，无法启动任务</span>
        </div>
      </div>
    </template>

  </el-form>
</template>

<script>
import _ from 'lodash';
import workflowBuildRows from '@/components/common/workflow_build_rows.vue';
import deployIcons from '@/components/common/deploy_icons';
import { precreateWorkflowTaskAPI, getAllBranchInfoAPI, runWorkflowAPI, workflowAPI } from '@api';

export default {
  data() {
    return {
      activeNames: [],
      pickedBuildTarget: [],
      specificEnv: false,
      runner: {
        workflow_name: '',
        product_tmpl_name: '',
        description: '',
        namespace: '',
        targets: [],
        tests: [],
      },
      workflowName: '',
      targetProduct: '',
      products: [],
      matchedProducts: [],
      repoInfoMap: {},
      workflowMeta: {},
      precreateLoading: false,
      startTaskLoading: false
    };
  },
  computed: {
    selectedDeployEnv() {
      return `${this.currentServiceMeta.ns}`;
    },
    workflowType() {
      if (this.workflowName) {
        const currentWorkflow = this.workflows.find(element => element.name === this.workflowName);
        return currentWorkflow.workflow_type;
      }
    },
    allServiceNames() {
      return _.sortBy(this.runner.targets.map(t => t), 'name');
    },
    targetsMap() {
      return this.$utils.arrayToMap(this.runner.targets, 'name');
    },
    pickedTargets: {
      get() {
        return this.runner.targets.filter(t => t.picked);
      },
      set(val) {
      }
    },
    pickedTargetNames: {
      get() {
        return this.runner.targets.filter(t => t.picked).map(t => t.name);
      },
      set(val) {
        for (const obj of this.runner.targets) {
          obj.picked = false;
        }
        for (const name of val) {
          if (this.targetsMap[name]) {
            this.targetsMap[name].picked = true;
          }

        }
      }
    },
    buildRepos() {
      return this.$utils.flattenArray(
        this.runner.targets.map(tar => tar.build.repos)
      );
    },
    testRepos() {
      return this.$utils.flattenArray(
        this.runner.tests.map(t => t.builds)
      );
    },
    allRepos() {
      return this.buildRepos.concat(this.testRepos);
    },
    allReposDeduped() {
      return this.$utils.deduplicateArray(
        this.allRepos,
        re => `${re.repo_owner}/${re.repo_name}`,
      );
    },
    allReposForQuery() {
      return this.allReposDeduped.map(re => ({
        repo_owner: re.repo_owner,
        repo: re.repo_name,
        default_branch: re.branch,
        codehost_id: re.codehost_id
      }));
    },

    haveForcedInput() {
      return !this.$utils.isEmpty(this.forcedUserInput);
    },
    forcedInputTargetMap() {
      return this.haveForcedInput ? this.$utils.arrayToMap(this.forcedUserInput.targets, 'name') : {};
    },
  },
  watch: {
    allRepos: {
      handler(newVal) {
        for (const repo of newVal) {
          this.$set(
            repo, 'showBranch',
            (this.runner.distribute_enabled && repo.releaseMethod === 'branch') ||
            !this.runner.distribute_enabled
          );
          this.$set(repo, 'showTag', this.runner.distribute_enabled && repo.releaseMethod === 'tag');
          this.$set(repo, 'showSwitch', this.runner.distribute_enabled);
          this.$set(repo, 'showPR', !this.runner.distribute_enabled);
        }
      },
      deep: true
    }
  },
  methods: {
    getWorkflow() {
      if (this.workflowType === 'workflow') {
        workflowAPI(this.workflowName).then(res => {
          this.workflowMeta = res;
          const namespace = this.currentServiceMeta.envName;
          const product = this.workflowMeta.product_tmpl_name;
          this.precreate(`${product} / ${namespace}`);
        });
      }
    },
    servicePipeStart() {
      this.$emit('success');
    },
    async filterProducts() {
      const prodProducts = this.products.filter(element => {
        if (element.product_name === this.targetProduct) {
          if (element.is_prod) {
            return element;
          }
        }
      });
      const testProducts = this.products.filter(element => {
        if (element.product_name === this.targetProduct) {
          if (!element.is_prod) {
            return element;
          }
        }
      });
      this.matchedProducts = prodProducts.concat(testProducts);
    },
    precreate(proNameAndNamespace) {
      const [productName, namespace] = proNameAndNamespace.split(' / ');
      this.precreateLoading = true;
      precreateWorkflowTaskAPI(this.workflowName, namespace).then(res => {
        // prepare targets for view
        for (let i = 0; i < res.targets.length; i++) {
          if (this.haveForcedInput) {
            const old = res.targets[i];
            const forcedObj = this.forcedInputTargetMap[`${old.service_name}/${old.name}`];
            if (forcedObj) {
              res.targets.splice(
                i,
                1,
                Object.assign(this.$utils.cloneObj(forcedObj), { deploy: old.deploy, has_build: old.has_build })
              );
            }
          }
          // 
          const maybeNew = res.targets[i];

          const targetIndex = this.currentServiceMeta.targets.findIndex((t) => {
            return t.service_name === maybeNew.service_name;
          });
          if (targetIndex >= 0) {
            maybeNew.picked = true;
          }
          else {
            maybeNew.picked = false;
          }
        }
        // prepare deploys for view
        for (const tar of res.targets) {
          const forced = this.haveForcedInput ? this.forcedInputTargetMap[`${tar.service_name}/${tar.name}`] : null;
          const depMap = forced ? this.$utils.arrayToMap((forced.deploy || []), this.deployID) : {};
          for (const dep of tar.deploy) {
            this.$set(dep, 'picked', !forced || (this.deployID(dep) in depMap));
          }
        }

        if (this.haveForcedInput) {
          res.product_tmpl_name = this.forcedUserInput.product_tmpl_name;
          res.description = this.forcedUserInput.description;
          res.tests = this.forcedUserInput.tests;
        }

        this.runner = res;
        this.precreateLoading = false;
        getAllBranchInfoAPI({ infos: this.allReposForQuery }, this.distributeEnabled ? 'bt' : 'bp').then(res => {
          // make these repo info more friendly
          res.map(repo => {
            if (repo.prs) {
              repo.prs.forEach(element => {
                element.pr = element.id;
              });
              repo.branchPRsMap = this.$utils.arrayToMapOfArrays(repo.prs, 'targetBranch');
            }
            else {
              repo.branchPRsMap = {}
            }
            if (repo.branches) {
              repo.branchNames = repo.branches.map(b => b.name);
            }
            else {
              repo.branchNames = [];
            }
          });
          // and make a map
          this.repoInfoMap = this.$utils.arrayToMap(res, re => `${re.repo_owner}/${re.repo}`);

          // prepare build/test repo for view
          // see watcher for allRepos
          for (const repo of this.allRepos) {
            this.$set(repo, '_id_', this.repoID(repo));
            const repoInfo = this.repoInfoMap[repo._id_];
            this.$set(repo, 'branchNames', repoInfo && repoInfo.branchNames);
            this.$set(repo, 'branchPRsMap', repoInfo && repoInfo.branchPRsMap);
            this.$set(repo, 'tags', repoInfo && repoInfo.tags || []);
            this.$set(repo, 'prNumberPropName', 'pr');
            this.$set(repo, 'errorMsg', repoInfo['error_msg'] || '');
            if (repo.tag) {
              this.$set(repo, 'releaseMethod', 'tag');
            }
            else {
              this.$set(repo, 'releaseMethod', 'branch');
            }
            // make sure branch/pr/tag is reactive
            this.$set(repo, 'branch', repo.branch || '');
            this.$set(repo, repo.prNumberPropName, repo[repo.prNumberPropName] || null);
            this.$set(repo, 'tag', repo.tag || '');
          }
        }).catch(() => {
          this.precreateLoading = false;
        });
      }).catch(() => {
        this.precreateLoading = false;
      });
    },

    repoID(repo) {
      return `${repo.repo_owner}/${repo.repo_name}`;
    },
    deployID(deploy) {
      return `${deploy.env}|${deploy.type}`;
    },

    submit() {
      if (!this.checkInput()) {
        return;
      }

      this.startTaskLoading = true;

      const clone = this.$utils.cloneObj(this.runner);

      const repoKeysToDelete = [
        '_id_', 'branchNames', 'branchPRsMap', 'tags', 'isGithub', 'prNumberPropName', 'id',
        'releaseMethod', 'showBranch', 'showTag', 'showSwitch', 'showPR'
      ];

      // filter targets
      clone.targets = clone.targets.filter(t => t.picked);

      for (const tar of clone.targets) {
        // trim target
        delete tar.picked;

        // trim build repos
        for (const repo of tar.build.repos) {
          repo['pr'] = repo['pr'] ? repo['pr'] : 0;
          for (const key of repoKeysToDelete) {
            delete repo[key];
          }
        }

        // filter deploys
        tar.deploy = tar.deploy.filter(d => d.picked);
        // trim deploys
        for (const dep of tar.deploy) {
          delete dep.picked;
        }
      }

      // trim test repos
      for (const test of clone.tests) {
        for (const repo of test.builds) {
          repo['pr'] = repo['pr'] ? repo['pr'] : 0;
          for (const key of repoKeysToDelete) {
            delete repo[key];
          }
        }
      }
      runWorkflowAPI(clone).then(res => {
        const projectName = this.workflowMeta.product_tmpl_name;
        this.$message.success('创建成功');
        this.$emit('success');
        this.$router.push(`/v1/projects/detail/${projectName}/pipelines/multi/${res.pipeline_name}/${res.task_id}?status=running`);
        this.$store.dispatch('refreshWorkflowList');
      }).catch(error => {
        if (error.response && error.response.data.code === 6168) {
          const projectName = error.response.data.extra.productName;
          const envName = error.response.data.extra.envName;
          const serviceName = error.response.data.extra.serviceName;
          this.$message({
            message: `检测到 ${projectName} 中 ${envName} 环境下的 ${serviceName} 服务未启动 <br> 请检查后再运行工作流`,
            type: 'warning',
            dangerouslyUseHTMLString: true,
            duration: 5000
          });
          this.$router.push(`/v1/projects/detail/${projectName}/envs/detail/${serviceName}?envName=${envName}&projectName=${projectName}`);
        }
      }).finally(() => {
        this.startTaskLoading = false;
      });
    },
    checkInput() {
      if (!this.runner.product_tmpl_name || !this.runner.namespace || !this.workflowName) {
        this.$message.error('请选择工作流');
        return false;
      }

      let invalidRepo = [];
      let emptyValue = [];

      this.allRepos.forEach(item => {
        if (item.use_default || !item.repo_name) {
          return;
        }

        if (this.showTag && item.tags.length === 0) {
          invalidRepo.push(item.repo_name);
        }

        let filled = false;
        if (item.showBranch && item.branch) {
          filled = true;
        }
        if (item.showTag && item.tag) {
          filled = true;
        }
        if (item.showPR && item[item.prNumberPropName]) {
          filled = true;
        }
        if (!filled) {
          emptyValue.push(item.repo_name);
        }
      });

      if (invalidRepo.length === 0 && emptyValue.length === 0) {
        return true;
      } else {
        if (invalidRepo.length > 0) {
          this.$message({
            message: invalidRepo.join(',') + ' 代码库不存在 Release Tag,请重新选择构建方式',
            type: 'error'
          });
        } else if (emptyValue.length > 0) {
          this.$message({
            message: emptyValue.join(',') + ' 代码库尚未选择构建信息',
            type: 'error'
          });
        }
        return false;
      }
    }
  },
  mounted() {
    if (this.workflows && this.workflows.length > 0) {
      this.workflowName = this.workflows[0].name;
      if (this.workflowType === 'workflow') {
        workflowAPI(this.workflowName).then(res => {
          this.workflowMeta = res;
          const namespace = this.currentServiceMeta.envName;
          const product = this.workflowMeta.product_tmpl_name;
          this.precreate(`${product} / ${namespace}`);
        });
      }
    }
  },
  props: {
    forcedUserInput: {
      type: Object,
      default() {
        return {};
      }
    },
    workflows: {
      required: true,
      type: Array,
      default: []
    },
    currentServiceMeta: {
      type: Object,
      required: true,
      default: null
    },
  },
  components: {
    workflowBuildRows,
    deployIcons,
  },
};
</script>

<style lang="less">
.run-workflow {
  .advanced-setting {
    padding: 0px 0px;
    margin-bottom: 10px;
  }
  .el-input,
  .el-select {
    &.full-width {
      width: 40%;
    }
  }
  .start-task {
    margin-left: 10px;
    margin-bottom: 10px;
  }
}
</style>
