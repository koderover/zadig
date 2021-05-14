<template>
  <el-form class="mobile-run-workflow"
           label-position="top">
    <el-form-item prop="productName">
      <slot name="label">
        <span>集成环境</span>
        <el-tooltip v-if="specificEnv"
                    effect="dark"
                    content="该工作流已指定环境运行，可通过修改 工作流->基本信息 来解除指定环境绑定"
                    placement="top">
          <span><i style="color:#909399"
               class="el-icon-question"></i></span>
        </el-tooltip>
      </slot>

      <el-select :value="runner.product_tmpl_name&&runner.namespace ? `${runner.product_tmpl_name} / ${runner.namespace}` : ''"
                 @change="precreate"
                 size="medium"
                 :disabled="specificEnv"
                 class="full-width">
        <el-option v-for="pro of matchedProducts"
                   :key="`${pro.product_name} / ${pro.env_name}`"
                   :label="`${pro.product_name} / ${pro.env_name}${pro.is_prod?'（生产）':''}`"
                   :value="`${pro.product_name} / ${pro.env_name}`">
          <span>{{`${pro.product_name} / ${pro.env_name}`}}
            <el-tag v-if="pro.is_prod"
                    type="danger"
                    size="mini"
                    effect="light">
              生产
            </el-tag>
          </span>
        </el-option>
        <el-option v-if="matchedProducts.length===0"
                   label=""
                   value="">
          {{`(环境不存在，请前往 PC 端创建环境)`}}
        </el-option>
      </el-select>

    </el-form-item>

    <div v-if="workflowMeta.build_stage.enabled"
         v-loading="precreateLoading">
      <el-form-item label="服务">
        <el-select v-model="pickedTargetNames"
                   filterable
                   multiple
                   clearable
                   size="medium"
                   class="full-width">
          <el-option v-for="(ser,index) of allServiceNames"
                     :key="index"
                     :disabled="!ser.has_build"
                     :label="ser.name"
                     :value="ser.name">
            <span v-if="!ser.has_build">
              {{`${ser.name}`}}
            </span>
          </el-option>
        </el-select>
      </el-form-item>
      <template>
        <van-divider v-if="pickedTargets.length > 0"
                     content-position="left">构建部署</van-divider>
        <div v-for="(target,index) in pickedTargets"
             :key="index">
          <div>
            <span>{{target.name}}</span>
            <span v-for="deploy of target.deploy"
                  :key="`${deploy.env}|${deploy.type}`">
              <el-checkbox v-if="deploy.type === 'k8s'"
                           v-model="deploy.picked"
                           size="small">
                {{ deploy.env }}
              </el-checkbox>
              <template v-else-if="deploy.type === 'pm'">
                {{ deploy.env }}
              </template>
            </span>
          </div>
          <workflow-build-rows :builds="target.build.repos"></workflow-build-rows>
          <van-divider dashed></van-divider>
        </div>
      </template>
    </div>
    <template>
      <van-divider v-if="runner.tests.length >0"
                   content-position="left">测试</van-divider>
      <div v-for="(runner,index) in runner.tests"
           :key="index">
        <div>
          <span>{{runner.test_module_name}}</span>

        </div>
        <van-divider dashed></van-divider>
      </div>
    </template>

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
      <van-button type="info"
                  @click.stop.prevent="startTask"
                  round
                  plain
                  :loading="startTaskLoading"
                  block>启动任务</van-button>
    </div>

  </el-form>
</template>

