<template>
  <div class="mobile-task-detail">
    <van-nav-bar left-arrow
                 fixed
                 @click-left="mobileGoback">
      <template #title>
        <span>{{workflowName}}</span>
        <van-tag plain
                 type="primary">{{`#${taskID}`}}</van-tag>
      </template>
    </van-nav-bar>
    <div class="task-info">
      <van-row>
        <van-col span="12">
          <div class="mobile-block">
            <h2 class="mobile-block-title">状态</h2>
            <div class="mobile-block-desc">
              <van-tag size="small"
                       :type="$utils.mobileElTagType(taskDetail.status)">
                {{ myTranslate(taskDetail.status) }}</van-tag>
            </div>
          </div>
        </van-col>
        <van-col span="12">
          <div class="mobile-block">
            <h2 class="mobile-block-title">创建者</h2>
            <div class="mobile-block-desc">{{ taskDetail.task_creator }}</div>
          </div>
        </van-col>
      </van-row>
      <van-row>
        <van-col span="12">
          <div class="mobile-block">
            <h2 class="mobile-block-title">集成环境</h2>
            <div class="mobile-block-desc"> {{ workflow.product_tmpl_name }} -
              {{ workflow.namespace }}
            </div>
          </div>
        </van-col>
        <van-col span="12">
          <div class="mobile-block">
            <h2 class="mobile-block-title">持续时间</h2>
            <div class="mobile-block-desc"> {{ taskDetail.interval }}</div>
          </div>
        </van-col>
      </van-row>
    </div>
    <template v-if="taskDetail.status!=='passed'">
      <div class="mobile-block">
        <h2 class="mobile-block-title">操作</h2>
      </div>
      <van-cell is-link
                title="操作"
                @click="showAction = true" />
      <van-action-sheet close-on-click-action
                        v-model="showAction"
                        :actions="actions"
                        cancel-text="取消"
                        @select="onSelectAction"
                        @open="openSelectAction"
                        @cancel="onCancel" />
    </template>
    <template v-if="buildDeployArray.length > 0">
      <div class="mobile-block">
        <h2 class="mobile-block-title">环境更新</h2>
      </div>
      <van-collapse v-model="buildActive">
        <van-collapse-item v-for="(item,index) in buildDeployArray"
                           :key="index"
                           :name="item._target">
          <template #title>
            <van-row>
              <van-col span="8">{{item._target}}</van-col>
              <van-col span="8">
                <span :class="item.buildOverallColor">
                  {{ item.buildOverallStatusZh }}
                </span>
                {{ item.buildOverallTimeZh }}
              </van-col>
              <van-col v-if="item.deploySubTasks"
                       span="8">
                <template>
                  <span v-for="(task,index) in item.deploySubTasks"
                        :key="index">
                    <span :class="colorTranslation(task.status, 'pipeline', 'task')">
                      {{myTranslate(task.status) }}
                    </span>
                    {{ makePrettyElapsedTime(task) }}
                  </span>
                </template>
              </van-col>
            </van-row>
          </template>
          <task-detail-build :buildv2="item.buildv2SubTask"
                             :docker_build="item.docker_buildSubTask"
                             :isWorkflow="true"
                             :serviceName="item._target"
                             :pipelineName="workflowName"
                             :projectName="projectName"
                             :taskID="taskID"
                             ref="buildComp"></task-detail-build>
          <task-detail-deploy :deploys="item.deploySubTasks"
                              :pipelineName="workflowName"
                              :taskID="taskID"></task-detail-deploy>
        </van-collapse-item>
      </van-collapse>
    </template>
  </div>
</template>
<script>
import { Col, Collapse, CollapseItem, Row, NavBar, Tag, Panel, Loading, Button, Notify, Tab, Tabs, Cell, CellGroup, Icon, Divider, ActionSheet } from 'vant';
import taskDetailBuild from './task_detail_build.vue';
import taskDetailDeploy from './task_detail_deploy.vue';
import { wordTranslate, colorTranslate } from '@utils/word_translate.js';
import {
  workflowTaskDetailAPI, workflowTaskDetailSSEAPI, restartWorkflowAPI, cancelWorkflowAPI
} from '@api';
export default {
  components: {
    [NavBar.name]: NavBar,
    [Tag.name]: Tag,
    [Panel.name]: Panel,
    [Loading.name]: Loading,
    [Button.name]: Button,
    [Notify.name]: Notify,
    [Tab.name]: Tab,
    [Tabs.name]: Tabs,
    [Cell.name]: Cell,
    [CellGroup.name]: CellGroup,
    [Icon.name]: Icon,
    [Col.name]: Col,
    [Row.name]: Row,
    [Divider.name]: Divider,
    [ActionSheet.name]: ActionSheet,
    [Collapse.name]: Collapse,
    [CollapseItem.name]: CollapseItem,
    taskDetailBuild,
    taskDetailDeploy

  },
  data() {
    return {
      showAction: false,
      actions: [

      ],
      workflow: {
      },
      taskDetail: {
        stages: [],
      },
      buildActive: [],
      securityActive: [],
      securitySummary: {},
    }
  },
  methods: {
    onCancel() {
      this.showAction = false;
    },
    openSelectAction() {
      this.actions = [];
      if (this.taskDetail.status === 'failed' || this.taskDetail.status === 'cancelled' || this.taskDetail.status === 'timeout') {
        this.actions.push({ name: '失败重试' });
      }
      if (this.taskDetail.status === 'running' || this.taskDetail.status === 'created') {
        this.actions.push({ name: '取消任务' });
      }
    },
    onSelectAction(action) {
      if (action.name === '失败重试') {
        restartWorkflowAPI(this.workflowName, this.taskID).then(res => {
          Notify({ type: 'success', message: '任务已重新启动' });
          this.$router.push('/mobile/status');
        });
      }
      else if (action.name === '取消任务') {
        cancelWorkflowAPI(this.workflowName, this.taskID).then(res => {
          if (this.$refs && this.$refs.buildComp) {
            this.$refs.buildComp.killLog('buildv2');
            this.$refs.buildComp.killLog('docker_build');
          }
          Notify({ type: 'success', message: '任务取消成功' });
        });
      }
      this.showAction = false;
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
    adaptTaskDetail(detail) {
      detail.interval = this.$utils.timeFormat(
        (detail.status === 'running'
          ? Math.round((new Date()).getTime() / 1000)
          : detail.end_time) - detail.start_time
      );
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
      const buildStage = this.taskDetail.stages.find(stage => stage.type === 'buildv2');
      const deployStage = this.taskDetail.stages.find(stage => stage.type === 'deploy');
      if (buildStage) {
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
    isStageDone(name) {
      if (this.taskDetail.stages.length > 0) {
        let stage = this.taskDetail.stages.find(element => {
          return element.type === name;
        });
        return stage ? stage.status === 'passed' : false;
      }

    },
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
    workflowProductTemplate() {
      return this.workflow.product_tmpl_name
    },
    projectName() {
      return this.$route.params.project_name ? this.$route.params.project_name : this.workflowProductTemplate;
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
        target.buildOverallTimeZh = this.$utils.timeFormat(
          this.calcElapsedTimeNum(target.buildv2SubTask) + this.calcElapsedTimeNum(target.docker_buildSubTask)
        );
      }
      return arr;
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
  mounted() {
    if (this.status === 'running') {
      this.fetchTaskDetail();
    } else {
      this.fetchOldTaskDetail();
    }
  },
}
</script>
<style lang="less">
.mobile-task-detail {
  padding-top: 46px;
  padding-bottom: 50px;
}
</style>