<template>
  <el-form class="run-workflow"
           label-width="80px">
    <el-form-item prop="productName"
                  label="集成环境">
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
          <router-link style="color:#909399"
                       :to="`/v1/projects/detail/${targetProduct}/envs/create`">
            {{`(环境不存在或者没有权限，点击创建环境)`}}
          </router-link>
        </el-option>
      </el-select>
      <el-tooltip v-if="specificEnv"
                  effect="dark"
                  content="该工作流已指定环境运行，可通过修改 工作流->基本信息 来解除指定环境绑定"
                  placement="top">
        <span><i style="color:#909399"
             class="el-icon-question"></i></span>
      </el-tooltip>

    </el-form-item>

    <div v-if="buildDeployEnabled"
         v-loading="precreateLoading">
      <el-form-item v-if="fastSelect"
                    label="构建">
        <el-select v-model="pickedBuildTarget"
                   filterable
                   clearable
                   multiple
                   value-key="name"
                   @change="pickBuildConfig"
                   size="medium"
                   class="full-width">
          <el-option v-for="(build,index) of buildTargets"
                     :key="index"
                     :label="build.name"
                     :value="build">
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="服务">
        <el-select v-model="pickedTargetNames"
                   filterable
                   multiple
                   clearable
                   value-key="key"
                   size="medium"
                   class="full-width">
          <el-option v-for="(ser,index) of allServiceNames"
                     :key="index"
                     :disabled="!ser.has_build"
                     :label="ser.name"
                     :value="ser">
            <span v-if="!ser.has_build">
              <router-link style="color:#ccc"
                           :to="`/v1/projects/detail/${runner.product_tmpl_name}/builds`">
                {{`${ser.name}(${ser.service_name}) (服务不存在构建，点击添加构建)`}}
              </router-link>
            </span>
            <span v-else>
              <span>{{ser.name}}</span><span style="color:#ccc"> ({{ser.service_name}})</span>
            </span>
          </el-option>
        </el-select>
        <template v-if="!fastSelect">
          <el-button size="small"
                     @click="fastSelect=!fastSelect"
                     type="text">快捷选服务
          </el-button>
          <el-tooltip effect="dark"
                      content="通过指定构建配置间接选择出需要的服务"
                      placement="top">
            <span><i style="color:#909399"
                 class="el-icon-question"></i></span>
          </el-tooltip>
        </template>

      </el-form-item>
      <!-- Build -->
      <div v-if="pickedTargets.length > 0">
        <workflow-build-rows :pickedTargets="pickedTargets"></workflow-build-rows>
      </div>
    </div>

    <div v-if="artifactDeployEnabled"
         v-loading="precreateLoading">
      <el-form-item label="服务">
        <el-select v-model="pickedTargetNames"
                   value-key="key"
                   filterable
                   multiple
                   clearable
                   size="medium"
                   class="full-width">
          <el-option v-for="(ser,index) of allServiceNames"
                     :key="index"
                     :label="ser.name"
                     :value="ser">
            <span>
              <span>{{ser.name}}</span><span style="color:#ccc"> ({{ser.service_name}})</span>
            </span>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="镜像仓库">
        <el-select v-model="pickedRegistry"
                   filterable
                   clearable
                   @change="changeRegistry"
                   size="medium"
                   class="full-width">
          <el-option v-for="(reg,index) of allRegistry"
                     :key="index"
                     :label="`${reg.reg_addr}/${reg.namespace}`"
                     :value="reg.id">
          </el-option>
        </el-select>
      </el-form-item>
      <el-table v-if="pickedTargets.length > 0"
                :data="pickedTargets"
                empty-text="无"
                class="service-deploy-table">
        <el-table-column prop="name"
                         label="服务"
                         width="150px">
        </el-table-column>
        <el-table-column label="镜像">
          <template slot-scope="scope">
            <div class="workflow-build-rows">
              <el-row class="build-row">
                <template>
                  <el-col :span="12">
                    <el-select v-if="imageMap[scope.row.name] && imageMap[scope.row.name].length > 0"
                               v-model="scope.row.image"
                               @visible-change="changeVirtualData($event, scope.row, scope.$index)"
                               filterable
                               clearable
                               size="small"
                               style="width:100%"
                               placeholder="请选择镜像">
                      <virtual-scroll-list :ref="`scrollListRef${scope.$index}`"
                                           style="height: 272px;overflow-y:auto;"
                                           :size="virtualData.size"
                                           :keeps="virtualData.keeps"
                                           :start="startAll[scope.row.name]"
                                           :dataKey="(img)=>{ return img.name+'-'+img.tag}"
                                           :dataSources="imageMap[scope.row.name]"
                                           :dataComponent="itemComponent">
                      </virtual-scroll-list>
                    </el-select>
                    <el-tooltip v-else
                                content="请求镜像失败，请手动输入镜像"
                                placement="top"
                                popper-class="gray-popper">
                      <el-input v-model="scope.row.image"
                                class="short-input"
                                size="small"
                                placeholder="请填写镜像"></el-input>
                    </el-tooltip>
                  </el-col>
                </template>
              </el-row>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="buildDeployEnabled"
         class="advanced-setting">
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
                 type="primary"
                 size="small">
        {{ startTaskLoading?'启动中':'启动任务' }}
      </el-button>

    </div>

  </el-form>
