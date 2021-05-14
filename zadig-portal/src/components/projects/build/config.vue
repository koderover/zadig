<template>
  <div class="buildConfig-container">
    <!--start of edit service target dialog-->
    <el-dialog title="修改服务"
               custom-class="create-buildconfig"
               :visible.sync="dialogEditTargetVisible">
      <el-form label-position="left"
               :model="editBuildManageTargets"
               @submit.native.prevent
               ref="editTargetForm">
        <el-form-item label="服务组件">
          <el-select v-model="editBuildManageTargets.targets"
                     value-key="key"
                     multiple
                     filterable>
            <el-option v-for="(service,index) in serviceTargets"
                       :key="index"
                       :label="`${service.service_module}(${service.service_name})`"
                       :value="service"></el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <div slot="footer"
           class="dialog-footer">
        <el-button type="primary"
                   native-type="submit"
                   @click="saveServiceTarget()"
                   class="start-create">确定</el-button>
      </div>
    </el-dialog>
    <!--end of edit service target dialog-->
    <div class="tab-container">
        <div>
          <el-alert type="info"
                    :closable="false"
                    description="定义服务组件的构建方式以及构建依赖">
          </el-alert>
          <div class="create-buildconfig">
              <router-link :to="`/v1/projects/detail/${projectName}/builds/create`">
                <el-button plain
                           size="small"
                           type="primary">添加</el-button>
              </router-link>
            <el-button v-if="!searchInputVisible"
                       @click="searchInputVisible=true"
                       plain
                       size="small"
                       type="primary"
                       icon="el-icon-search"></el-button>
            <transition name="fade">
              <el-input v-if="searchInputVisible"
                        size="small"
                        v-model.lazy="searchBuildConfig"
                        placeholder="请输入构建名称"
                        style="width: auto;"
                        autofocus
                        prefix-icon="el-icon-search">
              </el-input>
            </transition>

          </div>
          <div class="config-container">
            <el-table :data="filteredBuildConfigs"
                      style="width: 100%">
              <el-table-column label="名称">
                <template slot-scope="scope">
                  {{ scope.row.name }}
                </template>
              </el-table-column>
              <el-table-column prop="services"
                               label="服务组件">
                <template slot-scope="scope">
                  <template v-if="scope.row.targets.length > 0">
                    <div v-for="(item,index) in scope.row.targets"
                         :key="index">
                      <el-tooltip effect="dark"
                                  :content="item.service_name"
                                  placement="top">
                        <span>{{`${item.service_module}`}}</span>
                      </el-tooltip>
                    </div>
                  </template>
                    <span class="change-serviceTarget"><i @click="changeServiceTargets(scope.row)"
                         class="el-icon-edit"></i></span>
                </template>
              </el-table-column>
              <el-table-column label="更新时间">
                <template slot-scope="scope">
                  {{ $utils.convertTimestamp(scope.row.update_time) }}
                </template>
              </el-table-column>
              <el-table-column label="最后修改">
                <template slot-scope="scope">
                  {{scope.row.update_by }}
                </template>
              </el-table-column>
              <el-table-column label="操作"
                               width="240">
                <template slot-scope="scope">
                    <router-link
                                 :to="`/v1/projects/detail/${scope.row.productName}/builds/detail/${scope.row.name}/${scope.row.version}`">
                      <el-button type="primary"
                                 size="mini"
                                 plain>编辑</el-button>
                    </router-link>
                    <el-button @click="removeBuildConfig(scope.row)"
                               type="danger"
                               size="mini"
                               plain>删除</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>
    </div>
  </div>
</template>

