<template>
  <div class="product-status-container">
    <div v-for="task in productTasks.running"
         :key="task.task_id"
         class="task-container">
      <div class="progress-header">
        <div class="progress-header-view">
          <div class="status-view">
            <div class="status running">
              {{ wordTranslation(task.status,'pipeline','task') }}
            </div>
          </div>
          <div class="info-view">
            <span class="spec">
              <span>
                <label>产品工作流 {{`#${task.task_id}`}}</label>
                <br>
                <router-link
                             :to="`/v1/projects/detail/${task.product_name}/pipelines/multi/${task.pipeline_name}/${task.task_id}?status=${task.status}`">
                  <span class="workflow-name"><i
                       class="el-icon-link"></i>{{`${task.pipeline_name}`}}</span>
                </router-link>
              </span>
            </span>
            <span class="stages-tag">
              <el-tag v-if="showStage(task.stages,'buildv2')"
                      size=small
                      class="stage"
                      type="primary">构建</el-tag>
              <el-tag v-if="showStage(task.stages,'deploy')"
                      size=small
                      class="stage"
                      type="primary">部署</el-tag>
              <el-tag v-if="showStage(task.stages,'artifact')"
                      size=small
                      class="stage"
                      type="primary">交付物部署</el-tag>
              <el-tag v-if="showStage(task.stages,'testingv2')"
                      size=small
                      class="stage"
                      type="primary">测试</el-tag>
              <el-tag v-if="showStage(task.stages,'release_image')"
                      size=small
                      class="stage"
                      type="primary">分发</el-tag>
            </span>
            <section class="basic-info">
              <p class="author"><i class="el-icon-user"></i> {{task.task_creator}}</p>
              <p class="time"><i class="el-icon-time"></i>
                {{$utils.convertTimestamp(task.create_time)}} </p>
            </section>
          </div>
          <div class="operation-view">
            <el-tooltip v-if="!taskDetailExpand[task.task_id]"
                        class="item"
                        effect="dark"
                        content="查看任务流程"
                        placement="top">
              <span @click="showTaskDetail(task.task_id)"
                    class="icon el-icon-data-board view-detail"></span>
            </el-tooltip>
            <el-tooltip v-if="taskDetailExpand[task.task_id]"
                        class="item"
                        effect="dark"
                        content="收起任务流程"
                        placement="top">
              <span @click="closeTaskDetail(task.task_id)"
                    class="icon el-icon-arrow-up view-detail"></span>
            </el-tooltip>
            <el-tooltip class="item"
                        effect="dark"
                        content="删除任务"
                        placement="top">
              <span @click="taskOperate('running','cancel',task.task_id,task.pipeline_name)"
                    class="icon el-icon-delete delete"></span>
            </el-tooltip>

          </div>
        </div>
      </div>
      <div v-if="taskDetailExpand[task.task_id]"
           class="stages">
        <div v-if="showStage(task.stages,'buildv2')"
             class="stage"
             style="min-width: 250px;">
          <div class="line first"></div>
          <div class="stage-header stage-header-empty-status">
            <div class="stage-header-col stage-header-title ">
              <h3 class="stage-title">
                构建
              </h3>
              <i class="icon el-icon-right"></i>
            </div>
          </div>
          <ul class="list-unstyled steps cf-steps-list">
            <li v-if="buildSubtaskInfo(task.stages).staticCheckRepos.length > 0"
                class="cf-steps-list-item">
              <el-popover ref="static_check"
                          placement="right"
                          title="静态检查"
                          width="400"
                          trigger="click">
                <el-table :data="buildSubtaskInfo(task.stages).staticCheckRepos">
                  <el-table-column property="name"
                                   label="代码库"></el-table-column>
                  <el-table-column property="security_measure_count"
                                   label="代码安全扫描"></el-table-column>
                  <el-table-column property="issue_measure_count"
                                   label="代码缺陷"></el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status passed">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      静态检查
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
            <li v-if="buildSubtaskInfo(task.stages).utRepos.length > 0"
                class="cf-steps-list-item">
              <el-popover ref="ut"
                          placement="right"
                          title="单元测试"
                          width="400"
                          trigger="click">
                <el-table :data="buildSubtaskInfo(task.stages).utRepos">
                  <el-table-column property="name"
                                   label="代码库"></el-table-column>
                  <el-table-column label="覆盖率">
                    <template slot-scope="scope">
                      <i class="el-icon-data-analysis"></i>
                      <span v-if="scope.row.no_stmt !== 0">{{
                          (((scope.row.no_stmt-scope.row.no_missed_stmt)/scope.row.no_stmt)*100).toFixed(2)+"%"
                          }}</span>
                      <span v-else>-</span>
                    </template>
                  </el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="buildSubtaskInfo(task.stages).status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      单元测试
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
            <li class="cf-steps-list-item">
              <el-popover ref="script"
                          placement="right"
                          title="构建信息"
                          width="400"
                          trigger="click">
                <el-table :data="buildSubtaskInfo(task.stages).buildRepos">
                  <el-table-column property="repo_name"
                                   label="代码库"></el-table-column>
                  <el-table-column property="branch"
                                   label="分支"></el-table-column>
                  <el-table-column label="PR">
                    <template slot-scope="scope">
                      <span>{{scope.row.pr?scope.row.pr:'-'}}</span>
                    </template>
                  </el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="buildSubtaskInfo(task.stages).status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      脚本构建
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
            <li class="cf-steps-list-item">
              <el-popover ref="build_image"
                          placement="right"
                          title="镜像信息"
                          width="650"
                          trigger="click">
                <el-table :data="buildSubtaskInfo(task.stages).buildImage">
                  <el-table-column property="image_name"
                                   label="Image Name"></el-table-column>
                  <el-table-column property="registry_repo"
                                   label="Registry"></el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="buildSubtaskInfo(task.stages).dockerBuildStatus">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      构建镜像
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>
            </li>
          </ul>
        </div>
        <div v-if="showStage(task.stages,'deploy')"
             class="stage"
             style="min-width: 250px;">
          <div class="line"></div>
          <div class="stage-header stage-header-empty-status">
            <div class="stage-header-col stage-header-title ">
              <h3 class="stage-title">
                部署
              </h3>
              <i class="icon el-icon-right"></i>
            </div>
          </div>
          <ul class="list-unstyled steps cf-steps-list">
            <li class="cf-steps-list-item">
              <el-popover ref="deploy_env"
                          placement="right"
                          title="环境更新"
                          width="550"
                          trigger="click">
                <el-table :data="deploySubtaskInfo(task.stages).serviceLists">
                  <el-table-column property="service_name"
                                   label="服务列表"></el-table-column>
                  <el-table-column property="namespace"
                                   label="环境"></el-table-column>
                  <el-table-column property="image"
                                   label="镜像"></el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="deploySubtaskInfo(task.stages).status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      环境更新
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
          </ul>
        </div>
        <div v-if="showStage(task.stages,'testingv2')"
             class="stage"
             style="min-width: 250px;">
          <div class="line"></div>
          <div class="stage-header stage-header-empty-status">
            <div class="stage-header-col stage-header-title ">
              <h3 class="stage-title">
                测试
              </h3>
              <i class="icon el-icon-right"></i>
            </div>
          </div>
          <ul class="list-unstyled steps cf-steps-list">
            <li class="cf-steps-list-item">
              <el-popover ref="function_test"
                          placement="right"
                          title="功能测试-代码信息"
                          width="400"
                          trigger="click">
                <el-table :data="testSubtaskInfo(task).integration_test.builds">
                  <el-table-column property="repo_name"
                                   label="代码库"></el-table-column>
                  <el-table-column property="branch"
                                   label="分支"></el-table-column>
                  <el-table-column label="PR">
                    <template slot-scope="scope">
                      <span>{{scope.row.pr?scope.row.pr:'-'}}</span>
                    </template>
                  </el-table-column>
                </el-table>
                <div style="margin-top: 10px;margin-right: 15px;text-align: right;">
                  <el-link v-if="testSubtaskInfo(task).integration_test.report_ready"
                           :href="testSubtaskInfo(task).integration_test.report_url"
                           type="primary">测试报告</el-link>
                </div>
                <div slot="reference"
                     class="step step-status"
                     :class="testSubtaskInfo(task).integration_test.status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      功能测试
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
            <li class="cf-steps-list-item">
              <el-popover ref="performance_test"
                          placement="right"
                          title="性能测试-代码信息"
                          width="400"
                          trigger="click">
                <el-table :data="testSubtaskInfo(task).performance_test.builds">
                  <el-table-column property="repo_name"
                                   label="代码库"></el-table-column>
                  <el-table-column property="branch"
                                   label="分支"></el-table-column>
                  <el-table-column label="PR">
                    <template slot-scope="scope">
                      <span>{{scope.row.pr?scope.row.pr:'-'}}</span>
                    </template>
                  </el-table-column>
                </el-table>
                <div style="margin-top: 10px;margin-right: 15px;text-align: right;">
                  <el-link v-if="testSubtaskInfo(task).performance_test.report_ready"
                           :href="testSubtaskInfo(task).performance_test.report_url"
                           type="primary">测试报告</el-link>
                </div>

                <div slot="reference"
                     class="step step-status"
                     :class="testSubtaskInfo(task).performance_test.status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      性能测试
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
            <li class="cf-steps-list-item">
              <el-popover placement="right"
                          title="安全扫描"
                          width="600"
                          trigger="click">
                <el-table :data="testSubtaskInfo(task).security_scan.results">
                  <el-table-column property="image_name"
                                   label="镜像"></el-table-column>
                  <el-table-column width="80px"
                                   property="all"
                                   label="漏洞数量"></el-table-column>
                  <el-table-column width="80px"
                                   property="critical"
                                   label="严重"></el-table-column>
                  <el-table-column width="80px"
                                   property="medium"
                                   label="中危"></el-table-column>
                  <el-table-column width="80px"
                                   property="low"
                                   label="低危"></el-table-column>

                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="testSubtaskInfo(task).security_scan.status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      安全扫描
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
          </ul>
        </div>
        <div v-if="showStage(task.stages,'release_image')"
             class="stage"
             style="min-width: 250px;">
          <div class="line"></div>
          <div class="stage-header stage-header-empty-status">
            <div class="stage-header-col stage-header-title ">
              <h3 class="stage-title">
                分发
              </h3>
              <i class="icon el-icon-right"></i>
            </div>
          </div>
          <ul class="list-unstyled steps cf-steps-list">
            <li class="cf-steps-list-item">
              <el-popover ref="release_image"
                          placement="right"
                          title="镜像分发"
                          width="550"
                          trigger="click">
                <el-table :data="distributeSubtaskInfo(task.stages).releaseImages">
                  <el-table-column property="image_repo"
                                   label="镜像仓库"></el-table-column>
                  <el-table-column property="image_test"
                                   label="镜像名称"></el-table-column>
                </el-table>
                <div slot="reference"
                     class="step step-status"
                     :class="distributeSubtaskInfo(task.stages).status">
                  <div class="step-data">
                    <i class="el-icon-cloudy"></i>
                    <span class="step-description">
                      镜像分发
                    </span>

                    <span class="step-type"></span>
                  </div>

                </div>
              </el-popover>

            </li>
          </ul>
        </div>
      </div>
    </div>
    <div v-for="task in productTasks.pending"
         :key="task.task_id"
         class="progress-header">
      <div class="progress-header-view">
        <div class="status-view">
          <div class="status pending">
            队列中
          </div>
        </div>
        <div class="info-view">
          <span class="spec">
            <span>
              <label>工作流 {{`#${task.task_id}`}}</label>
              <br>
              <router-link
                           :to="`/v1/projects/detail/${task.product_name}/pipelines/multi/${task.pipeline_name}/${task.task_id}?status=${task.status}`">
                <span class="workflow-name"><i
                     class="el-icon-link"></i>{{`${task.pipeline_name}`}}</span>
              </router-link>
            </span>
          </span>
          <span class="stages-tag">
            <el-tag v-if="showStage(task.stages,'buildv2')"
                    size=small
                    class="stage"
                    type="primary">构建</el-tag>
            <el-tag v-if="showStage(task.stages,'deploy')"
                    size=small
                    class="stage"
                    type="primary">部署</el-tag>
            <el-tag v-if="showStage(task.stages,'artifact')"
                    size=small
                    class="stage"
                    type="primary">交付物部署</el-tag>
            <el-tag v-if="showStage(task.stages,'testingv2')"
                    size=small
                    class="stage"
                    type="primary">测试</el-tag>
            <el-tag v-if="showStage(task.stages,'release_image')"
                    size=small
                    class="stage"
                    type="primary">分发</el-tag>
          </span>
          <section class="basic-info">
            <p class="author"><i class="el-icon-user"></i> {{task.task_creator}}</p>
            <p class="time"><i class="el-icon-time"></i>
              {{$utils.convertTimestamp(task.create_time)}} </p>
          </section>
        </div>
        <div class="operation-view">
          <span style="visibility:hidden;"
                class="icon el-icon-data-board view-detail"></span>
          <el-tooltip class="item"
                      effect="dark"
                      content="删除任务"
                      placement="top">
            <span @click="taskOperate('queue','cancel',task.task_id,task.pipeline_name)"
                  class="icon el-icon-delete delete"></span>
          </el-tooltip>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { cancelWorkflowAPI } from '@api';