</template>

<script>
import _ from 'lodash';
import virtualListItem from './virtual_list_item'
import workflowBuildRows from '@/components/common/workflow_build_rows.vue';
import deployIcons from '@/components/common/deploy_icons';
import { listProductAPI, precreateWorkflowTaskAPI, getAllBranchInfoAPI, runWorkflowAPI, getBuildTargetsAPI, getRegistryWhenBuildAPI, imagesAPI } from '@api';
import virtualScrollList from 'vue-virtual-scroll-list';
export default {
  data() {
    return {
      itemComponent: virtualListItem,
      allRegistry: [],
      activeNames: [],
      buildTargets: [],
      pickedBuildTarget: [],
      imageMap: [],
      pickedRegistry: '',
      specificEnv: true,
      runner: {
        workflow_name: '',
        product_tmpl_name: '',
        description: '',
        namespace: '',
        targets: [],
        tests: []
      },
      products: [],
      matchedProducts: [],
      repoInfoMap: {},
      precreateLoading: false,
      startTaskLoading: false,
      fastSelect: false,
      createVersion: false,
      versionInfo: {
        version: "",
        enabled: true,
        desc: "",
        labels: [],
        labelStr: ""
      },
      versionRules: {
        version: [
          { required: true, message: '请填写版本名称', trigger: ['blur', 'change'] }
        ]
      },
      virtualData: {
        keeps: 20,
        size: 34,
        start: 0
      },
      startAll: {},
    };
  },
  computed: {
    artifactDeployEnabled() {
      return (this.workflowMeta.artifact_stage && this.workflowMeta.artifact_stage.enabled) ? true : false;
    },
    buildDeployEnabled() {
      return this.workflowMeta.build_stage.enabled;
    },
    distributeEnabled() {
      return this.runner.distribute_enabled ? true : false;
    },
    allServiceNames() {
      let allNames = [];
      if (this.buildDeployEnabled) {
        allNames = _.sortBy(this.runner.targets.map(element => {
          element.key = element.name + '/' + element.service_name
          return element
        }), 'service_name');
      }
      else if (this.artifactDeployEnabled) {
        const k8sServices = this.runner.targets.filter(element => {
          for (let index = 0; index < element.deploy.length; index++) {
            const deployItem = element.deploy[index];
            if (deployItem.type !== 'pm') {
              return element;
            }
          }
        });
        allNames = _.sortBy(k8sServices.map(element => {
          element.key = element.name + '/' + element.service_name
          return element
        }), 'service_name');
      }
      this.getServiceImgs(allNames.filter(service => service.has_build).map(service => service.name));
      return allNames;
    },
    targetsMap() {
      return _.keyBy(this.runner.targets, (i) => {
        return i.service_name + '/' + i.name;
      });
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
        return this.runner.targets.filter(t => t.picked);
      },
      set(val) {
        for (const obj of this.runner.targets) {
          obj.picked = false;
        }
        for (const { service_name, name } of val) {
          if (this.targetsMap[`${service_name}/${name}`]) {
            this.targetsMap[`${service_name}/${name}`].picked = true;
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
      if (this.buildDeployEnabled) {
        return this.buildRepos.concat(this.testRepos);
      } else {
        return this.testRepos;
      }
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
    }
  },
  methods: {
    changeVirtualData(event, row, index) {
      let opt = event ? 0 : -1;
      let id = this.imageMap[row.name].map(img => img.full).indexOf(row.image) + opt;
      if (this.startAll[row.name]) {
        this.startAll[row.name] = id;
      } else {
        this.$set(this.startAll, row.name, id);
      }
      this.$refs[`scrollListRef${index}`].virtual.updateRange(id, id + this.virtualData.keeps);
    },
    changeRegistry(val) {
      if (val) {
        this.imageMap = [];
        let allClickableServeiceNames = this.allServiceNames.filter(service => service.has_build).map(service => service.name);
        imagesAPI(_.uniq(allClickableServeiceNames), val).then((images) => {
          images = images || [];
          for (const image of images) {
            image.full = `${image.name}:${image.tag}`;
          }
          this.imageMap = this.$utils.makeMapOfArray(images, 'name');
          this.pickedTargets.forEach(tar => {
            tar.image = '';
            this.startAll[tar.name] = 0;
          })
        })
      }
    },
    getServiceImgs(val) {
      return new Promise((resolve, reject) => {
        const registryId = this.pickedRegistry;
        imagesAPI(_.uniq(val), registryId).then((images) => {
          images = images || [];
          for (const image of images) {
            image.full = `${image.name}:${image.tag}`;
          }
          this.imageMap = this.$utils.makeMapOfArray(images, 'name');
          resolve();
        })
      })
    },
    pickBuildConfig() {
      let allBuildTargets = [];
      this.pickedBuildTarget.forEach(t => {
        allBuildTargets = allBuildTargets.concat(t.targets);
      })
      this.pickedTargetNames = this.allServiceNames.filter(t => {
        const index = allBuildTargets.findIndex(i => {
          return i.service_name === t.service_name && i.service_module === t.name;
        });
        if (index >= 0) {
          return t;
        }
      });
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
          const maybeNew = res.targets[i];
          maybeNew.picked = this.haveForcedInput && (`${maybeNew.service_name}/${maybeNew.name}` in this.forcedInputTargetMap);
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
      const repoKeysToDelete = [
        '_id_', 'branchNames', 'branchPRsMap', 'tags', 'isGithub', 'prNumberPropName', 'id',
        'releaseMethod', 'showBranch', 'showTag', 'showSwitch', 'showPR'
      ];
      const clone = this.$utils.cloneObj(this.runner);
      // filter picked targets
      clone.targets = clone.targets.filter(t => t.picked);
      if (this.artifactDeployEnabled) {
        clone.registry_id = this.pickedRegistry;
        clone.artifact_args = [];
        clone.targets.forEach(element => {
          clone.artifact_args.push({
            service_name: element.service_name,
            name: element.name,
            image: element.image,
            deploy: element.deploy
          });
        });
        delete clone.targets;
        if (this.createVersion) {
          if (this.versionInfo.labelStr !== '') {
            this.versionInfo.labels = this.versionInfo.labelStr.trim().split(';');
          }
          clone.version_args = this.$utils.cloneObj(this.versionInfo);
        }
      }
      else {
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
      }

      runWorkflowAPI(clone, this.artifactDeployEnabled).then(res => {
        const projectName = this.targetProduct;
        const taskId = res.task_id;
        const orgId = this.currentOrganizationId;
        const pipelineName = res.pipeline_name;
        this.$message.success('创建成功');
        this.$emit('success');
        this.$router.push(`/v1/projects/detail/${projectName}/pipelines/multi/${pipelineName}/${taskId}?status=running`);
        this.$store.dispatch('refreshWorkflowList');
      }).catch(error => {
        console.log(error);
        // handle error
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
      if (!this.runner.product_tmpl_name || !this.runner.namespace) {
        this.$message.error('请选择集成环境');
        return false;
      }
      let invalidRepo = [];
      let emptyValue = [];
      let invalidService = [];
      this.pickedTargets.forEach(item => {
        if (item.image === '') {
          invalidService.push(item.name);
        }
      });
      if (this.artifactDeployEnabled) {
        if (this.pickedTargets.length === 0) {
          this.$message({
            message: '请选择需要部署的服务',
            type: 'error'
          });
          return false;
        }
        else {
          if (this.pickedTargets.length > 0 && this.pickedRegistry === '') {
            this.$message({
              message: '请选择镜像仓库',
              type: 'error'
            });
            return false;
          }
          else if (invalidService.length > 0) {
            this.$message({
              message: invalidService.join(',') + ' 服务尚未选择镜像',
              type: 'error'
            });
            return false;
          }
          else if (this.createVersion && this.versionInfo.version === '') {
            this.$message({
              message: '请填写版本名称',
              type: 'error'
            });
            return false;
          }
          else {
            return true;
          }
        }

      }
      else {
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

    }
  },
  created() {
    const projectName = this.workflowMeta.product_tmpl_name;
    this.artifactDeployEnabled && getRegistryWhenBuildAPI().then((res) => {
      this.allRegistry = res;
      if (this.forcedUserInput.registryId) {
        this.pickedRegistry = this.forcedUserInput.registryId;
        this.getServiceImgs(this.forcedUserInput.artifactArgs.map(r => r.name)).then(() => {
          this.forcedUserInput.artifactArgs.forEach((art, index) => {
            this.changeVirtualData(false, art, index);
          })
        })
        return;
      }
      if (res && res.length > 0) {
        for (let i = 0; i < res.length; i++) {
          if (res[i].is_default) {
            this.pickedRegistry = res[i].id;
            break
          };
        }
      }
    })
    if (this.forcedUserInput && this.forcedUserInput.registryId) {
      if (this.forcedUserInput.versionArgs && this.forcedUserInput.versionArgs.version) {
        this.createVersion = true;
        Object.assign(this.versionInfo, this.forcedUserInput.versionArgs);
        this.versionInfo.labelStr = this.versionInfo.labels.join(';');
      }
    } else {
      this.buildDeployEnabled && getBuildTargetsAPI(projectName).then(res => {
        this.buildTargets = res;
      });
    }
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
      } else {
        this.specificEnv = false;
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
  components: {
    workflowBuildRows,
    deployIcons,
    virtualScrollList
  },
};
</script>

<style lang="less">
.run-workflow {
  .service-deploy-table {
    width: auto;
  }
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
  .service-deploy-table {
    margin-bottom: 15px;
    padding: 0px 5px;
  }
  .create-version {
    .el-input,
    .el-textarea,
    .el-select {
      &.full-width {
        width: 40%;
      }
    }
    .create-checkbox {
      margin-bottom: 15px;
    }
  }
  .start-task {
    margin-left: 10px;
    margin-bottom: 10px;
  }
}
</style>
