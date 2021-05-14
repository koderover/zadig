<template>
  <el-form class="workflow-args"
           label-width="80px">
    <el-form-item prop="productName"
                  label="集成环境">
      <el-select :value="runner.product_tmpl_name&&runner.namespace ? `${runner.product_tmpl_name} / ${runner.namespace}` : ''"
                 @change="precreate"
                 size="small"
                 class="full-width">
        <el-option v-for="pro of matchedProducts"
                   :key="`${pro.product_name} / ${pro.env_name}`"
                   :label="`${pro.product_name} / ${pro.env_name}（${pro.is_prod?'生产':'测试'}）`"
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
      </el-select>
    </el-form-item>

    <div v-if="workflowMeta.build_stage.enabled"
         v-loading="precreateLoading">
      <el-form-item label="服务">
        <el-select v-model="pickedTargets"
                   filterable
                   multiple
                   clearable
                   value-key="key"
                   size="small"
                   class="full-width">
          <el-option v-for="(ser,index) of allServiceNames"
                     :key="index"
                     :disabled="!ser.has_build"
                     :label="ser.name"
                     :value="ser">
            <span v-if="!ser.has_build">
              <router-link style="color:#ccc"
                           :to="`/v1/projects/detail/${runner.product_tmpl_name}/builds`">
                {{`${ser.name}(${ser.service_name})(服务不存在构建，点击添加构建)`}}
              </router-link>
            </span>
            <span v-else>
              <span>{{ser.name}}</span><span style="color:#ccc"> ({{ser.service_name}})</span>
            </span>
          </el-option>
        </el-select>
      </el-form-item>

      <div v-if="pickedTargets.length > 0">
        <workflow-build-rows :pickedTargets="pickedTargets"></workflow-build-rows>
      </div>
    </div>

    <div class="start-task"
         v-if="whichSave === 'inside'">
      <el-button @click="submit"
                 type="primary"
                 size="small">
        确定
      </el-button>
    </div>

  </el-form>
</template>

<script>
import _ from 'lodash';
import workflowBuildRows from '@/components/common/workflow_build_rows.vue';
import deployIcons from '@/components/common/deploy_icons';
import { listProductAPI, createWorkflowTaskAPI, getAllBranchInfoAPI } from '@api';