import { wordTranslate } from '@utils/word_translate';
export default {
  data() {
    return {
      taskDetailExpand: {},
    }
  },
  watch: {
    expandId: {
      handler(newVal) {
        this.taskDetailExpand[newVal] = true;
      },
      deep: true
    }
  },
  methods: {
    /* 
  任务操作
  * @param  {string}           task_type 任务类型（running，queue）
  * @param  {string}           operation 操作 （cancel，restart，delete）
  * @param  {number}           id 任务 id
  * @param  {string}           pipeline_name 流水线名
  * @return {}           
  */
    taskOperate(task_type, operation, id, pipeline_name) {
      if (task_type === 'running') {
        switch (operation) {
          case 'cancel':
            cancelWorkflowAPI(pipeline_name, id).then(res => {
              this.$notify({
                title: '成功',
                message: '运行任务取消成功',
                type: 'success',
                offset: 50
              });
            });
            break;
          case 'restart':
            break;
          case 'delete':
            break;
          default:
            break;
        }
      } else if (task_type === 'queue') {
        switch (operation) {
          case 'cancel':
            cancelWorkflowAPI(pipeline_name, id).then(res => {
              this.$notify({
                title: '成功',
                message: '队列任务取消成功',
                type: 'success',
                offset: 50
              });
            });
            break;
          case 'restart':
            break;
          case 'delete':
            break;
          default:
            break;
        }
      }
    },
    wordTranslation(word, category, subitem = '') {
      return wordTranslate(word, category, subitem);
    },
    showStage(stages, stage_name) {
      let flag = false;
      stages.forEach(stage => {
        if (stage_name === stage.type) {
          flag = true;
        }
      });
      return flag;
    },
    showTaskDetail(task_id) {
      this.taskDetailExpand[task_id] = true;
    },
    closeTaskDetail(task_id) {
      this.taskDetailExpand[task_id] = false;
    },
    buildSubtaskInfo(stages) {
      let meta = {
        status: '',
        dockerBuildStatus: '',
        staticCheckRepos: [],
        buildImage: [],
        utRepos: [],
        buildRepos: []
      };
      stages.forEach(stage => {
        if (stage.type === 'buildv2') {
          meta.status = stage.status;
          for (const sub_task in stage.sub_tasks) {
            const static_check_element = stage.sub_tasks[sub_task]['static_check_status']['repos'];
            const ut_element = stage.sub_tasks[sub_task]['ut_status']['repos'];
            const build_repos_element = stage.sub_tasks[sub_task]['job_ctx']['builds'];
            const build_image_element = stage.sub_tasks[sub_task]['docker_build_status'];
            meta.dockerBuildStatus = stage.sub_tasks[sub_task]['docker_build_status']['status'];
            meta.buildImage.push(build_image_element);
            if (static_check_element) {
              meta.staticCheckRepos = meta.staticCheckRepos.concat(static_check_element);
            }
            else {
              meta.staticCheckRepos = []
            }
            if (ut_element) {
              meta.utRepos = meta.utRepos.concat(ut_element);
            }
            else {
              meta.utRepos = []
            }
            meta.buildRepos = meta.buildRepos.concat(build_repos_element);

          }
        }
      });
      return meta
    },
    deploySubtaskInfo(stages) {
      let meta = {
        status: '',
        serviceLists: []
      };
      stages.forEach(stage => {
        if (stage.type === 'deploy') {
          meta.status = stage.status;
          for (const sub_task in stage.sub_tasks) {
            const deploy_element = stage.sub_tasks[sub_task];
            meta.serviceLists.push(deploy_element);
          }
        }
      });
      return meta
    },
    testSubtaskInfo(task) {
      const workflowName = task.workflow_args.workflow_name;
      const templateName = task.workflow_args.product_tmpl_name;
      const taskId = task.task_id;
      let meta = {
        status: '',
        integration_test: { status: '', builds: [], report_url: '', report_ready: false },
        performance_test: { status: '', builds: [], report_url: '', report_ready: false },
        security_scan: { status: '', results: [] }
      };
      task.stages.forEach(stage => {
        if (stage.type === 'testingv2') {
          meta.status = stage.status;
          for (const key in stage.sub_tasks) {
            if (stage.sub_tasks.hasOwnProperty(key) && stage.sub_tasks[key].job_ctx.test_type === 'function') {
              const testJobName = workflowName + '-' + taskId + '-' + stage.sub_tasks[key].test_name;
              const testModuleName = stage.sub_tasks[key].test_module_name;
              meta.integration_test.test_name = stage.sub_tasks[key].test_name;
              meta.integration_test.status = stage.sub_tasks[key].status;
              meta.integration_test.builds = stage.sub_tasks[key].job_ctx.builds;
              meta.integration_test.report_ready = stage.sub_tasks[key].report_ready;
              meta.integration_test.report_url = (`/v1/projects/detail/${templateName}/pipelines/multi/testcase/${workflowName}/${taskId}/test/${testModuleName}/${testJobName}/case?is_workflow=1&service_name=${testModuleName}&test_type=function`);
            }
          }

          for (const key in stage.sub_tasks) {
            if (stage.sub_tasks.hasOwnProperty(key) && stage.sub_tasks[key].job_ctx.test_type === 'performance') {
              const testJobName = workflowName + '-' + taskId + '-' + stage.sub_tasks[key].test_name;
              const testModuleName = stage.sub_tasks[key].test_module_name;
              meta.performance_test.test_name = stage.sub_tasks[key].test_name;
              meta.performance_test.builds = stage.sub_tasks[key].job_ctx.builds;
              meta.performance_test.status = stage.sub_tasks[key].status;
              meta.performance_test.report_ready = stage.sub_tasks[key].report_ready;
              meta.performance_test.report_url = (`/v1/projects/detail/${templateName}/pipelines/multi/testcase/${workflowName}/${taskId}/test/${testModuleName}/${testJobName}/case?is_workflow=1&service_name=${testModuleName}&test_type=performance`);
            }
          }

        }
        if (stage.type === 'security') {
          meta.security_scan.status = stage.status;
          for (const key in stage.sub_tasks) {
            meta.security_scan.results.push({
              image_name: stage.sub_tasks[key].image_name,
              all: stage.sub_tasks[key].summary ? stage.sub_tasks[key].summary.Total : '-',
              critical: stage.sub_tasks[key].summary ? stage.sub_tasks[key].summary.Critical : '-',
              high: stage.sub_tasks[key].summary ? stage.sub_tasks[key].summary.High : '-',
              medium: stage.sub_tasks[key].summary ? stage.sub_tasks[key].summary.Medium : '-',
              low: stage.sub_tasks[key].summary ? stage.sub_tasks[key].summary.Low : '-'
            });
          }
        }
      });
      return meta
    },
    distributeSubtaskInfo(stages) {
      let meta = {
        status: '',
        releaseImages: []
      };
      stages.forEach(stage => {
        if (stage.type === 'release_image') {
          meta.status = stage.status;
          for (const sub_task in stage.sub_tasks) {
            const release_element = stage.sub_tasks[sub_task];
            meta.releaseImages.push(release_element);
          }
        }
      });
      return meta
    },
  },
  props: {
    productTasks: {
      type: Object,
      required: true
    },
    expandId: {
      type: Number,
      required: true
    }
  },
  created() {
    this.taskDetailExpand[this.expandId] = true;
  },
}
</script>
<style lang="less">
.product-status-container {
  position: relative;
  margin-left: 0px;
  margin-right: 0px;
  .progress-header {
    box-shadow: 1px 0px 10px -5px rgba(0, 0, 0, 0.3);
    margin-bottom: 8px;
    .progress-header-view {
      background: #fff;
      border-bottom: 1px solid #eaeaea;
      display: flex;
      padding: 10px 13px 10px 13px;
      margin-top: 0;
      margin-bottom: 0;
      list-style: none;
      font-size: 14px;
      min-height: 60px;
      .status-view {
        flex-grow: 0;
        flex-shrink: 0;
        flex-basis: 160px;
        .status {
          width: 114px;
          height: 31px;
          border-radius: 50px;
          color: #fff;
          font-weight: bold;
          font-size: 13px;
          line-height: 30px;
          margin-left: 15px;
          text-align: center;
          position: relative;
          padding-left: 15px;
          padding-right: 15px;
          bottom: -10px;
          margin-right: 8px;
          transition: width 100ms ease;
          &.failed {
            background-color: #ff1949;
          }
          &.running {
            background-color: #1989fa;
          }
          &.pending {
            background-color: #606266;
          }
        }
      }
      .info-view {
        flex: 1 1 auto;
        width: calc(100% - 600px);
        padding-right: 18px;
        padding-left: 20px;
        display: flex;
        .spec {
          display: flex;
          align-items: center;
          width: 100%;
          span {
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            max-width: 45%;
            label {
              font-size: 14px;
              font-weight: bold;
              color: #a3a3a3;
              line-height: 18px;
            }
            .workflow-name {
              color: #1989fa;
              font-size: 16px;
              line-height: 16px;
            }
          }
        }
        .stages-tag {
          display: flex;
          align-items: center;
          width: 100%;
          .stage {
            margin-right: 10px;
          }
        }
        .basic-info {
          flex: 0 0 19%;
          align-items: center;
          position: relative;
          .time,
          .author {
            margin: 6px 0;
            color: #666;
            font-size: 14px;
          }
        }
      }
      .operation-view {
        display: flex;
        align-content: center;
        align-items: center;
        justify-content: flex-end;
        span {
          margin-right: 25px;
          font-size: 20px;
        }
        .icon {
          cursor: pointer;
          &.delete {
            color: #ff1949;
          }
          &.view-detail {
            color: #1989fa;
          }
        }
      }
    }
  }
  .stages {
    display: flex;
    flex-wrap: nowrap;
    overflow-x: auto;
    margin: 25px 0px 0px;
    padding-bottom: 35px;
    .stage {
      width: 25%;
      background: -webkit-gradient(
        linear,
        right top,
        left top,
        from(rgba(150, 150, 150, 0.1)),
        color-stop(56.91%, rgba(0, 0, 0, 0))
      );
      background: linear-gradient(
        270deg,
        rgba(150, 150, 150, 0.1) 0%,
        rgba(0, 0, 0, 0) 56.91%
      );
      padding: 11px 30px 20px 40px;
      position: relative;
      overflow: hidden;
      .line.first {
        border-top: none;
        border-top-right-radius: 0px;
      }
      .line.first::before {
        content: " ";
        display: inline-block;
        width: 7px;
        height: 7px;
        border-radius: 5px;
        background-color: #ccc;
        position: absolute;
        top: 0;
        right: -4px;
      }
      .line {
        position: absolute;
        top: 40px;
        bottom: 10px;
        border-right: 1px solid #ccc;
        left: -13px;
        width: 34px;
        border-top: 1px solid #ccc;
        border-top-right-radius: 7px;
      }
      .stage-header {
        background-color: #fff;
        overflow: hidden;
        margin-bottom: 20px;
        box-shadow: 0px 4px 12px 0px rgba(0, 0, 0, 0.13);
        filter: progid:DXImageTransform.Microsoft.dropshadow(OffX=0px, OffY=4px, Color='#21000000');
        display: flex;
        align-items: center;
        justify-content: flex-start;
        padding-left: 10px;
        padding-right: 10px;
        min-height: 62px;
        .stage-header-title {
          width: 50%;
          padding-right: 12px;
        }
        .stage-title {
          text-align: left;
          color: #000;
          font-size: 14px;
          line-height: 1.4;
          text-transform: uppercase;
          font-weight: bold;
          margin: 0;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
      }
      .stage-header.stage-header-empty-status {
        padding-left: 20px;
      }
      .stage-header > * {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        padding-top: 15px;
        padding-bottom: 15px;
      }
      .steps {
        margin: 0;
        padding: 0;
      }
      .step {
        background-color: #fff;
        display: block;
        -webkit-box-shadow: 0 2px 20px 0 rgba(0, 0, 0, 0.03);
        box-shadow: 0 2px 20px 0 rgba(0, 0, 0, 0.03);
        text-overflow: ellipsis;
        overflow: hidden;
        margin-bottom: 20px;
        border-left: 5px solid #ccc;
        padding: 15px 10px 15px 10px;
        &.failed {
          border-left-color: #ff1949;
        }
        &.running {
          border-left-color: #1989fa;
          animation: blink 1.6s infinite;
        }
        &.passed {
          border-left-color: #67c23a;
        }
        &.pending {
          border-left-color: #606266;
        }
      }

      .list-unstyled {
        padding-left: 0;
        list-style: none;
      }
      .cf-steps-list {
        .step {
          display: flex;
          cursor: pointer;
        }
        .step::before {
          content: " ";
          display: inline-block;
          background-color: #ccc;
          width: 15px;
          height: 15px;
          border-radius: 50%;
          left: 14px;
          margin-top: 4px;
          position: absolute;
        }
        .running::before {
          background-color: #1989fa;
        }
        .passed::before {
          background-color: #67c23a;
        }
        .failed::before {
          background-color: #ff1949;
        }
        .step-data {
          min-width: 0;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          -webkit-box-flex: 1;
          -ms-flex-positive: 1;
          flex-grow: 1;
          min-width: 0;
          position: relative;
          .icon {
            float: left;
            margin-right: 10px;
            width: 30px;
            margin-top: 1px;
          }
          .step-description {
            font-size: 13px;
            color: #606266;
            line-height: 18px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            padding-right: 15px;
          }
          .step-type {
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            font-size: 12px;
            color: #999;
            font-weight: bold;
          }
        }
      }
    }
  }
}
</style>