<script>
import workflowBuildRows from './workflow_build_rows.vue';
import { listProductAPI, precreateWorkflowTaskAPI, getAllBranchInfoAPI, runWorkflowAPI } from '@api';
import { Divider, Button, Notify } from 'vant';
export default {
  components: {
    workflowBuildRows,
    [Divider.name]: Divider,
    [Button.name]: Button,
    [Notify.name]: Notify
  },
  data() {
    return {
      activeNames: [],
      specificEnv: false,
      runner: {
        workflow_name: '',
        product_tmpl_name: '',
        description: '',
        namespace: '',
        targets: [],
        tests: [],
      },

      products: [],
      matchedProducts: [],
      repoInfoMap: {},
      precreateLoading: false,
      startTaskLoading: false,
    };
  },
  computed: {
    allServiceNames() {
      return this.runner.targets.map(t => t);
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
          this.targetsMap[name].picked = true;
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
        for (let i = 0; i < res.targets.length; i++) {
          if (this.haveForcedInput) {
            const old = res.targets[i];
            const forcedObj = this.forcedInputTargetMap[old.name];
            if (forcedObj) {
              res.targets.splice(
                i,
                1,
                Object.assign(this.$utils.cloneObj(forcedObj), { deploy: old.deploy })
              );
            }
          }
          const maybeNew = res.targets[i];
          maybeNew.picked = this.haveForcedInput && (maybeNew.name in this.forcedInputTargetMap);
        }

        // prepare deploys for view
        for (const tar of res.targets) {
          const forced = this.haveForcedInput ? this.forcedInputTargetMap[tar.name] : null;
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
        getAllBranchInfoAPI({ infos: this.allReposForQuery }).then(res => {
          // make these repo info more friendly
          for (const repo of res) {
            repo.prs.forEach(element => {
              element.pr = element.id;
            });
            repo.branchPRsMap = this.$utils.arrayToMapOfArrays(repo.prs, 'targetBranch');
            repo.branchNames = repo.branches.map(b => b.name);
          }
          // and make a map
          this.repoInfoMap = this.$utils.arrayToMap(res, re => `${re.repo_owner}/${re.repo}`);
          for (const repo of this.allRepos) {
            this.$set(repo, '_id_', this.repoID(repo));
            const repoInfo = this.repoInfoMap[repo._id_];
            this.$set(repo, 'branchNames', repoInfo && repoInfo.branchNames);
            this.$set(repo, 'branchPRsMap', repoInfo && repoInfo.branchPRsMap);
            this.$set(repo, 'tags', repoInfo && repoInfo.tags);
            this.$set(repo, 'prNumberPropName', 'pr');
            this.$set(repo, 'releaseMethod', 'branch');

            // make sure branch/pr/tag is reactive
            this.$set(repo, 'branch', repo.branch || '');
            this.$set(repo, repo.prNumberPropName, repo[repo.prNumberPropName] || null);
            this.$set(repo, 'tag', repo.tag || '');
          }

        }).catch(() => {
          this.precreateLoading = false;
        });;
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

    startTask() {
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
        Notify({ type: 'success', message: '任务创建成功' });
        this.$emit('success');
        this.$router.push(`/mobile/pipelines/project/${this.targetProduct}/multi/${res.pipeline_name}/${res.task_id}?status=running`);
        this.$store.dispatch('refreshWorkflowList');
      }).catch(error => {
        if (error.response && error.response.data.code === 6168) {
          const projectName = error.response.data.extra.productName;
          const envName = error.response.data.extra.envName;
          const serviceName = error.response.data.extra.serviceName;
          Notify({ type: 'warning', duration: 5000, message: `检测到 ${projectName} 中 ${envName} 环境下的 ${serviceName} 服务未启动 <br> 请检查后再运行工作流` });
        }
      }).finally(() => {
        this.startTaskLoading = false;
      });
    },
    checkInput() {
      if (!this.runner.product_tmpl_name || !this.runner.namespace) {
        Notify({ type: 'warning', message: '请选择集成环境' });
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
          Notify({ type: 'warning', message: invalidRepo.join(',') + ' 代码库不存在 Release Tag,请重新选择构建方式' });
        } else if (emptyValue.length > 0) {
          Notify({ type: 'warning', message: emptyValue.join(',') + ' 代码库尚未选择构建信息' });
        }
        return false;
      }
    }
  },
  created() {
    const projectName = this.workflowMeta.product_tmpl_name;
    listProductAPI('', projectName).then(res => {
      this.products = res;
      this.filterProducts();
      const product = this.forcedUserInput.product_tmpl_name;
      const namespace = this.forcedUserInput.namespace;
      if (this.workflowMeta.env_name && this.products.find(p => (p.product_name === this.workflowMeta.product_tmpl_name) && (p.env_name === this.workflowMeta.env_name))) {
        const namespace = this.workflowMeta.env_name;
        const product = this.workflowMeta.product_tmpl_name;
        this.specificEnv = true;
        this.precreate(`${product} / ${namespace}`);
      }
      if (this.haveForcedInput && this.products.find(p => p.product_name === product)) {
        this.precreate(`${product} / ${namespace}`);
      }
    });
  },
  props: {
    workflowName: {
      type: String,
      required: true,
    },
    targetProduct: {
      type: String,
      required: true,
    },
    workflowMeta: {
      type: Object,
      required: true
    },
    forcedUserInput: {
      type: Object,
      default() {
        return {};
      }
    },
  },

};
</script>

<style lang="less">
.mobile-run-workflow {
  .service-deploy-table,
  .test-table {
    width: auto;
  }
  .advanced-setting {
    padding: 0px 0px;
    margin-bottom: 10px;
  }
  .el-input,
  .el-select {
    &.full-width {
      width: 100%;
    }
  }
  .service-deploy-table,
  .test-table {
    margin-bottom: 15px;
    padding: 0px 5px;
  }
  .start-task {
    margin-left: 10px;
    margin-bottom: 10px;
  }
  .el-form-item {
    margin-bottom: 5px;
  }
}
</style>