<script>
import { getBuildConfigsAPI, deleteBuildConfigAPI, saveBuildConfigTargetsAPI, getServiceTargetsAPI } from '@api';
import { flattenDeep } from 'lodash';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
      buildConfigs: [],
      serviceTargets: [],
      editBuildManageTargets: {
        name: '',
        targets: []
      },
      searchBuildConfig: '',
      dialogEditTargetVisible: false,
      searchInputVisible: false,
    };
  },
  methods: {
    filterAvailableServices(services) {
      let existServices = [];
      this.buildConfigs.forEach(element => {
        existServices.push(element.targets);
      });
      return services.filter(element => {
        if (!(flattenDeep(existServices).includes(element.service_name))) {
          return element
        }
      });
    },
    removeBuildConfig(obj) {
      const projectName = this.projectName;
      const str = obj.pipelines.length > 0
        ? `该配置在 ${obj.pipelines} 存在引用，确定要删除 ${obj.name} 吗？`
        : `确定要删除 ${obj.name} 吗？`
      this.$confirm(str, '确认', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteBuildConfigAPI(obj.name, obj.version, projectName).then(() => {
          this.$message.success('删除成功');
          this.getBuildConfig();
        });
      });
    },
    changeServiceTargets(scope) {
      const projectName = this.projectName;
      getServiceTargetsAPI(projectName).then((data) => {
        this.serviceTargets = [...this.filterAvailableServices(data), ...scope.targets].map(element => {
          element.key = element.service_name + '/' + element.service_module
          return element
        })
      });
      this.editBuildManageTargets = this.$utils.cloneObj(scope);
      this.dialogEditTargetVisible = true;
    },
    saveServiceTarget() {
      const projectName = this.projectName;
      let targetsPayload = {
        name: this.editBuildManageTargets.name,
        targets: this.editBuildManageTargets.targets,
        productName: projectName
      };
      saveBuildConfigTargetsAPI(projectName, targetsPayload).then(() => {
        this.dialogEditTargetVisible = false;
        this.getBuildConfig();
        this.$message({
          type: 'success',
          message: '修改服务成功'
        });
      })
    },
    getBuildConfig() {
      const projectName = this.projectName;
      getBuildConfigsAPI(projectName).then((res) => {
        res.forEach(element => {
          element.targets = element.targets.map(t => {
            t.key = t.service_name + '/' + t.service_module;
            return t;
          });
        });
        this.buildConfigs = this.$utils.deepSortOn(res, 'name');
      });
    }
  },
  computed: {
    filteredBuildConfigs() {
      if (this.searchBuildConfig) {
        this.$router.replace({
          query: {
            name: this.searchBuildConfig
          }
        });
      }
      return this.$utils.filterObjectArrayByKey('name', this.searchBuildConfig, this.buildConfigs);
    },
    projectName() {
      return this.$route.params.project_name;
    }
  },
  created() {
    this.getBuildConfig();
    if (this.$route.query.name) {
      this.searchInputVisible = true;
      this.searchBuildConfig = this.$route.query.name;
    }
    else if (this.$route.query.add) {
      this.$router.replace(`/v1/projects/detail/${this.projectName}/builds/create`);
    }
    bus.$emit(`set-topbar-title`, { title: '', breadcrumb: [{ title: '项目', url: '/v1/projects' }, { title: this.projectName, url: `/v1/projects/detail/${this.projectName}` }, { title: '构建', url: '' }] });
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
};
</script>


<style lang="less">
.buildConfig-container {
  flex: 1;
  position: relative;
  overflow: auto;
  padding: 15px 20px;
  .breadcrumb {
    margin-bottom: 25px;
    .el-breadcrumb {
      font-size: 16px;
      line-height: 1.35;
      .el-breadcrumb__item__inner a:hover,
      .el-breadcrumb__item__inner:hover {
        color: #1989fa;
        cursor: pointer;
      }
    }
  }
  .module-title h1 {
    font-weight: 200;
    font-size: 2rem;
    margin-bottom: 1.5rem;
  }
  .create-buildconfig {
    margin-top: 25px;
    margin-bottom: 15px;
  }
  .change-serviceTarget {
    color: #1989fa;
    cursor: pointer;
  }
  .el-table th > .cell {
    color: #97a8be;
  }
  .el-input {
    display: inline-block;
  }
}

.create-buildconfig {
  width: 400px;
  .el-dialog__header {
    text-align: center;
    border-bottom: 1px solid #e4e4e4;
    padding: 15px;
    .el-dialog__close {
      font-size: 10px;
    }
  }
  .el-dialog__body {
    padding-bottom: 0px;
  }
  .dialog-footer {
    .start-create {
      width: 100%;
    }
  }
  .el-select {
    width: 100%;
  }
  .el-input {
    display: inline-block;
  }
}

.deltpt-dialog {
  .el-dialog__footer {
    padding: 10px 20px 15px;
    text-align: center;
    box-sizing: border-box;
  }
  .el-dialog__header,
  .el-dialog__body {
    text-align: center;
  }
  .el-input {
    display: inline-block;
  }
}
</style>