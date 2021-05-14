<template>
    <div class="workflow-task-detail workflow-or-pipeline-task-detail">
      <!--start of workspace-tree-dialog-->
      <el-dialog :visible.sync="artifactModalVisible"
                 width="60%"
                 title="文件导出"
                 class="downloadArtifact-dialog">
        <artifact-download ref="downloadArtifact"
                          :workflowName="workflowName"
                          :taskId="taskID"
                          :showArtifact="artifactModalVisible"></artifact-download>
      </el-dialog>
      <!--end of workspace-tree-dialog-->
      <el-row>
        <el-col :span="6">

          <div class="section-head">
            基本信息
          </div>

          <el-form class="basic-info"
                   label-width="100px">
            <el-form-item label="状态">
              <el-tag size="small"
                      effect="dark"
                      :type="$utils.taskElTagType(taskDetail.status)"
                      close-transition>
                {{ myTranslate(taskDetail.status) }}
              </el-tag>
            </el-form-item>
            <el-form-item label="创建者">
              {{ taskDetail.task_creator }}
            </el-form-item>
            <el-form-item v-if="taskDetail.task_revoker"
                          label="取消者">
              {{ taskDetail.task_revoker }}
            </el-form-item>
            <el-form-item label="集成环境">
              {{ workflow.product_tmpl_name }} - {{ workflow.namespace }}
            </el-form-item>
            <el-form-item label="持续时间">
              {{ taskDetail.interval }}
              <el-tooltip v-if="taskDetail.intervalSec<0"
                          content="本地系统时间和服务端可能存在不一致，请同步。"
                          placement="top">
                <i class="el-icon-warning"
                   style="color:red"></i>
              </el-tooltip>
            </el-form-item>
            <el-form-item v-if="showOperation()"
                          label="操作">
                <el-button v-if="taskDetail.status==='failed' || taskDetail.status==='cancelled' || taskDetail.status==='timeout'"
                           @click="rerun"
                           type="text"
                           size="medium">失败重试</el-button>
                <el-button v-if="taskDetail.status==='running'||taskDetail.status==='created'"
                           @click="cancel"
                           type="text"
                           size="medium">取消任务</el-button>
            </el-form-item>
          </el-form>
        </el-col>

        <el-col v-if="buildSummary.length > 0 || jenkinsSummary.length > 0"
                :span="14">
          <div class="section-head">
              构建信息
          </div>
          <div class="build-summary" v-if="buildSummary.length > 0">
            <el-table :data="buildSummary"
                      style="width: 100%">
              <el-table-column label="服务"
                               width="160">
                <template slot-scope="scope">
                  {{scope.row.service_name}}
                </template>
              </el-table-column>
              <el-table-column label="代码">
                <template slot-scope="scope">
                  <div v-if="scope.row.builds.length > 0">
                    <el-row :gutter="0"
                            v-for="(build,index) in scope.row.builds"
                            :key="index">
                      <el-col :span="24">
                        <el-tooltip :content="build.source==='gerrit'?`暂不支持在 gerrit 上查看 Release`:`在 ${build.source} 上查看 Release`"
                                    placement="top"
                                    effect="dark">
                          <span v-if="build.tag"
                                class="link">
                            <a v-if="build.source==='github'||build.source==='gitlab'"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tags/${build.tag}`"
                               target="_blank">{{build.tag}}
                            </a>
                            <span v-if="build.source==='gerrit'">{{build.tag}}</span>
                          </span>
                        </el-tooltip>
                        <el-tooltip :content="build.source==='gerrit'?`暂不支持在 gerrit 上查看 Branch`:`在 ${build.source} 上查看 Branch`"
                                    placement="top"
                                    effect="dark">
                          <span v-if="build.branch && !build.tag"
                                class="link">
                            <a v-if="build.source==='github'||build.source==='gitlab'"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                               target="_blank">{{"Branch-"+build.branch}}
                            </a>
                            <a v-if="!build.source"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/tree/${build.branch}`"
                               target="_blank">{{"Branch-"+build.branch}}
                            </a>
                            <span v-if="build.source==='gerrit'">{{"Branch-"+build.branch}}</span>
                          </span>
                        </el-tooltip>
                        <el-tooltip :content="`在 ${build.source} 上查看 PR`"
                                    placement="top"
                                    effect="dark">
                          <span v-if="build.pr && build.pr>0"
                                class="link">
                            <a v-if="build.source==='github'"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/pull/${build.pr}`"
                               target="_blank">{{"PR-"+build.pr}}
                            </a>
                            <a v-if="build.source==='gitlab'"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/merge_requests/${build.pr}`"
                               target="_blank">{{"PR-"+build.pr}}
                            </a>
                            <a v-if="!build.source"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/pull/${build.pr}`"
                               target="_blank">{{"PR-"+build.pr}}
                            </a>
                          </span>
                        </el-tooltip>
                        <el-tooltip :content="`在 ${build.source} 上查看 Commit`"
                                    placement="top"
                                    effect="dark">
                          <span v-if="build.commit_id"
                                class="link">
                            <a v-if="build.source==='github'||build.source==='gitlab'"
                               :href="`${build.address}/${build.repo_owner}/${build.repo_name}/commit/${build.commit_id}`"
                               target="_blank">{{build.commit_id.substring(0, 8)}}
                            </a>
                            <span
                                  v-else-if="build.source==='gerrit'&& (!build.pr || build.pr===0)">{{build.commit_id.substring(0, 8)}}</span>
                            <span v-else-if="build.source==='gerrit'&& build.pr && build.pr!==0"
                                  class="link">
                              <a :href="`${build.address}/c/${build.repo_name}/+/${build.pr}`"
                                 target="_blank">{{`Change-${build.pr}`}}
                              </a>
                              {{build.commit_id.substring(0, 8)}}
                            </span>
                          </span>
                        </el-tooltip>
                      </el-col>
                    </el-row>
                  </div>
                  <span v-else> 暂无代码 </span>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="build-summary" v-if="jenkinsSummary.length > 0">
            <Etable :tableColumns="jenkinsBuildColumns" :tableData="jenkinsSummary" id="id" />
          </div>
        </el-col>
        <el-col v-if="taskDetail.workflow_args && taskDetail.workflow_args.version_args"
                :span="14">
          <div class="section-head">
            版本信息
          </div>
          <div class="version-summary">
            <el-form class="basic-info"
                     label-width="100px">
              <el-form-item label="版本名称">
                {{taskDetail.workflow_args.version_args.version}}
              </el-form-item>
              <el-form-item label="版本描述">
                {{taskDetail.workflow_args.version_args.desc}}
              </el-form-item>
              <el-form-item label="版本标签">
                <span v-for="(label,index) in taskDetail.workflow_args.version_args.labels"
                      :key="index"
                      style="margin-right:3px">
                  <el-tag size="small">{{label}}</el-tag>
                </span>
              </el-form-item>
            </el-form>
          </div>

        </el-col>
      </el-row>

      <template v-if="buildDeployArray.length > 0">
        <div class="section-head">
          环境更新
        </div>
        <el-alert class="description" v-if="jenkinsSummary.length > 0"
          show-icon
          title="使用 Zadig 构建将为您节省20%构建时间，建议您迁移构建过程到 Zadig"
           :closable="false"
          type="warning">
        </el-alert>
        <div ></div>
        <el-table :data="buildDeployArray"
                  row-key="_target"
                  :expand-row-keys="expandedBuildDeploys"
                  @expand-change="updateBuildDeployExpanded"
                  row-class-name="my-table-row"
                  empty-text="无"
                  class="build-deploy-table">
          <el-table-column type="expand">
            <template slot-scope="scope">
              <task-detail-build :buildv2="scope.row.buildv2SubTask"
                                 :docker_build="scope.row.docker_buildSubTask"
                                 :isWorkflow="true"
                                 :serviceName="scope.row._target"
                                 :pipelineName="workflowName"
                                 :projectName="projectName"
                                 :taskID="taskID"
                                 ref="buildComp"></task-detail-build>
              <task-detail-deploy :deploys="scope.row.deploySubTasks"
                                  :pipelineName="workflowName"
                                  :taskID="taskID"></task-detail-deploy>
            </template>
          </el-table-column>

          <el-table-column prop="_target"
                           label="服务"
                           width="200px"></el-table-column>

          <el-table-column label="构建"
                           width="250px">
            <template slot-scope="scope">
              <span :class="scope.row.buildOverallColor">
                {{ scope.row.buildOverallStatusZh }}
              </span>
              {{ scope.row.buildOverallTimeZh }}
              <el-tooltip v-if="scope.row.buildOverallTimeZhSec<0"
                          content="本地系统时间和服务端可能存在不一致，请同步。"
                          placement="top">
                <i class="el-icon-warning"
                   style="color:red"></i>
              </el-tooltip>
            </template>
          </el-table-column>

          <el-table-column>
            <template slot="header"
                      slot-scope="scope">
              部署
              <deploy-icons></deploy-icons>
            </template>
            <template slot-scope="scope">
              <div v-if="scope.row.deploySubTasks">
                <template>
                  <span v-for="(task,index) in scope.row.deploySubTasks"
                        :key="index">
                    <span :class="colorTranslation(task.status, 'pipeline', 'task')">
                      <span v-if="task.service_type === 'k8s'">
                        <i class="iconfont iconrongqifuwu"></i>
                        {{task.service_name}}
                      </span>
                      <span v-else-if="task.service_type === 'helm'">
                        <i class="iconfont iconhelmrepo"></i>
                        {{task.container_name}}
                      </span>
                      {{':'+ myTranslate(task.status)}}
                    </span>
                    {{ makePrettyElapsedTime(task) }}
                    <el-tooltip v-if="calcElapsedTimeNum(task)<0"
                                content="本地系统时间和服务端可能存在不一致，请同步。"
                                placement="top">
                      <i class="el-icon-warning"
                         style="color:red"></i>
                    </el-tooltip>
                  </span>
                </template>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </template>
      <template v-if="artifactDeployArray.length > 0">
        <div class="section-head">
          环境更新
        </div>

        <el-table :data="artifactDeployArray"
                  row-key="_target"
                  :expand-row-keys="expandedArtifactDeploys"
                  @expand-change="updateArtifactDeployExpanded"
                  row-class-name="my-table-row"
                  empty-text="无"
                  class="build-deploy-table">
          <el-table-column type="expand">
            <template slot-scope="scope">
              <task-detail-artifact-deploy :deploy="scope.row.deploySubTask">
              </task-detail-artifact-deploy>
            </template>
          </el-table-column>

          <el-table-column prop="_target"
                           label="服务"
                           width="200px"></el-table-column>

          <el-table-column>
            <template slot="header"
                      slot-scope="scope">
              交付物部署
            </template>
            <template slot-scope="scope">
              <span :class="scope.row.buildOverallColor">
                {{ scope.row.buildOverallStatusZh }}
              </span>
              {{ scope.row.buildOverallTimeZh }}
              <el-tooltip v-if="scope.row.buildOverallTimeZhSec<0"
                          content="本地系统时间和服务端可能存在不一致，请同步。"
                          placement="top">
                <i class="el-icon-warning"
                   style="color:red"></i>
              </el-tooltip>
            </template>
          </el-table-column>
        </el-table>
      </template>
      <template v-if="distributeArrayExpanded.length > 0">
        <div class="section-head">
          分发
        </div>

        <el-table :data="distributeArrayExpanded"
                  :span-method="distributeSpanMethod"
                  row-class-name="my-table-row"
                  empty-text="无"
                  style="width: 100%"
                  class="release-table">
          <el-table-column prop="_target"
                           label="服务"
                           width="200px"></el-table-column>

          <el-table-column label="运行状态"
                           width="180px">
            <template slot-scope="scope">
              <span :class="colorTranslation(scope.row.status, 'pipeline', 'task')">
                {{ myTranslate(scope.row.status) }}
              </span>
              {{ makePrettyElapsedTime(scope.row) }}
              <el-tooltip v-if="calcElapsedTimeNum(scope.row)<0"
                          content="本地系统时间和服务端可能存在不一致，请同步。"
                          placement="top">
                <i class="el-icon-warning"
                   style="color:red"></i>
              </el-tooltip>
            </template>
          </el-table-column>

          <el-table-column label="分发方式"
                           width="120px">
            <template slot-scope="scope">
              {{ $translate.translateSubTaskType(scope.row.type) }}
            </template>
          </el-table-column>
          <el-table-column label="输出">
            <template slot-scope="scope">
              <template v-if="typeof scope.row.location === 'object'">
                <span style="display:block"
                      v-for="(item,index) in scope.row.location"
                      :key="index">{{ item.name }}</span>
              </template>
              <span
                    v-else-if="typeof scope.row.location === 'string'">{{ scope.row.location }}</span>
              <el-popover v-else-if="scope.row.error"
                          placement="top"
                          trigger="hover"
                          :content="scope.row.error">
                <span class="distribute-error"
                      slot="reference">错误 <i class="icon error el-icon-warning"></i></span>
              </el-popover>
            </template>
          </el-table-column>
        </el-table>
      </template>
      <el-backtop target=".workflow-or-pipeline-task-detail"></el-backtop>
    </div>
</template>

<script>
import {
  workflowTaskDetailAPI, workflowTaskDetailSSEAPI, restartWorkflowAPI, cancelWorkflowAPI
} from '@api';
import { wordTranslate, colorTranslate } from '@utils/word_translate.js';
import deployIcons from '@/components/common/deploy_icons';
import artifactDownload from '@/components/common/artifact_download.vue';
import taskDetailBuild from './workflow_multi_task_detail/task_detail_build.vue';
import taskDetailDeploy from './workflow_multi_task_detail/task_detail_deploy.vue';
import taskDetailArtifactDeploy from './workflow_multi_task_detail/task_detail_artifact_deploy.vue';
import bus from '@utils/event_bus';
import Etable from '@/components/common/etable'
import _ from 'lodash'

export default {
  data() {
    return {
      workflow: {
      },
      securitySummary: {},
      taskDetail: {
        stages: [],
      },
      rules: {
        version: [
          { required: true, message: '请填写版本名称', trigger: 'blur' }
        ]
      },
      argColumns: [
        {
          prop: 'name',
          label: 'name',
        },
        {
          prop: 'value',
          label: 'value',
        },
      ],
      jenkinsBuildColumns: [
        {
          prop: 'service_name',
          label: '服务',
          width: 160
        },
        {
          label: 'Jenkins Job Name',
          render: (scope) => {
            return (<div>{scope.row.jenkins_build_args.job_name}</div>)
          }
        },
        {
          label: '构建参数',
          width: 220,
          render: (scope) => {
            return (<el-popover placement="top-end" width="300" trigger="hover">
                      <Etable tableColumns={this.argColumns} tableData={scope.row.jenkins_build_args.jenkins_build_params} id="id" />
                      <span size="small" style="color: #409EFF" slot="reference">查看</span>
                    </el-popover>)
          }
        },
      ],
      inputTagVisible: false,
      inputValue: '',
      artifactModalVisible: false,
      expandedBuildDeploys: [],
      expandedArtifactDeploys: [],
      expandedTests: [],
    };
  },
  computed: {
    workflowName() {
      return this.$route.params.workflow_name;
    },
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    taskID() {
      return this.$route.params.task_id;
    },
    status() {
      return this.$route.query.status;
    },
    securityCheckDone() {
      return this.isStageDone('security');
    },
    workflowProductTemplate() {
      return this.workflow.product_tmpl_name
    },
    projectName() {
      return this.$route.params.project_name ? this.$route.params.project_name : this.workflowProductTemplate;
    },
    artifactDeployMap() {
      const map = {};
      const stage = this.taskDetail.stages.find(stage => stage.type === 'artifact');
      if (stage) {
        this.collectSubTask(map, 'deploy');
      }
      return map;
    },
    artifactDeployArray() {
      const arr = this.$utils.mapToArray(this.artifactDeployMap, '_target');
      for (const target of arr) {
        target.buildOverallStatusZh = this.myTranslate(target.deploySubTask.status);
        target.buildOverallColor = this.colorTranslation(target.deploySubTask.status, 'pipeline', 'task');
        target.buildOverallTimeZhSec = this.calcElapsedTimeNum(target.deploySubTask);
        target.buildOverallTimeZh = this.$utils.timeFormat(target.buildOverallTimeZhSec);
      }
      return arr;
    },
    buildDeployMap() {
      const map = {};
      this.collectBuildDeploySubTask(map);
      this.collectSubTask(map, 'docker_build');
      return map;
    },
    buildDeployArray() {
      const arr = this.$utils.mapToArray(this.buildDeployMap, '_target');
      for (const target of arr) {
        target.buildOverallStatus = this.$utils.calcOverallBuildStatus(
          target.buildv2SubTask, target.docker_buildSubTask
        );
        target.buildOverallStatusZh = this.myTranslate(target.buildOverallStatus);
        target.buildOverallColor = this.colorTranslation(target.buildOverallStatus, 'pipeline', 'task');
        target.buildOverallTimeZhSec = this.calcElapsedTimeNum(target.buildv2SubTask) + this.calcElapsedTimeNum(target.docker_buildSubTask);
        target.buildOverallTimeZh = this.$utils.timeFormat(target.buildOverallTimeZhSec);
      }
      return arr;
    },
    buildSummary() {
      let buildArr = this.$utils.mapToArray(this.buildDeployMap, '_target').filter(item => item.buildv2SubTask.type === 'buildv2');
      let summary = buildArr.map(element => {
          return {
            service_name: element._target,
            builds: _.get(element, 'buildv2SubTask.job_ctx.builds' , ''),
          }
      });
      return summary;
    },
    jenkinsSummary() {
      let buildArr = this.$utils.mapToArray(this.buildDeployMap, '_target').filter(item => item.buildv2SubTask.type === 'jenkins_build');
      let summary = buildArr.map(element => {
          return {
            service_name: element._target,
            builds: _.get(element, 'buildv2SubTask.job_ctx' , ''),
            jenkins_build_args:  _.get(element, 'buildv2SubTask.jenkins_build_args' , ''),
          }
      });
      return summary;
    },
    distributeMap() {
      const map = {};
      this.collectSubTask(map, 'distribute2kodo');
      this.collectSubTask(map, 'release_image');
      this.collectSubTask(map, 'distribute');
      return map;
    },
    distributeArray() {
      const arr = this.$utils.mapToArray(this.distributeMap, '_target');
      for (const item of arr) {
        if (item.distribute2kodoSubTask) {
          item.distribute2kodoSubTask.distribute2kodoPath = item.distribute2kodoSubTask.remote_file_key;
        }
        if (item.release_imageSubTask) {
          item.release_imageSubTask._image = item.release_imageSubTask.image_release
            ? item.release_imageSubTask.image_release.split('/')[2]
            : '*';
        }
      }
      return arr;
    },
    distributeArrayExpanded() {
      const wanted = ['distribute2kodoSubTask', 'release_imageSubTask', 'distributeSubTask'];

      const outputKeys = {
        distribute2kodoSubTask: 'package_file',
        release_imageSubTask: '_image',
        distributeSubTask: 'package_file',
      };
      const locationKeys = {
        distribute2kodoSubTask: 'distribute2kodoPath',
        release_imageSubTask: 'image_repo',
        distributeSubTask: 'dist_host',
      };

      const twoD = this.distributeArray.map(map => {
        let typeCount = 0;
        const arr = [];
        for (const key of wanted) {
          if (key in map) {
            typeCount++;
            const item = map[key];
            item._target = map._target;
            item.output = item[outputKeys[key]];
            if (key === 'release_imageSubTask') {
              item.location = item.releases ? item.releases : item.image_release;
            } else {
              item.location = item[locationKeys[key]];
            }
            arr.push(item);
          }
        }
        arr[0].typeCount = typeCount;
        return arr;
      });
      return this.$utils.flattenArray(twoD);
    },
  },
  methods: {
    isStageDone(name) {
      if (this.taskDetail.stages.length > 0) {
        let stage = this.taskDetail.stages.find(element => {
          return element.type === name;
        });
        return stage ? stage.status === 'passed' : false;
      }

    },
    rerun() {
      const taskUrl = `/v1/projects/detail/${this.projectName}/pipelines/multi/${this.workflowName}/${this.taskID}`;
      restartWorkflowAPI(this.workflowName, this.taskID).then(res => {
        this.$message.success('任务已重新启动');
        this.$router.push(taskUrl);
      });
    },
    cancel() {
      cancelWorkflowAPI(this.workflowName, this.taskID).then(res => {
        if (this.$refs && this.$refs.buildComp) {
          this.$refs.buildComp.killLog('buildv2');
          this.$refs.buildComp.killLog('docker_build');
        }
        if (this.$refs && this.$refs.testComp) {
          this.$refs.testComp.killLog('test');
        }
        this.$message.success('任务取消成功');
      });
    },
    collectSubTask(map, typeName) {
      const stage = this.taskDetail.stages.find(stage => stage.type === typeName);
      if (stage) {
        for (const target in stage.sub_tasks) {
          if (!(target in map)) {
            map[target] = {};
          }
          map[target][`${typeName}SubTask`] = stage.sub_tasks[target];
        }
      }
    },
    collectBuildDeploySubTask(map) {
      const buildStageArray = this.taskDetail.stages.filter(stage => stage.type === 'buildv2' || stage.type === 'jenkins_build');
      const deployStage = this.taskDetail.stages.find(stage => stage.type === 'deploy');
      if (buildStageArray) {
        buildStageArray.forEach(buildStage => {
        for (const buildKey in buildStage.sub_tasks) {
          if (!(buildStage.sub_tasks[buildKey].service_name in map)) {
            map[buildStage.sub_tasks[buildKey].service_name] = {};
          }
          map[buildStage.sub_tasks[buildKey].service_name][`buildv2SubTask`] = buildStage.sub_tasks[buildKey];
          map[buildStage.sub_tasks[buildKey].service_name][`deploySubTasks`] = [];
          if (deployStage) {
            for (const deployKey in deployStage.sub_tasks) {
              if (buildStage.sub_tasks[buildKey].service_name === deployStage.sub_tasks[deployKey].container_name) {
                map[buildStage.sub_tasks[buildKey].service_name][`deploySubTasks`].push(deployStage.sub_tasks[deployKey]);
              }
            }
          }
        }
        })
      }

    },

    fetchTaskDetail() {
      return workflowTaskDetailSSEAPI(this.workflowName, this.taskID).then(res => {
        this.adaptTaskDetail(res.data);
        this.taskDetail = res.data;
        this.workflow = res.data.workflow_args;
      }).closeWhenDestroy(this);
    },
    fetchOldTaskDetail() {
      workflowTaskDetailAPI(this.workflowName, this.taskID).then(res => {
        this.adaptTaskDetail(res);
        this.taskDetail = res;
        this.workflow = res.workflow_args;
      });
    },
    adaptTaskDetail(detail) {
      detail.intervalSec = (detail.status === 'running' ? Math.round((new Date()).getTime() / 1000) : detail.end_time) - detail.start_time;
      detail.interval = this.$utils.timeFormat(detail.intervalSec);
    },
    repoID(repo) {
      return `${repo.source}/${repo.repo_owner}/${repo.repo_name}`;
    },

    myTranslate(word) {
      return wordTranslate(word, 'pipeline', 'task');
    },
    colorTranslation(word, category, subitem) {
      return colorTranslate(word, category, subitem);
    },
    calcElapsedTimeNum(subTask) {
      if (this.$utils.isEmpty(subTask) || subTask.status === '') {
        return 0;
      }
      const endTime = subTask.status === 'running' ? Math.floor(Date.now() / 1000) : subTask.end_time;
      return endTime - subTask.start_time;
    },
    makePrettyElapsedTime(subTask) {
      return this.$utils.timeFormat(this.calcElapsedTimeNum(subTask));
    },

    distributeSpanMethod({ row, column, rowIndex, columnIndex }) {
      if (columnIndex === 0) {
        if ('typeCount' in row) {
          return {
            rowspan: row.typeCount,
            colspan: 1,
          };
        }
        return {
          rowspan: 0,
          colspan: 1,
        };
      }
      return {
        rowspan: 1,
        colspan: 1,
      };
    },

    updateBuildDeployExpanded(row, expandedRows) {
      this.expandedBuildDeploys = expandedRows.map(r => r._target);
    },
    updateArtifactDeployExpanded(row, expandedRows) {
      this.expandedBuildDeploys = expandedRows.map(r => r._target);
    },
    updateTestExpanded(row, expandedRows) {
      this.expandedTests = expandedRows.map(r => r._target);
    },
    showOperation() {
      if (this.taskDetail.status === 'failed' || this.taskDetail.status === 'cancelled' || this.taskDetail.status === 'timeout') {
        return true
      }
      if (this.taskDetail.status === 'running' || this.taskDetail.status === 'created') {
        return true
      }

      if (this.taskDetail.status === 'passed' && this.distributeArrayExpanded.length > 0) {
        return true
      }

      return false
    },
    setTitleBar() {
      bus.$emit(`set-topbar-title`, {
        title: '',
        breadcrumb: [
          { title: '项目', url: '/v1/projects' },
          { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
          { title: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
          { title: this.workflowName, url: `/v1/projects/detail/${this.projectName}/pipelines/multi/${this.workflowName}` },
          { title: `#${this.taskID}`, url: '' }]
      });
      bus.$emit(`set-sub-sidebar-title`, {
        title: this.projectName,
        url: `/v1/projects/detail/${this.projectName}`,
        routerList: [
          { name: '工作流', url: `/v1/projects/detail/${this.projectName}/pipelines` },
          { name: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs` },
          { name: '服务', url: `/v1/projects/detail/${this.projectName}/services` },
          { name: '构建', url: `/v1/projects/detail/${this.projectName}/builds` },
          ]
      });
    }
  },
  watch: {
    $route(to, from) {
      this.setTitleBar();
      if (this.$route.query.status === 'passed' || this.$route.query.status === 'failed' || this.$route.query.status === 'timeout' || this.$route.query.status === 'cancelled') {
        this.fetchOldTaskDetail();
      }
      else {
        this.fetchTaskDetail();
      }
    },
    securityCheckDone(val, oldVal) {
      if (val && val !== oldVal) {
        this.securityArray.forEach(element => {
          this.getSecurityReportSummary(element.securitySubTask.image_id);
        });
      }
    }
  },
  created() {
    this.setTitleBar();
    if (this.$route.query.status === 'passed' || this.$route.query.status === 'failed' || this.$route.query.status === 'timeout' || this.$route.query.status === 'cancelled') {
      this.fetchOldTaskDetail();
    }
    else {
      this.fetchTaskDetail();
    }
  },
  components: {
    deployIcons,
    artifactDownload,
    taskDetailBuild,
    taskDetailDeploy,
    taskDetailArtifactDeploy,
    Etable
  },
};
</script>

<style lang="less">
.issue-popper {
  display: inline-block;
  font-size: 14px;
  p {
    margin: 0.5em 0;
  }
  .issue-url {
    color: #1989fa;
    cursor: pointer;
  }
}
.workflow-task-detail {
  flex: 1;
  position: relative;
  padding: 0px 20px;
  overflow: auto;
  .el-breadcrumb {
    font-size: 16px;
  }
  .build-summary {
    .repo-name {
      font-size: 15px;
    }
    .link a {
      color: #1989fa;
      cursor: pointer;
    }
    .el-row {
      margin-bottom: 5px;
    }
  }
  .version-summary {
    .title {
      font-size: 14px;
      color: #606266;
      line-height: 40px;
    }
    .content {
      font-size: 14px;
      color: #333;
    }
  }

  .section-head {
    width: 222px;
    font-size: 16px;
    height: 28px;
    line-height: 28px;
    margin-top: 25px;
    color: #303133;
    border-bottom: 1px solid #eee;
  }
  .section-title {
    display: inline-block;
    color: #666;
    font-size: 13px;
    margin-top: 20px;
    margin-left: 15px;
  }
  .version-link,
  .download-artifact-link {
    color: #1989fa;
    cursor: pointer;
  }

  .basic-info,
  .build-deploy-table,
  .test-table,
  .release-table {
    margin-top: 10px;
  }
  .el-form-item {
    margin-bottom: 0;
  }
  .el-form-item__label {
    text-align: left;
  }

  .build-deploy-table,
  .test-table,
  .release-table {
    span[class^="color-"] {
      margin-right: 8px;
    }
    .icon {
      cursor: pointer;
      font-size: 18px;
    }
    .error {
      color: #ff1989;
    }
  }
  .security-table,
  .release-table {
    margin-left: 48px;
  }
  .show-test-result {
    a {
      color: #1989fa;
      cursor: pointer;
    }
  }
  .el-table__expanded-cell {
    padding: 0;
  }

  .my-table-row {
    background-color: #f5faff;
  }
  .issue-name-wrapper {
    display: block;
    a {
      color: #1989fa;
      margin-right: 4px;
    }
  }
}
.description {
    font-size: 14px;
    color: #606266;
    margin-top: 10px;
}
</style>
