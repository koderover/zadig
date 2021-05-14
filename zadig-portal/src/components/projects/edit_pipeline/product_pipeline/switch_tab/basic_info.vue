<template>
  <div class="product-basic-info">
    <el-card class="box-card">
      <div>
        <el-form :model="pipelineInfo"
                 :rules="rules"
                 ref="pipelineInfo"
                 label-position="top"
                 label-width="80px">
          <el-row>
            <el-col :span="5">
              <el-form-item prop="name"
                            label="工作流名称">
                <el-input v-model="pipelineInfo.name"
                          :disabled="editMode"
                          style="width:80%"
                          placeholder="请输入工作流名称"></el-input>
              </el-form-item>
            </el-col>
            <el-col :span="5">
              <el-form-item prop="product_tmpl_name"
                            label="选择项目">
                <el-select v-model="pipelineInfo.product_tmpl_name"
                           style="width:80%"
                           @change="getProductEnv(pipelineInfo.product_tmpl_name)"
                           placeholder="请选择项目"
                           :disabled="$route.query.projectName || editMode"
                           filterable>
                    <el-option v-for="pro in projects" :key="pro.value" :label="pro.label"
                               :value="pro.value">
                    </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="5">
              <el-form-item prop="env_name">
                <template slot="label">
                  <span>指定环境</span>
                  <el-tooltip effect="dark"
                              content="支持工作流默认部署到某个环境"
                              placement="top">
                    <i class="pointer el-icon-question"></i>
                  </el-tooltip>
                </template>
                <el-select v-model="pipelineInfo.env_name"
                           style="width:80%"
                           placeholder="请选择环境"
                           clearable
                           filterable>
                    <el-option :label="env.env_name" v-for="(env,index) in filteredEnvs" :key="index"
                               :value="env.env_name">
                    </el-option>
                    <el-option v-if="filteredEnvs.length===0"
                               label=""
                               value="">
                      <router-link style="color:#909399"
                                   :to="`/v1/projects/detail/${pipelineInfo.product_tmpl_name}/envs/create`">
                        {{`(环境不存在，点击创建环境)`}}
                      </router-link>
                    </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="5">
              <el-form-item prop="reset_image"
                            label="镜像版本回退">
                <el-checkbox v-model="pipelineInfo.reset_image"></el-checkbox>
              </el-form-item>
            </el-col>
            <el-col :span="4">
              <el-form-item prop="is_parallel">
                <template slot="label">
                  <span>并发运行 </span>
                  <el-tooltip effect="dark"
                              content="当更新不同服务时触发该工作流，产生的多个任务将会并发执行以提升构建、部署、测试效率"
                              placement="top">
                    <i class="pointer el-icon-question"></i>
                  </el-tooltip>
                </template>
                <el-checkbox v-model="pipelineInfo.is_parallel"></el-checkbox>
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="描述">
            <el-input type="textarea"
                      style="width:100%"
                      v-model="pipelineInfo.description"></el-input>
          </el-form-item>
        </el-form>
      </div>
    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus';
import { templatesAPI, listProductAPI } from '@api';

export default {
  data() {
    return {
      projects: [],
      productList: [],
      rules: {
        name: [
          {
            type: 'string',
            required: true,
            validator: this.validatePipelineName,
            trigger: 'blur'
          }
        ],
        product_tmpl_name: [
          {
            type: 'string',
            required: true,
            message: '请选择项目',
            trigger: 'blur'
          }
        ]
      }
    };
  },
  methods: {
    validatePipelineName(rule, value, callback) {
      const result = this.$utils.validatePipelineName([], value);
      if (result === true) {
        callback();
      } else {
        callback(new Error(result));
      }
    },
    getProductEnv(projectName) {
      listProductAPI('', projectName).then(res => {
        this.productList = res;
      });
    }
  },
  props: {
    pipelineInfo: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    filteredEnvs() {
      const currentProject = this.pipelineInfo.product_tmpl_name
      if (currentProject !== '') {
        return this.productList.filter(element => {
          return element.product_name === currentProject
        });
      }
      else {
        return [];
      }
    }
  },
  created() {
    if (this.$route.query.projectName) {
      this.pipelineInfo.product_tmpl_name = this.$route.query.projectName;
    }

    if (!this.$route.query.projectName && !this.editMode) {
      templatesAPI().then(res => {
        this.projects = res;
      });
    }
    const projectName = this.pipelineInfo.product_tmpl_name;
    bus.$on('check-tab:basicInfo', () => {
      this.$refs.pipelineInfo.validate(valid => {
        bus.$emit('receive-tab-check:basicInfo', valid);
      });
    });
    listProductAPI('', projectName).then(res => {
      this.productList = res;
    });
  },
  beforeDestroy() {
    bus.$off('check-tab:basicInfo');
  },
};
</script>

<style lang="less">
.product-basic-info {
  .pointer{
    cursor: pointer;
  }
  .box-card {
    .el-card__header {
      text-align: center;
    }
    .el-form {
      .el-form-item {
        margin-bottom: 5px;
      }
      .pipe-schedule-container {
        .el-form-item__content {
          margin-left: 0px !important;
        }
      }
    }
    .divider {
      height: 1px;
      background-color: #dfe0e6;
      margin: 13px 0;
      width: 100%;
    }
  }
}
</style>
