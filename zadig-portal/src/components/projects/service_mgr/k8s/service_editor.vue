<template>
  <div
       class="pipeline-workflow__column pipeline-workflow__column_left pipeline-workflow__column--w100p">
    <div class="white-box-with-shadow">
      <div v-if="showNoContent.show"
           class="no-content">
        <img src="@assets/icons/illustration/editor_nodata.svg"
             alt="">
        <p>{{showNoContent.msg}}</p>
      </div>
      <template v-else>
        <div class="white-box-with-shadow__content">
          <div class="row cf-pipeline-yml-build__wrapper">
            <div class="cf-pipeline-yml-build__editor cf-pipeline-yml-build__editor_inline">
              <div v-if="serviceType === 'k8s'"
                   class="cf-pipeline-yml-build__editor-wrapper">
                <div class="shared-service-checkbox">
                  <el-checkbox v-model="service.visibility"
                               true-label="public"
                               :disabled="service.product_name !== projectName"
                               @change="updateTemplatePermission"
                               false-label="private">共享服务
                    <el-tooltip effect="dark"
                                placement="top">
                      <div slot="content">共享服务可在其它项目的服务编排中使用</div>
                      <i class="el-icon-question"></i>
                    </el-tooltip>
                  </el-checkbox>
                </div>
                <codemirror style="height:100%;width:100%"
                            ref="myCm"
                            :value="service.yaml"
                            :options="cmOptions"
                            @ready="onCmReady"
                            @focus="onCmFocus"
                            @input="onCmCodeChange">
                </codemirror>
              </div>
            </div>
          </div>

        </div>
        <div v-if="info.message"
             class="yaml-errors__container yaml-errors__accordion-opened">
          <ul class="yaml-errors__errors-list yaml-infos__infos-list">
            <li class="yaml-errors__errors-list-item yaml-infos__infos-list-item">
              <div class="yaml-errors__errors-list-item-text">{{info.message}}</div>
            </li>
          </ul>
        </div>
        <div v-if="errors.length > 0"
             class="yaml-errors__container yaml-errors__accordion-opened">
          <ul class="yaml-errors__errors-list">
            <li v-for="(error,index) in errors"
                :key="index"
                class="yaml-errors__errors-list-item">
              <div class="yaml-errors__errors-list-item-counter"> {{index+1}} </div>
              <div class="yaml-errors__errors-list-item-text">{{error.message}}</div>
            </li>
          </ul>
        </div>
        <div v-if="!hideSave"
             class="controls__wrap">
          <div class="controls__right">
            <el-button type="primary"
                       size="small"
                       class="save-btn"
                       :disabled="disabledSave"
                       @click="updateService"
                       plain>保存</el-button>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
<script>
import jsyaml from 'js-yaml';
import { debounce } from 'lodash';
import { codemirror } from 'vue-codemirror';
import 'codemirror/lib/codemirror.css';
import 'codemirror/mode/yaml/yaml.js';
import 'codemirror/theme/xq-light.css';
import 'codemirror/addon/scroll/annotatescrollbar.js';
import 'codemirror/addon/search/matchesonscrollbar.js';
import 'codemirror/addon/search/matchesonscrollbar.css';
import 'codemirror/addon/search/match-highlighter.js';
import 'codemirror/addon/search/jump-to-line.js';

