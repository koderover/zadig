<template>
  <div class="service-details-container">
    <div class="info-card">
      <div class="info-header">
        <span>基本信息</span>
      </div>
      <el-row :gutter="0"
              class="info-body">
        <el-col :span="12"
                class="WAN">
          <div class="addr-title title">
            外网访问
          </div>
          <div class="addr-content">
            <template v-if="allHosts.length > 0">
              <div v-for="host of allHosts"
                   :key="host.host">
                <a :href="`http://${host.host}`"
                   target="_blank">{{ host.host }}</a>
              </div>
            </template>
            <div v-else>无</div>
          </div>
        </el-col>
        <el-col :span="12"
                class="LAN">
          <div class="addr-title title">
            内网访问
          </div>
          <div class="addr-content">
            <template v-if="allEndpoints.length > 0">
              <div v-for="(ep,index) in allEndpoints"
                   :key="index">
                <span>{{ `${ep.service_name}:${ep.service_port}` }}</span>
                <el-popover v-if="index===0"
                            placement="bottom"
                            popper-class="ns-pop"
                            trigger="hover">
                  <span class="title">同 NS 访问：</span>
                  <div v-for="(sameNs,indexSame) in allEndpoints"
                       :key="indexSame+'same'">
                    <span class="addr">{{ `${sameNs.service_name}:${sameNs.service_port}` }}</span>
                    <span v-clipboard:copy="`${sameNs.service_name}:${sameNs.service_port}`"
                          v-clipboard:success="copyCommandSuccess"
                          v-clipboard:error="copyCommandError"
                          class="copy-btn el-icon-copy-document">
                    </span>
                  </div>
                  <span class="title">跨 NS 访问：</span>
                  <div v-for="(crossNs,indexCross) in allEndpoints"
                       :key="indexCross+'cross'">
                    <span
                          class="addr">{{ `${crossNs.service_name}.${namespace}:${crossNs.service_port}` }}</span>
                    <span v-clipboard:copy="`${crossNs.service_name}.${namespace}:${crossNs.service_port}`"
                          v-clipboard:success="copyCommandSuccess"
                          v-clipboard:error="copyCommandError"
                          class="copy-btn el-icon-copy-document">
                    </span>
                  </div>
                  <span slot="reference"><i class="show-more el-icon-more"></i></span>
                </el-popover>
              </div>
            </template>
            <div v-else>无</div>
          </div>
        </el-col>
      </el-row>

    </div>

    <div class="info-card">
      <div class="info-header">
        <span>基本操作</span>
      </div>
      <div class="info-body fundamental-ops">
        <template>
          <router-link
                       :to="`/v1/projects/detail/${this.projectName}/envs/detail/${this.serviceName}/config${window.location.search}`">
            <el-button icon="ion-android-open"
                       type="primary"
                       size="small"
                       plain>
              配置管理
            </el-button>
          </router-link>
          <el-button @click="showExport"
                     icon="ion-android-download icon-bold"
                     type="primary"
                     size="small"
                     plain>
            Yaml 导出
          </el-button>
          <router-link
                       :to="`/v1/projects/detail/${originProjectName}/services?service_name=${serviceName}&rightbar=var`">
            <el-button icon="ion-link icon-bold"
                       type="primary"
                       size="small"
                       plain>
              服务管理
            </el-button>
          </router-link>
        </template>
      </div>
    </div>
    <div class="info-card">
      <div class="info-header">
        <span>服务实例</span>
        <el-popover placement="top"
                    trigger="hover">
          <div v-for="(color, status) in statusColorMap"
               :key="status"
               class="stat-tooltip-row">
            <span :class="['tooltip', color]"></span> {{ status }}
          </div>
          <i class="el-icon-question pointer"
             slot="reference"></i>
        </el-popover>
      </div>
      <div class="info-body">
        <template>
          <el-table :data="currentService.scales"
                    row-key="name"
                    :expand-row-keys="expands"
                    @expand-change="expandScale"
                    style="width: 100%">
            <el-table-column width="140"
                             prop="name"
                             label="名称">
              <template slot-scope="scope">
                <span>{{ scope.row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="images"
                             label="镜像">
              <template slot-scope="scope">
                <div v-for="(item,index) of scope.row.images"
                     :key="index"
                     class="image-row">
                  <template v-if="!item.edit">
                    <span class="service-name">{{ item.name }}</span>
                    <el-tooltip effect="light"
                                :content="item.image"
                                placement="top">
                      <span class="service-image">{{splitImg(item.image) }}</span>
                    </el-tooltip>
                    <el-button @click="showEditImage(item)"
                               type="primary"
                               plain
                               size="mini"
                               class="edit-btn"
                               icon="el-icon-edit"
                               circle></el-button>
                  </template>
                  <template v-else>
                    <span class="service-name">{{ item.name }}</span>
                    <el-select v-model="item.image"
                               size="medium"
                               allow-create
                               filterable
                               placeholder="请选择版本"
                               class="select-image">
                      <el-option v-for="(image,index) in imgBucket[item.name]"
                                 :value="image.host+'/'+image.owner+'/'+image.name+':'+image.tag"
                                 :label="image.tag"
                                 :key="index">
                      </el-option>
                    </el-select>
                    <div>
                      <span>
                        <i title="取消"
                           @click="cancelEditImage(item)"
                           class="el-icon-circle-close icon-color icon-color-cancel operation">取消</i>
                      </span>
                      <span>
                        <i title="保存"
                           @click="saveImage(item,scope.row.name,scope.row.type)"
                           class="el-icon-circle-check icon-color icon-color-confirm operation">保存</i>
                      </span>
                    </div>
                  </template>

                </div>
              </template>
            </el-table-column>
            <el-table-column props="replicas"
                             width="125px"
                             label="副本数量">
              <template slot-scope="scope">
                <el-input-number size="mini"
                                 :min="0"
                                 :max="20"
                                 @change="(currentValue)=>{scaleService(scope.row.name,scope.row.type,currentValue)}"
                                 v-model="scope.row.replicas"></el-input-number>
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="220px">
              <template slot-scope="scope">
                <el-button @click="restartService(scope.row.name,scope.row.type)"
                           size="mini">重启实例</el-button>
                <el-button @click="showScaleEvents(scope.row.name,scope.row.type)"
                           size="mini">查看事件</el-button>
              </template>
            </el-table-column>
            <el-table-column label="详情"
                             props="pods"
                             type="expand"
                             width="80px">
              <template slot-scope="scope">
                <div v-if="activePod[scope.$index]"
                     class="info-body pod-container">

                  <span v-for="(pod,index) of scope.row.pods"
                        :key="index"
                        @click="selectPod(pod,scope.$index)"
                        :ref="pod.name"
                        :class="{pod:true, [pod.__color]:true, active: pod===activePod[scope.$index] }">
                  </span>
                  <transition name="fade">
                    <div v-if="activePod[scope.$index].name"
                         class="pod-info">
                      <el-row :class="['pod-row',`indicator-${activePod[scope.$index].name}`, activePod[scope.$index].__color]"
                              data-triangle-offset="5px"
                              ref="pod-row">
                        <el-col :span="12">
                          <span class="title">实例名称：</span>
                          <span class="content">{{ activePod[scope.$index].name }}</span>
                        </el-col>
                        <el-col :span="6">
                          <span class="title">运行时长：</span>
                          <span class="content">{{ activePod[scope.$index].age }}</span>
                        </el-col>
                        <el-col :span="6"
                                class="op-buttons">
                          <el-button @click="restartPod(activePod[scope.$index])"
                                     :disabled="!activePod[scope.$index].canOperate"
                                     size="small">重启</el-button>
                          <el-button @click="showPodEvents(activePod[scope.$index])"
                                     size="small">查看事件</el-button>
                        </el-col>
                      </el-row>
                      <el-row v-for="container of activePod[scope.$index].containers"
                              :key="container.name"
                              :class="['container-row', container.__color]">
                        <el-col :span="12">
                          <div>
                            <span class="title">容器名称：</span>
                            <span class="content">{{ container.name }}</span>
                          </div>
                          <div>
                            <span class="title">当前镜像：</span>
                            <span class="content">{{ container.imageShort }}</span>
                          </div>
                          <div v-if="container.message">
                            <span class="title">错误信息：</span>
                            <span class="content">{{ container.message }}</span>
                          </div>
                        </el-col>
                        <el-col :span="7">
                          <div>
                            <span class="title">状态：</span>
                            <span class="content">{{ container.status }}</span>
                          </div>
                          <div v-if="container.startedAtReadable">
                            <span class="title">启动时间：</span>
                            <span class="content">{{ container.startedAtReadable }}</span>
                          </div>
                        </el-col>
                        <el-col :span="5"
                                class="op-buttons">
                          <el-button @click="showContainerExec(activePod[scope.$index].name,container.name)"
                                     :disabled="!activePod[scope.$index].canOperate"
                                     icon="iconfont iconTerminal"
                                     size="small"> 调试</el-button>
                          <el-button @click="showContainerLog(activePod[scope.$index].name,container.name)"
                                     :disabled="!activePod[scope.$index].canOperate"
                                     icon="el-icon-document"
                                     size="small">实时日志</el-button>
                        </el-col>
                      </el-row>
                    </div>
                  </transition>

                </div>
              </template>
            </el-table-column>
          </el-table>
        </template>
      </div>
    </div>

    <el-dialog :visible.sync="logModal.visible"
               :close-on-click-modal="false"
               width="70%"
               title="容器日志"
               class="log-dialog">
      <span slot="title"
            class="modal-title">
        <span class="unimportant">Pod 名称:</span>
        {{logModal.podName}}
        <span class="unimportant">容器:</span>
        {{logModal.containerName}}
        <i class="el-icon-full-screen screen"
           @click="fullScreen('logModal')"></i>
      </span>
      <containerLog id="logModal"
                    :podName="logModal.podName"
                    :containerName="logModal.containerName"
                    :visible="logModal.visible"></containerLog>
    </el-dialog>

    <el-dialog :visible.sync="execModal.visible"
               width="70%"
               :close-on-click-modal="false"
               title="Pod 调试"
               class="log-dialog">
      <span slot="title"
            class="modal-title">
        <span class="unimportant">Pod 名称:</span>
        {{execModal.podName}}
        <span class="unimportant">容器:</span>
        {{execModal.containerName}}
        <i class="el-icon-full-screen screen"
           @click="fullScreen(execModal.podName +'-debug')"></i>
      </span>
      <xterm-debug :id="execModal.podName +'-debug'"
                   :productName="projectName"
                   :namespace="namespace"
                   :serviceName="serviceName"
                   :containerName="execModal.containerName"
                   :podName="execModal.podName"
                   :visible="execModal.visible"
                   ref="debug"></xterm-debug>
    </el-dialog>

    <el-dialog :visible.sync="exportModal.visible"
               width="70%"
               title="YAML 配置导出"
               class="export-dialog">
      <h1 v-if="exportModal.textObjects.length === 0"
          v-loading="exportModal.loading"
          class="nothing">
        {{ exportModal.loading ? '' : notSupportYaml }}
      </h1>
      <template v-else>
        <div class="op-row expanded">
          <el-button @click="copyAllYAML"
                     plain
                     type="primary"
                     size="medium"
                     class="at-right">复制全部</el-button>
        </div>
        <div v-for="(obj, i) of exportModal.textObjects"
             :key="obj.originalText"
             class="config-viewer">
          <div>
            <div :class="{'op-row': true, expanded: obj.expanded}">
              <el-button @click="toggleYAML(obj)"
                         type="text"
                         icon="el-icon-caret-bottom">
                {{ obj.expanded ? '收起' : '展开' }}
              </el-button>
              <el-button @click="copyYAML(obj, i)"
                         type="primary"
                         plain
                         size="small"
                         class="at-right">复制</el-button>
            </div>
            <editor v-show="obj.expanded"
                    :value="obj.readableText"
                    :options="exportModal.editorOption"
                    @init="editorInit($event, obj)"
                    lang="yaml"
                    theme="tomorrow_night"
                    width="100%"
                    height="800"></editor>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-dialog :visible.sync="eventsModal.visible"
               width="70%"
               title="查看事件"
               class="events-dialog">
      <span slot="title"
            class="modal-title">
        <span class="unimportant">实例名称:</span>
        {{ eventsModal.name }}
      </span>

      <div v-if="eventsModal.data.length === 0"
           class="events-no-data">
        <span class="el-table__empty-text">暂时没有事件</span>
      </div>
      <el-table :data="eventsModal.data"
                v-else>
        <el-table-column prop="message"
                         label="消息"></el-table-column>
        <el-table-column prop="reason"
                         label="状态"
                         width="240"></el-table-column>
        <el-table-column prop="count"
                         label="总数"
                         width="70"></el-table-column>
        <el-table-column prop="firstSeenReadable"
                         label="最早出现于"
                         width="160"></el-table-column>
        <el-table-column prop="lastSeenReadable"
                         label="最近出现于"
                         width="160"></el-table-column>
      </el-table>

    </el-dialog>

  </div>
</template>

<script>
import containerLog from '../service_detail/container_log.vue';
import { restartPodAPI, restartServiceAPI, scaleServiceAPI, scaleEventAPI, podEventAPI, exportYamlAPI, imagesAPI, updateServiceImageAPI } from '@api';
import moment from 'moment';
import aceEditor from 'vue2-ace-bind';
import 'brace/mode/yaml';
import 'brace/theme/xcode';
import 'brace/theme/tomorrow_night';
import 'brace/ext/searchbox';
import { getServiceInfo } from '@api';
import bus from '@utils/event_bus';
import { fullScreen } from '@/utilities/full_screen'
export default {
  data() {
    return {
      fullScreen,
      currentService: {
        ingress: [],
        pods: [],
        service_name: '',
        service_endpoints: []
      },
      expands: [],
      activePod: {},
      imgBucket: {},
      logModal: {
        visible: false,
        podName: null,
        containerName: null
      },
      execModal: {
        visible: false,
        podName: null,
        containerName: null
      },
      exportModal: {
        textObjects: [],
        visible: false,
        loading: false,
        editorOption: {
          showLineNumbers: true,
          showFoldWidgets: true,
          showGutter: true,
          displayIndentGuides: true,
          showPrintMargin: false,
          readOnly: true,
          tabSize: 2,
          maxLines: Infinity
        }
      },
      eventsModal: {
        visible: false,
        name: '',
        data: []
      },
      window: window,
      statusColorMap: {
        running: 'green',
        pending: 'yellow',
        failed: 'red',
        unstable: 'red',
        unknown: 'purple',
        terminating: 'gray'
      },
    };
  },

  computed: {
    allHosts() {
      if (this.currentService.ingress) {
        return this.currentService.ingress.reduce((carry, ing) => carry.concat(ing.host_info), []);
      }
      else {
        return []
      }
    },
    allEndpoints() {
      if (this.currentService.service_endpoints) {
        return this.currentService.service_endpoints.reduce(
          (carry, point) => {
            if (point.endpoints) {
              return carry.concat(point.endpoints
              )
            }
          }, []
        );
      }
      else {
        return [];
      }
    },
    projectName() {
      return (this.$route.params.project_name ? this.$route.params.project_name : this.$route.query.projectName);
    },
    originProjectName() {
      return (this.$route.query.originProjectName ? this.$route.query.originProjectName : this.projectName);
    },
    serviceName() {
      return this.$route.params.service_name;
    },
    envName() {
      return this.$route.query.envName;
    },
    envSource() {
      return this.$route.query.envSource || '';
    },
    notSupportYaml() {
      return '没有找到数据'
    },
    namespace() {
      return this.$route.query.namespace;
    }
  },

  methods: {
    copyCommandSuccess(event) {
      this.$message({
        message: '地址已成功复制到剪贴板',
        type: 'success'
      });
    },
    copyCommandError(event) {
      this.$message({
        message: '地址复制失败',
        type: 'error'
      });
    },
    splitImg(img) {
      if (img) {
        if (img.includes('/')) {
          const length = img.split('/').length;
          return img.split('/')[length - 1];
        }
        else {
          return img;
        }
      }
    },
    fetchServiceData() {
      const projectName = this.projectName;
      const serviceName = this.serviceName;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      getServiceInfo(projectName, serviceName, envName, envType).then((res) => {
        if (res.scales) {
          if (res.scales.length > 0 && res.scales[0].pods.length > 0) {
            this.expands = [res.scales[0].name];
            this.$set(this.activePod, 0, res.scales[0].pods[0]);
          }
          res.scales.forEach(scale => {
            scale.pods.forEach(pod => {
              pod.status = pod.status.toLowerCase();
              pod.__color = this.statusColorMap[pod.status];
              pod.canOperate = !(pod.status in {
                pending: 1,
                terminating: 1
              });
              pod.containers.forEach(con => {
                con.edit = false;
                con.image2Apply = con.image;
                con.imageShort = con.image.split('/').pop();
                con.status = con.status.toLowerCase();
                con.__color = this.statusColorMap[con.status];
                con.startedAtReadable = con.started_at
                  ? moment(con.started_at, 'X').format('YYYY-MM-DD HH:mm:ss')
                  : '';
              });
            });
          });
        }
        else {
          res.scales = [];
        }
        this.currentService = res;
      });
    },
    expandScale(row, rows) {
      const rowIndex = this.currentService.scales.indexOf(row);
      const pods = row.pods;
      if (pods.length > 0) {
        this.$set(this.activePod, rowIndex, pods[0]);
      }
    },
    getImages(containerName) {
      const containerNames = [containerName];
      imagesAPI(containerNames).then(res => {
        this.$set(this.imgBucket, containerName, res);
      });
    },
    showEditImage(item) {
      this.$set(item, 'edit', true);
      this.getImages(item.name);
    },
    cancelEditImage(item) {
      this.$set(item, 'edit', false);
    },
    saveImage(item, scaleName, typeUppercase) {
      const envType = '';
      const type = typeUppercase.toLowerCase();
      item.edit = false;
      this.$message({
        message: '正在更新镜像',
        duration: 3500
      });
      let payload = {
        product_name: this.projectName,
        service_name: this.serviceName,
        container_name: item.name,
        name: scaleName,
        image: item.image
      };
      if (this.envName) {
        payload.env_name = this.envName;
      }
      updateServiceImageAPI(payload, type, envType).then((res) => {
        this.fetchServiceData();
        this.$message({
          message: '镜像更新成功',
          type: 'success'
        });
      })
    },
    restartService(scaleName, type) {
      const projectName = this.projectName;
      const serviceName = this.serviceName;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      restartServiceAPI(projectName, serviceName, envName, scaleName, type, envType).then((res) => {
        this.fetchServiceData();
        this.$message({
          message: '重启实例成功',
          type: 'success'
        });
      })
    },
    scaleService(scaleName, type, scaleNumber) {
      const projectName = this.projectName;
      const serviceName = this.serviceName;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      scaleServiceAPI(projectName, serviceName, envName, scaleName, scaleNumber, type, envType).then((res) => {
        this.fetchServiceData();
        this.$message({
          message: '伸缩服务成功',
          type: 'success'
        });
      })
    },
    selectPod(target, index) {
      this.activePod[index] = target;
      // https://stackoverflow.com/questions/5041494
      const sheet = document.styleSheets[0];
      const len = sheet.cssRules.length;
      sheet.insertRule(`.service-details-container .pod-info .indicator-${target.name}::before
          { left: ${this.$refs[target.name][0].offsetLeft + 4}px!important; }`, len);
      sheet.insertRule(`.service-details-container .pod-info
          { top: ${this.$refs[target.name][0].offsetTop + 30}px!important; }`, len + 1);

    },
    restartPod(pod) {
      const ownerQuery = this.envName ? `&envName=${this.envName}` : '';
      const projectName = `${this.projectName}${ownerQuery}`;
      const podName = pod.name;
      const envType = '';
      restartPodAPI(podName, projectName, envType).then((res) => {
        this.fetchServiceData();
        this.$message({
          message: '重启成功',
          type: 'success'
        });
      })
    },
    showContainerLog(pod_name, container_name) {
      this.logModal.visible = true;
      this.logModal.podName = pod_name;
      this.logModal.containerName = container_name;
    },
    showContainerExec(pod_name, container_name) {
      this.execModal.visible = true;
      this.execModal.podName = pod_name;
      this.execModal.containerName = container_name;
    },
    showExport() {
      const projectName = this.projectName;
      const serviceName = this.serviceName;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      this.exportModal.visible = true;
      this.exportModal.textObjects = [];
      this.exportModal.loading = true;
      exportYamlAPI(projectName, serviceName, envName, envType).then((res) => {
        this.exportModal.textObjects = res.map(txt => ({
          originalText: txt,
          readableText: txt.replace(/\\n/g, '\n').replace(/\\t/g, '\t'),
          expanded: true,
          editor: null
        }));
        this.exportModal.loading = false;
      })
    },
    editorInit(e, obj) {
      obj.editor = e;
    },
    copyYAML(obj, i) {
      const e = obj.editor;
      e.setValue(obj.originalText);
      e.focus();
      e.selectAll();
      if (document.execCommand('copy')) {
        this.$message.success('复制成功');
      } else {
        this.$message.error('复制失败');
      }
      e.setValue(obj.readableText);
    },
    toggleYAML(obj) {
      obj.expanded = !obj.expanded;
    },
    copyAllYAML() {
      const textArea = document.createElement('textarea');
      textArea.value = this.exportModal.textObjects
        .map(obj => obj.originalText)
        .join('\n\n---\n\n');
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();

      if (document.execCommand('copy')) {
        this.$message.success('复制成功');
      } else {
        this.$message.error('复制失败');
      }
      document.body.removeChild(textArea);
    },
    showPodEvents(pod) {
      const projectName = this.projectName;
      const podName = pod.name;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      this.eventsModal.visible = true;
      podEventAPI(projectName, podName, envName, envType).then((res) => {
        this.eventsModal.data = res.map(row => {
          row.firstSeenReadable = moment(row.first_seen, 'X').format('YYYY-MM-DD HH:mm');
          row.lastSeenReadable = moment(row.last_seen, 'X').format('YYYY-MM-DD HH:mm');
          return row;
        });
      })
    },
    showScaleEvents(scaleName, type) {
      const projectName = this.projectName;
      const envName = this.envName ? this.envName : '';
      const envType = '';
      this.eventsModal.visible = true;
      this.eventsModal.name = scaleName;
      scaleEventAPI(projectName, scaleName, envName, type, envType).then((res) => {
        this.eventsModal.data = res.map(row => {
          row.firstSeenReadable = moment(row.first_seen, 'X').format('YYYY-MM-DD HH:mm');
          row.lastSeenReadable = moment(row.last_seen, 'X').format('YYYY-MM-DD HH:mm');
          return row;
        });
      })
    }
  },
  created() {
    this.fetchServiceData();
    bus.$emit(`set-topbar-title`,
      {
        title: '',
        breadcrumb: [
          { title: '项目', url: '/v1/projects' },
          { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` },
          { title: '集成环境', url: `/v1/projects/detail/${this.projectName}/envs/detail` },
          { title: this.envName, url: `/v1/projects/detail/${this.projectName}/envs/detail?envName=${this.envName}` },
          { title: this.serviceName, url: `` }
        ]
      });
  },
  destroyed() { },
  components: {
    containerLog,
    editor: aceEditor
  }
};

function myScroll(container, toElem) {
  container.scrollTop = toElem.offsetTop;
}
</script>

<style lang="less" scoped>
.screen {
  float: right;
  margin-right: 30px;
  color: #909399;
  cursor: pointer;
  font-size: 20px;
  &:hover {
    color: #409eff;
  }
}

/deep/.el-dialog__headerbtn {
  top: 18px;
  font-size: 20px;
}
@import "~@assets/css/component/service-detail.less";
</style>
