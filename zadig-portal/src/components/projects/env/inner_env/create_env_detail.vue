<template>
  <div class="create-product-detail-container"
       v-loading="loading"
       element-loading-text="正在加载中"
       element-loading-spinner="el-icon-loading">
    <div class="module-title">
      <h1>创建环境</h1>
    </div>
    <div v-if="showEmptyServiceModal"
         class="no-resources">
      <div>
        <img src="@assets/icons/illustration/environment.svg"
             alt="">
      </div>
      <div class="description">
        <p>该环境暂无服务，请点击
          <router-link :to="`/v1/projects/detail/${projectName}/services`">
            <el-button type="primary"
                       size="mini"
                       round
                       plain>项目->服务</el-button>
          </router-link>
          添加服务
        </p>
      </div>
    </div>
    <div v-else>
      <el-form label-width="200px"
               ref="create-env-ref"
               :model="projectConfig"
               :rules="rules">
        <el-form-item label="环境名称："
                      prop="env_name">
          <el-input v-model="projectConfig.env_name"
                    size="small"></el-input>
        </el-form-item>
      </el-form>

      <el-card v-if="projectConfig.vars && projectConfig.vars.length > 0  && !$utils.isEmpty(containerMap)"
               class="box-card-service"
               :body-style="{padding: '0px', margin: '10px 0 0 0'}">
        <div class="module-title">
          <h1>变量列表</h1>
        </div>
        <div class="kv-container">
          <el-table :data="projectConfig.vars"
                    style="width: 100%">
            <el-table-column label="Key">
              <template slot-scope="scope">
                <span>{{ scope.row.key }}</span>
              </template>
            </el-table-column>
            <el-table-column label="Value">
              <template slot-scope="scope">
                <el-input size="small"
                          v-model="scope.row.value"
                          type="textarea"
                          :autosize="{ minRows: 1, maxRows: 4}"
                          placeholder="请输入内容"></el-input>
              </template>
            </el-table-column>
            <el-table-column label="关联服务">
              <template slot-scope="scope">
                <span>{{ scope.row.services?scope.row.services.join(','):'-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作"
                             width="150">
              <template slot-scope="scope">
                <template>
                  <span class="operate">
                    <el-button v-if="scope.row.state === 'unused'"
                               type="text"
                               @click="deleteRenderKey(scope.$index,scope.row.state)"
                               class="delete">移除</el-button>
                    <el-tooltip v-else
                                effect="dark"
                                content="模板中用到的渲染 Key 无法被删除"
                                placement="top">
                      <span class="el-icon-question"></span>
                    </el-tooltip>
                  </span>
                </template>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="addKeyInputVisable"
               class="add-key-container">
            <el-table :data="addKeyData"
                      :show-header="false"
                      style="width: 100%">
              <el-table-column>
                <template slot-scope="scope">
                  <el-form :model="addKeyData[0]"
                           :rules="keyCheckRule"
                           ref="addKeyForm"
                           hide-required-asterisk>
                    <el-form-item label="Key"
                                  prop="key"
                                  inline-message>
                      <el-input size="small"
                                type="textarea"
                                :autosize="{ minRows: 1, maxRows: 4}"
                                v-model="addKeyData[0].key"
                                placeholder="请输入 Key">
                      </el-input>
                    </el-form-item>
                  </el-form>
                </template>
              </el-table-column>
              <el-table-column>
                <template slot-scope="scope">
                  <el-form :model="addKeyData[0]"
                           :rules="keyCheckRule"
                           ref="addValueForm"
                           hide-required-asterisk>
                    <el-form-item label="Value"
                                  prop="value"
                                  inline-message>
                      <el-input size="small"
                                type="textarea"
                                :autosize="{ minRows: 1, maxRows: 4}"
                                v-model="addKeyData[0].value"
                                placeholder="请输入 Value">
                      </el-input>
                    </el-form-item>
                  </el-form>
                </template>
              </el-table-column>
              <el-table-column width="100">
                <template slot-scope="scope">
                  <span style="display: inline-block;margin-bottom:15px">
                    <el-button @click="addRenderKey()"
                               type="text">确认</el-button>
                    <el-button @click="addKeyInputVisable=false"
                               type="text">取消</el-button>
                  </span>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <span class="add-kv-btn">
            <i title="添加"
               @click="addKeyInputVisable=true"
               class="el-icon-circle-plus"> 新增</i>
          </span>
        </div>
      </el-card>
      <div>
        <div style="line-height: 20px;font-size:16px;color: rgb(153, 153, 153);">服务列表</div>
        <template>
          <el-card v-if="!$utils.isEmpty(containerMap)"
                   class="box-card-service"
                   :body-style="{padding: '0px'}">
            <div slot="header"
                 class="clearfix">
              <span class="second-title">
                微服务 K8s YAML 部署
              </span>
              <span class="service-filter">
                快速过滤:
                <el-tooltip class="img-tooltip"
                            effect="dark"
                            placement="top">
                  <div slot="content">智能选择会优先选择最新的容器镜像，如果在 Registry<br />
                    下不存在该容器镜像，则会选择模板中的默认镜像进行填充</div>
                  <i class="el-icon-info"></i>
                </el-tooltip>
                <el-select size="small"
                           class="img-select"
                           v-model="quickSelection"
                           placeholder="请选择">
                  <el-option label="全容器-智能选择镜像"
                             value="latest"></el-option>
                  <el-option label="全容器-全部默认镜像"
                             value="default"></el-option>
                </el-select>
              </span>
            </div>

            <el-form class="service-form"
                     label-width="190px">
              <div class="group"
                   v-for="(typeServiceMap, serviceName) in containerMap"
                   :key="serviceName">
                <el-tag>{{ serviceName }}</el-tag>
                <div class="service">
                  <div v-for="service in typeServiceMap"
                       :key="`${service.service_name}-${service.type}`"
                       class="service-block">

                    <div v-if="service.type==='k8s' && service.containers"
                         class="container-images">
                      <el-form-item v-for="con of service.containers"
                                    :key="con.name"
                                    :label="con.name">
                        <el-select v-model="con.image"
                                   filterable
                                   size="small">
                          <el-option v-for="img of imageMap[con.name]"
                                     :key="`${img.name}-${img.tag}`"
                                     :label="img.tag"
                                     :value="img.full"></el-option>
                        </el-select>
                      </el-form-item>
                    </div>

                  </div>
                </div>
              </div>
            </el-form>
          </el-card>
        </template>
      </div>
      <el-form label-width="200px"
               class="ops">
        <el-form-item>
          <el-button @click="startDeploy"
                     :loading="startDeployLoading"
                     type="primary"
                     size="medium">确定</el-button>
          <el-button @click="goBack"
                     :loading="startDeployLoading"
                     size="medium">取消</el-button>
        </el-form-item>
      </el-form>
      <footer v-if="startDeployLoading"
              class="create-footer">
        <el-row :gutter="20">
          <el-col :span="16">
            <div class="grid-content bg-purple">
              <div class="description">
                <el-tag type="primary">正在创建环境中....</el-tag>
              </div>
            </div>
          </el-col>

          <el-col :span="8">
            <div class="grid-content bg-purple">
              <div class="deploy-loading">
                <div class="spinner__item1"></div>
                <div class="spinner__item2"></div>
                <div class="spinner__item3"></div>
                <div class="spinner__item4"></div>
              </div>
            </div>
          </el-col>
        </el-row>
      </footer>
    </div>

  </div>
</template>

<script>
import { imagesAPI, initProductAPI, createProductAPI } from '@api';
import bus from '@utils/event_bus';
import { uniq } from 'lodash';
import { serviceTypeMap } from '@utils/word_translate';
import { cloneDeep } from 'lodash'
let validateKey = (rule, value, callback) => {
  if (typeof value === 'undefined' || value == '') {
    callback(new Error('请输入Key'));
  } else {
    if (!/^[a-zA-Z0-9_]+$/.test(value)) {
      callback(new Error('Key 只支持字母大小写和数字，特殊字符只支持下划线'));
    } else {
      callback();
    }
  }
};
let validateEnvName = (rule, value, callback) => {
  if (typeof value === 'undefined' || value == '') {
    callback(new Error('填写环境名称'));
  } else {
    if (!/^[a-z0-9-]+$/.test(value)) {
      callback(new Error('环境名称只支持小写字母和数字，特殊字符只支持中划线'));
    } else {
      callback();
    }
  }
};
export default {
  data() {
    return {
      selection: '',
      projectConfig: {
        product_name: '',
        cluster_id: '',
        env_name: '',
        source: 'system',
        vars: [],
        revision: null,
        isPublic: true,
        roleIds: [],
      },
      startDeployLoading: false,
      loading: false,
      addKeyInputVisable: false,
      imageMap: {},
      containerMap: {},
      quickSelection: '',
      unSelectedImgContainers: [],
      serviceTypeMap: serviceTypeMap,
      rules: {
        env_name: [
          { required: true, trigger: 'change', validator: validateEnvName, }
        ]
      },
      addKeyData: [
        {
          key: '',
          value: '',
          state: 'unused'
        }
      ],
      keyCheckRule: {
        key: [
          {
            type: 'string',
            required: true,
            validator: validateKey,
            trigger: 'blur'
          }
        ],
        value: [
          {
            type: 'string',
            required: false,
            message: 'value',
            trigger: 'blur'
          }
        ]
      }
    };
  },

  computed: {
    projectName() {
      return this.$route.params.project_name;
    },
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    },
    showEmptyServiceModal() {
      return this.$utils.isEmpty(this.containerMap);
    }
  },
  methods: {
    async checkProjectFeature() {
      const projectName = this.projectName;
      this.projectInfo = await getSingleProjectAPI(projectName);
    },
    async getTemplateAndImg() {
      this.loading = true;
      const template = await initProductAPI(this.projectName, this.isStcov);
      this.loading = false;
      this.projectConfig.revision = template.revision;
      this.projectConfig.vars = template.vars;
      for (const group of template.services) {
        group.sort((a, b) => {
          if (a.service_name !== b.service_name) {
            return a.service_name.charCodeAt(0) - b.service_name.charCodeAt(0);
          }
          if (a.type === 'k8s' || b.type === 'k8s') {
            return a.type === 'k8s' ? 1 : -1;
          }
          return 0;
        });
      }

      const containerMap = {};
      const containerNames = [];
      for (const group of template.services) {
        for (const ser of group) {
          if (ser.type === 'k8s') {
            containerMap[ser.service_name] = containerMap[ser.service_name] || {};
            containerMap[ser.service_name][ser.type] = ser;
            ser.picked = true;
            const containers = ser.containers;
            if (containers) {
              for (const con of containers) {
                containerNames.push(con.name);
                Object.defineProperty(con, 'defaultImage', {
                  value: con.image,
                  enumerable: false,
                  writable: false
                });
              }
            }
          }
        }
      }
      this.projectConfig.services = template.services;
      this.containerMap = containerMap;
      imagesAPI(uniq(containerNames)).then((images) => {
        if (images) {
          for (const image of images) {
            image.full = `${image.host}/${image.owner}/${image.name}:${image.tag}`;
          }
          this.imageMap = this.makeMapOfArray(images, 'name');
          this.quickSelection = 'latest';
        }
      })
    },
    makeMapOfArray(arr, namePropName) {
      const map = {};
      for (const obj of arr) {
        if (!map[obj[namePropName]]) {
          map[obj[namePropName]] = [obj];
        } else {
          map[obj[namePropName]].push(obj);
        }
      }
      return map;
    },
    checkImgSelected(container_img_selected) {
      let containerNames = [];
      for (let service in container_img_selected) {
        for (let containername in container_img_selected[service]) {
          if (container_img_selected[service][containername] === '') {
            containerNames.push(containername);
          }
        }
      }
      this.unSelectedImgContainers = containerNames;
      return containerNames;
    },
    mapImgToprojectConfig(product_tpl, container_img_selected) {
      for (let service_con_img in container_img_selected) {
        for (let container in container_img_selected[service_con_img]) {
          product_tpl.services.map(service_group => {
            service_group.map(service => {
              service.containers.map((con, index_con) => {
                if (con.name === container) {
                  service.containers[index_con] = {
                    name: con.name,
                    image: container_img_selected[service.service_name][con.name]
                  };
                }
              });
            });
          });
        }
      }
    },
    addRenderKey() {
      if (this.addKeyData[0].key !== '') {
        this.$refs['addKeyForm'].validate(valid => {
          if (valid) {
            this.projectConfig.vars.push(this.$utils.cloneObj(this.addKeyData[0]));
            this.addKeyData[0].key = '';
            this.addKeyData[0].value = '';
            this.$refs['addKeyForm'].resetFields();
            this.$refs['addValueForm'].resetFields();
          } else {
            return false;
          }
        });
      }
    },
    deleteRenderKey(index, state) {
      if (state === 'present') {
        this.$confirm('该 Key 被产品服务模板引用，确定删除', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.projectConfig.vars.splice(index, 1);
        }).catch(() => {
          this.$message({
            type: 'info',
            message: '已取消删除'
          });
        });
      }
      else {
        this.projectConfig.vars.splice(index, 1);
      }
    },
    startDeploy() {
      this.deployEnv();
    },
    deployEnv() {
      const picked2D = [];
      const picked1D = [];
      this.$refs['create-env-ref'].validate((valid) => {
        if (valid) {
          for (const name in this.containerMap) {
            let atLeastOnePicked = false;
            const typeServiceMap = this.containerMap[name];
            for (const type in typeServiceMap) {
              const service = typeServiceMap[type];
              if (service.type === 'k8s' && service.picked) {
                atLeastOnePicked = true;
              }
            }
            if (!atLeastOnePicked) {
              this.$message.warning(`每个服务至少要选择一种，${name} 未勾选`);
              return;
            }
          }

          for (const group of this.projectConfig.services) {
            for (const ser of group) {
              if (ser.picked) {
                picked1D.push(ser);
              }
              const containers = ser.containers;
              if (containers && ser.picked && ser.type === 'k8s') {
                for (const con of ser.containers) {
                  if (!con.image) {
                    this.$message.warning(`${con.name}未选择镜像`);
                    return;
                  }
                }
              }
            }
          }
          picked2D.push(picked1D);
          const payload = this.$utils.cloneObj(this.projectConfig);
          payload.source = 'spock';
          const envType = 'test';
          this.startDeployLoading = true;
          createProductAPI(payload, envType).then((res) => {
            const envName = payload.env_name;
            this.startDeployLoading = false;
            this.$message({
              message: '创建环境成功',
              type: 'success'
            });
            this.$router.push(`/v1/projects/detail/${this.projectName}/envs/detail?envName=${envName}`);
          }, () => {
            this.startDeployLoading = false;
          })
        } else {
          return;
        }
      });
    },
    goBack() {
      this.$router.back();
    },
  },
  watch: {
    quickSelection(select) {
      for (const group of this.projectConfig.services) {
        for (const ser of group) {
          ser.picked = (ser.type === 'k8s' && (select === 'latest' || select === 'default'));
          const containers = ser.containers;
          if (containers) {
            for (const con of ser.containers) {
              if (select === 'latest') {
                if (this.imageMap[con.name]) {
                  con.image = this.imageMap[con.name][0].full;
                } else {
                  con.image = con.defaultImage;
                }
              }
              if (select === 'default') {
                con.image = con.defaultImage;
              }
            }
          }
        }
      }
    }
  },
  created() {
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: `项目`, url: `/v1/projects/detail/${this.projectName}` }, { title: `${this.projectName}`, url: `/v1/projects/detail/${this.projectName}` }, { title: '集成环境', url: `` }, { title: '创建', url: `` }] });
    this.projectConfig.product_name = this.projectName;
    this.getTemplateAndImg();
  },
};
</script>