import 'codemirror/addon/dialog/dialog.js';
import 'codemirror/addon/dialog/dialog.css';
import 'codemirror/addon/search/searchcursor.js';
import 'codemirror/addon/search/search.js';
import { validateYamlAPI, updateServicePermissionAPI, serviceTemplateAPI } from '@api';
export default {
  props: {
    serviceInTree: {
      type: Object,
      required: true
    },
    serviceCount: {
      type: Number,
      required: false,
      default: 0
    }
  },
  data() {
    return {
      // codemirror options
      cmOptions: {
        tabSize: 5,
        readOnly: false,
        mode: 'text/yaml',
        lineNumbers: true,
        line: true,
        collapseIdentical: true,
      },
      errors: [],
      info: { message: '' },
      service: {
      },
      showNoContent: {
        show: false,
        msg: ''
      },
      stagedYaml: {},
    }
  },
  methods: {
    getService(val) {
      const serviceName = val ? val.service_name : this.serviceName;
      const projectName = val.product_name;
      const serviceType = val.type;
      this.service.yaml = '';
      if (val && serviceType) {
        serviceTemplateAPI(serviceName, serviceType, projectName).then(res => {
          this.service = res;
          if (this.$route.query.kind) {
            this.jumpToWord(`kind: ${this.$route.query.kind}`);
          }
        });
      }
    },
    updateTemplatePermission() {
      if (this.serviceInTree.status === 'added') {
        updateServicePermissionAPI(
          this.service
        ).then(response => {
          this.$emit('onRefreshService');
          this.$emit('onRefreshSharedService');
          if (this.service.visibility === 'public') {
            this.$message.success('设置服务共享成功');
          }
          else if (this.service.visibility === 'private') {
            this.$message.success('服务已取消共享');
          }
        });
      }
    },
    updateService() {
      const projectName = this.projectName;
      const serviceName = this.service.service_name;
      const visibility = this.service.visibility ? this.service.visibility : 'private';
      const yaml = this.service.yaml;
      const payload = {
        product_name: projectName,
        service_name: serviceName,
        visibility: visibility,
        type: 'k8s',
        yaml: yaml,
        source: 'spock'
      };
      this.$emit('onUpdateService', payload);
    },
    onCmCodeChange: debounce(function (newCode) {
      this.errors = [];
      this.service.yaml = newCode;
      if (this.service.yaml) {
        this.validateYaml(newCode);
        if (this.service.status === 'named') {
          this.stagedYaml[this.service.service_name] = newCode;
        }
      }
    }, 100),
    validateYaml(code) {
      const payload = this.service;
      validateYamlAPI(payload).then((res) => {
        if (res && res.length > 0) {
          this.errors = res;
        }
        else if (res && res.length === 0) {
          this.errors = [];
          this.kindParser(payload.yaml);
        }
      })
    },
    kindParser(yaml) {
      let yamlJsonArray = yaml.split('---').filter(element => {
        return element.indexOf('kind') > 0;
      }).map(element => {
        return jsyaml.load(element);
      });
      this.$emit('onParseKind', {
        service_name: this.service.service_name,
        payload: yamlJsonArray.filter((item) => {
          if (item) {
            return item;
          }
        })
      });
    },
    jumpToWord(word) {
      this.$nextTick(() => {
        const result = this.codemirror.showMatchesOnScrollbar(word);
        if (result.matches.length > 0) {
          const line = result.matches[0];
          this.codemirror.setSelection(line.from, line.to);
          this.codemirror.scrollIntoView({ from: line.from, to: line.to }, 200);
        }
      })
    },
    onCmReady(cm) {
    },
    onCmFocus(cm) {
    },
    editorFocus() {
      this.codemirror.focus();
    }
  },
  computed: {
    codemirror() {
      return this.$refs.myCm.codemirror;
    },
    projectName() {
      return this.$route.params.project_name;
    },
    serviceType() {
      return this.serviceInTree.type;
    },
    serviceName() {
      return this.serviceInTree.service_name
    },
    disabledSave() {
      return (this.errors.length > 0) ? true : false;
    },
    hideSave() {
      if (this.service.source === 'gitlab' || this.service.source === 'github' || this.service.source === 'gerrit' || (this.service.visibility === 'public' && this.service.product_name !== this.projectName)) {
        return true;
      }
      else {
        return false;
      }
    }
  },
  watch: {
    'serviceCount': {
      handler(val, old_val) {
        if (val === 0) {
          this.showNoContent = {
            show: true,
            msg: '暂无服务,创建服务请在左侧栏点击「添加服务」按钮'
          }
        }
        else if (val > 0 && this.showNoContent.msg === '暂无服务,创建服务请在左侧栏点击「添加服务」按钮') {
          this.showNoContent.show = false;
        }
      },
      immediate: true
    },

    'serviceInTree': {
      handler(val, old_val) {
        if (this.serviceCount > 0) {
          if (val.visibility === 'public' && val.product_name !== this.projectName) {
            this.info = {
              message: '信息：其它项目的共享服务，不支持在本项目中编辑，编辑器为只读模式'
            }
          }
          else if (val.product_name === this.projectName && val.source && val.source !== 'spock') {
            this.info = {
              message: '信息：当前服务为仓库管理服务，编辑器为只读模式'
            }
          }
          else {
            this.info = {
              message: ''
            }
          }
          if (val.status === 'added') {
            this.getService(val);
            if (val.source === 'gitlab' || val.source === 'gerrit' || val.source === 'github' || (val.visibility === 'public' && val.product_name !== this.projectName)) {
              this.cmOptions.readOnly = true;
            }
            else {
              this.cmOptions.readOnly = false;
            }
          }
          else if (val.status === 'named') {
            this.service = {
              yaml: '',
              service_name: val.service_name,
              status: 'named'
            }
            this.cmOptions.readOnly = false;
            if (this.stagedYaml[val.service_name]) {
              this.service.yaml = this.stagedYaml[val.service_name];
            }
            this.editorFocus();
          }
        }
      },
      immediate: true
    }
  },
  components: {
    codemirror
  },
}
</script>
<style lang="less">
@import "~@assets/css/component/service-editor.less";
</style>