export default {
  data() {
    return {
      runner: {
        workflow_name: '',
        product_tmpl_name: '',
        description: '',
        namespace: '',
        targets: [],
        tests: [],
      },
      pickedTargets: [],
      products: [],
      repoInfoMap: {},
      precreateLoading: false,
    };
  },
  computed: {
    distributeEnabled() {
      return this.workflowMeta.distribute_stage.enabled;
    },
    allServiceNames() {
      return this.runner.targets.map(element => {
        element.key = element.name + '/' + element.service_name
        return element
      });
    },
    allRepos() {
      let buildRepos = this.runner.targets.length > 0 ? this.$utils.flattenArray(this.runner.targets.map(tar => tar.build.repos)) : [];
      let testRepos = this.runner.tests.length > 0 ? this.$utils.flattenArray(this.runner.tests.map(t => t.builds)) : [];
      return buildRepos.concat(testRepos);
    },
    allReposForQuery() {
      let allReposDeduped = this.$utils.deduplicateArray(
        this.allRepos,
        re => `${re.repo_owner}/${re.repo_name}`,
      ) || [];  
      return allReposDeduped.map(re => ({
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
      if (this.haveForcedInput) {
        if (this.artifactDeployEnabled) {
          return _.keyBy(this.forcedUserInput.artifactArgs, (i) => {
            return i.service_name + '/' + i.name;
          });
        }
        return _.keyBy(this.forcedUserInput.targets, (i) => {
          return i.service_name + '/' + i.name;
        });
      }
      return {};
    },
    forcedInputTarget() {
      if (this.haveForcedInput) {
        if (this.artifactDeployEnabled) {
          return this.forcedUserInput.artifactArgs;
        }
        return this.forcedUserInput.targets;
      }
      return {};
    },
    matchedProducts() {
      return this.products.filter(p => p.product_name === this.targetProduct);
    },
  },
  watch: {
    allRepos: {
      handler(newVal) {
        for (const repo of newVal) {
          this.$set(
            repo, 'showBranch',
            (this.distributeEnabled && repo.releaseMethod === 'branch') ||
            !this.distributeEnabled
          );
          this.$set(repo, 'showTag', this.distributeEnabled && repo.releaseMethod === 'tag');
          this.$set(repo, 'showSwitch', this.distributeEnabled);
          this.$set(repo, 'showPR', !this.distributeEnabled);
        }
      },
      deep: true
    },
    'runner.namespace'(newVal) {
      this.runner.tests.forEach(info => {
        info.namespace = newVal;
      })
    }
  },
  methods: {
    precreate(proNameAndNamespace) {
      const [productName, namespace] = proNameAndNamespace.split(' / ');
      this.precreateLoading = true;

      var deployID = (deploy) => {
        return `${deploy.env}|${deploy.type}`;
      }
      createWorkflowTaskAPI(this.targetProduct, namespace).then(res => {
        // prepare targets for view
        for (let i = 0; i < res.targets.length; i++) {
          res.targets[i].picked = false;   
          if (this.haveForcedInput) {
            const old = res.targets[i];
            const forcedObj = this.forcedInputTargetMap[`${old.service_name}/${old.name}`];
            if (forcedObj) {
              res.targets.splice(
                i,
                1,
                Object.assign(this.$utils.cloneObj(forcedObj), { deploy: old.deploy }, { picked: true })
              );
            }
          }
        }
        for (const tar of res.targets) {
          const forced = this.haveForcedInput ? this.forcedInputTargetMap[`${tar.service_name}/${tar.name}`] : null;
          const depMap = forced ? this.$utils.arrayToMap((forced.deploy || []), deployID) : {};
          for (const dep of tar.deploy) {
            this.$set(dep, 'picked', !forced || (deployID(dep) in depMap));
          }
        }
        this.runner.workflow_name = this.workflowName;
        this.runner.product_tmpl_name = this.targetProduct;
        this.runner.namespace = namespace;

        if (this.haveForcedInput) {
          this.runner.description = this.forcedUserInput.description;
          this.runner.tests = this.forcedUserInput.tests || [];
          this.$set(this, 'pickedTargets', res.targets.filter(t => { if (t.picked) { return t } }));
        }

        this.runner.targets = res.targets;
        this.precreateLoading = false;
        getAllBranchInfoAPI({ infos: this.allReposForQuery }).then(res => {
          // make these repo info more friendly
          for (const repo of res) {
            if (repo.prs) {
              repo.prs.forEach(element => {
                element.pr = element.id;
              });
              repo.branchPRsMap = this.$utils.arrayToMapOfArrays(repo.prs, 'targetBranch');
            } else {
              repo.branchPRsMap = {};
            }
            repo.branchNames = repo.branches && repo.branches.map(b => b.name) || [];
          }
          // and make a map
          this.repoInfoMap = this.$utils.arrayToMap(res, re => `${re.repo_owner}/${re.repo}`);
          for (const repo of this.allRepos) {
            this.$set(repo, '_id_', `${repo.repo_owner}/${repo.repo_name}`);
            const repoInfo = this.repoInfoMap[repo._id_];
            this.$set(repo, 'branchNames', repoInfo && repoInfo.branchNames);
            this.$set(repo, 'branchPRsMap', repoInfo && repoInfo.branchPRsMap);
            this.$set(repo, 'tags', repoInfo && repoInfo.tags);
            this.$set(repo, 'prNumberPropName', 'pr');
            this.$set(repo, 'releaseMethod', 'branch');
            this.$set(repo, 'errorMsg', repoInfo && repoInfo.error_msg || '');
            // make sure branch/pr/tag is reactive
            this.$set(repo, 'branch', repo.branch || '');
            this.$set(repo, repo.prNumberPropName, repo[repo.prNumberPropName] || undefined);
            this.$set(repo, 'tag', repo.tag || '');
          }
        }).catch((err) => {
          this.precreateLoading = false;
        });
      }).catch((error) => {
        this.precreateLoading = false;
      });
    },

    submit() {
      if (!this.checkInput()) {
        return;
      }
      const clone = this.$utils.cloneObj(this.runner);
      const repoKeysToDelete = [
        '_id_', 'branchNames', 'branchPRsMap', 'tags', 'isGithub', 'prNumberPropName', 'id',
        'releaseMethod', 'showBranch', 'showTag', 'showSwitch', 'showPR'
      ];
      // filter targets
      clone.targets = this.pickedTargets;
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

      if (clone.tests) {
        // trim test repos
        for (const test of clone.tests) {
          for (const repo of test.builds) {
            repo['pr'] = repo['pr'] ? repo['pr'] : 0;
            for (const key of repoKeysToDelete) {
              delete repo[key];
            }
          }
        }
      }

      if (!this.workflowMeta.test_stage.enabled) {
        delete clone.tests;
      }
      this.$emit('success', clone);
      return clone;
    },
    checkInput() {
      if (!this.runner.product_tmpl_name || !this.runner.namespace) {
        this.$message.error('请选择集成环境');
        return false;
      }

      let invalidRepo = [];
      let emptyValue = [];

      this.allRepos.forEach(item => {
        if (item.use_default || !item.repo_name) {
          return;
        }

        if (item.showTag && item.tags.length === 0) {
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
            message: emptyValue.join(',') + ' 代码尚未选择构建信息',
            type: 'error'
          });
        }
        return false;
      }
    }
  },
  created() {
    this.runner.tests = this.testInfos;
    listProductAPI().then(res => {
      this.products = res;
      const product = this.forcedUserInput.product_tmpl_name;
      const namespace = this.forcedUserInput.namespace;
      if (this.haveForcedInput &&
        this.products.find(p => p.product_name === product)) {
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
    testInfos: {
      type: Array,
      default() {
        return [];
      }
    },
    whichSave: {
      typeof: String,
      default: 'inside'  
    }
  },
  components: {
    workflowBuildRows,
    deployIcons
  },
};
</script>

<style lang="less" >
.workflow-args {
  .el-input,
  .el-select {
    &.full-width {
      width: 40%;
    }
    width: 100%;
  }
}
</style>