<style lang="less">
.create-product-detail-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 20px;
  font-size: 13px;
  .module-title h1 {
    font-weight: 200;
    font-size: 1.5rem;
    margin-bottom: 30px;
  }
  .btn {
    display: inline-block;
    min-width: 87px;
    height: 30px;
    padding: 0 8px;
    margin: 0 auto 38px;
    font-size: 12px;
    line-height: 30px;
    font-weight: 500;
    transition: all 0.15s;
    line-height: 32px;
    border-radius: 4px;
    cursor: pointer;
    border: 1px solid transparent;
    white-space: nowrap;
  }
  .btn-primary {
    color: #1989fa;
    background-color: rgba(25, 137, 250, 0.04);
    border-color: rgba(25, 137, 250, 0.4);
    &:hover {
      color: #fff;
      background-color: #1989fa;
      border-color: #1989fa;
    }
  }
  .btn-mute {
    color: rgba(94, 97, 102, 0.8);
    background-color: transparent;
    border-color: rgba(94, 97, 102, 0.4);
    &:hover {
      color: rgba(94, 97, 102, 0.8);
      background-color: transparent;
      border-color: rgba(94, 97, 102, 0.4);
    }
    &[disabled] {
      color: rgba(94, 97, 102, 0.8);
      background-color: transparent;
      border-color: rgba(94, 97, 102, 0.4);
      cursor: not-allowed;
    }
  }
  .box-card,
  .box-card-service {
    margin-top: 25px;
    margin-bottom: 25px;
    box-shadow: none;
    border: none;
    .item {
      .item-name {
        margin: 10px 0;
      }
      .el-row {
        margin-bottom: 15px;
      }
      .img-tooltip {
        font-size: 15px;
        color: #5a5e66;
        &:hover {
          color: #1989fa;
          cursor: pointer;
        }
      }
      .img-select {
        width: 140px;
      }
    }
    .services-container {
      p {
        margin: 0;
        padding: 0;
        line-height: 20px;
      }
      .container-name {
        color: #2f3033;
        font-weight: 700;
      }
      .el-table {
        .cell {
          padding-left: 5px;
          padding-top: 5px;
          padding-bottom: 5px;
        }
      }
      .service-wrap {
        padding-bottom: 20px;
        .service-name-tag {
          margin-bottom: 3px;
        }
      }
    }
  }
  .el-card__header {
    padding-left: 0px;
    padding-top: 10px;
    border-bottom: 1px solid #dcdfe5;
    box-sizing: border-box;
  }
  .el-collapse-item__header {
    padding-left: 0px;
  }
  .no-resources {
    border-radius: 4px;
    border-style: hidden;
    border-collapse: collapse;
    box-shadow: 0 0 0 2px #f1f1f1;
    padding: 45px;
    img {
      display: block;
      width: 360px;
      height: 360px;
      margin: 10px auto;
    }
    .description {
      text-align: center;
      margin: 16px auto;
      p {
        color: #8d9199;
        font-size: 15px;
      }
    }
  }
  .create-footer {
    position: fixed;
    -webkit-box-sizing: border-box;
    box-sizing: border-box;
    width: 800px;
    bottom: 0;
    padding: 15px 60px 10px 0px;
    z-index: 5;
    text-align: left;
    border-top: 1px solid #fff;
    background-color: #fff;
    .grid-content {
      border-radius: 4px;
      min-height: 36px;
      .description {
        line-height: 36px;
        p {
          margin: 0;
          text-align: left;
          line-height: 36px;
          font-size: 16px;
          color: #676767;
        }
      }
      .deploy-loading {
        width: 100px;
        text-align: center;
        line-height: 36px;
        margin-left: 70px;
        div {
          width: 8px;
          height: 8px;
          background-color: #1989fa;
          border-radius: 100%;
          display: inline-block;
          animation: sk-bouncedelay 1.4s infinite ease-in-out both;
          margin-right: 4px;
        }
        .spinner__item1 {
          animation-delay: -0.6s;
        }
        .spinner__item2 {
          animation-delay: -0.4s;
        }
        .spinner__item3 {
          animation-delay: -0.2s;
        }
        @keyframes sk-bouncedelay {
          0%,
          80%,
          100% {
            -webkit-transform: scale(0);
            transform: scale(0);
            opacity: 0;
          }
          40% {
            -webkit-transform: scale(1);
            transform: scale(1);
            opacity: 1;
          }
        }
      }
    }
  }

  .el-input__inner {
    width: 250px;
  }
  .el-form-item__label {
    text-align: left;
  }
  .env-form {
    display: flex;
    .el-form-item {
      margin-bottom: 0;
      margin-right: 0;
      width: 50%;
    }
  }
  .second-title {
    color: #606266;
    font-size: 14px;
  }
  .small-title {
    color: #969799;
    font-size: 12px;
  }
  .service-filter {
    margin-left: 56px;
    color: #409eff;
    .el-input__inner {
      color: #409eff;
      border-color: #8cc5ff;
      &::placeholder {
        color: #8cc5ff;
      }
    }
  }

  .el-tag {
    background-color: rgba(64, 158, 255, 0.2);
    /* min-width: 500px; */
    /* text-align: center; */
  }
  .service-form {
    margin: 10px 0 0 0;
    padding-left: 10px;
    .el-form-item {
      margin-bottom: 10px;
    }
    .group {
      margin-top: 10px;
      box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
      padding: 10px;
    }
  }
  .service {
    display: flex;
  }
  .service-block {
    /* width: 50%; */
    margin: 10px 30px 0 0;
    .el-checkbox {
      font-size: 24px;
      .el-checkbox__input {
        height: 22px;
      }
    }
  }
  .container-images {
    border: 1px solid #ddd;
    border-radius: 5px;
    margin: 5px 0 0 0;
    padding: 10px 10px 0 10px;
  }
  .el-card__header {
    border-bottom: none;
    position: relative;
    padding-bottom: 10px;
    &::before {
      content: "";
      position: absolute;
      left: 0;
      bottom: 0;
      height: 1px;
      width: 400px;
      border-bottom: 1px solid #eee;
    }
  }
  .ops {
    margin-top: 25px;
  }
  .kv-container {
    .el-table {
      .unused {
        background: #e6effb;
      }
      .present {
        background: #fff;
      }
      .new {
        background: oldlace;
      }
    }

    .el-table__row {
      .cell {
        span {
          font-weight: 400;
        }
        .operate {
          font-size: 1.12rem;
          .delete {
            color: #ff1949;
          }
        }
      }
    }
    .render-value {
      max-width: 100%;
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .add-key-container {
      .el-form-item__label {
        display: none;
      }
      .el-form-item {
        margin-bottom: 15px;
      }
    }
    .add-kv-btn {
      display: inline-block;
      margin-top: 10px;
      margin-left: 5px;
      i {
        color: #5e6166;
        font-size: 14px;
        line-height: 14px;
        cursor: pointer;
        padding-right: 4px;
        color: #1989fa;
      }
    }
  }
}
</style>
