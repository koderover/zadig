<template>
  <div class="product-distribute">
    <el-card class="box-card">

      <el-table :data="serviceDists">
        <el-table-column prop="target"
                         label="服务">
          <template slot-scope="{ row }">
            <span
                  v-if="row.target">{{`${row.target.service_name}/${row.target.service_module}`}}</span>
          </template>
        </el-table-column>

        <el-table-column>
          <template slot="header"
                    slot-scope="_">
            <el-checkbox v-model="preferImageEverywhere"
                         :indeterminate="isImageIndeterminate"
                         size="mini"
                         class="head">
              镜像分发
            </el-checkbox>
          </template>
          <template slot="default"
                    slot-scope="{ row }">
            <el-checkbox v-model="row.image_distribute"
                         size="mini"></el-checkbox>
          </template>
        </el-table-column>

        <el-table-column>
          <template slot="header"
                    slot-scope="_">
            <el-checkbox v-model="preferStorageEverywhere"
                         :indeterminate="isStorageIndeterminate"
                         size="mini"
                         class="head">
              存储空间分发
            </el-checkbox>
          </template>
          <template slot="default"
                    slot-scope="{ row }">
            <el-checkbox v-model="row.qstack_distribute"
                         size="mini"></el-checkbox>
          </template>
        </el-table-column>

        <el-table-column label="操作"
                         width="100px">
          <template slot-scope="{ row }">
            <el-button @click="removeServiceDist(row.$index)"
                       type="danger"
                       icon="el-icon-delete"
                       size="mini">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="service-adder">
        <el-select style="width:360px"
                   v-model="serviceToAdd"
                   filterable
                   value-key="key"
                   size="small">
          <el-option v-for="(tar,index) of unconfiguredTargetsDisplayed"
                     :key="index"
                     :label="`${tar.service_name}${tar.service_module?'/'+tar.service_module:''}`"
                     :value="tar"></el-option>
        </el-select>
        <el-button @click="addServiceDist"
                   type="default"
                   size="small"
                   icon="el-icon-plus">添加服务</el-button>
      </div>

      <div v-show="showImageRepos"
           class="where-to-dist">
        <span class="title">
          镜像仓库：
        </span>
        <el-select style="width:360px"
                   value-key="id"
                   v-model="distribute_stage.releaseIds"
                   multiple
                   filterable
                   clearable
                   size="small">
          <el-option v-for="(repo,index) of imageRepos"
                     :key="index"
                     :label="`${repo.regAddr.split('://')[1]}/${repo.regNamespace}`"
                     :value="repo.id"></el-option>
        </el-select>
      </div>

      <div v-show="showStorageList"
           class="where-to-dist">
        <span class="title">
          对象存储：
        </span>
        <el-select style="width:360px"
                   v-model="distribute_stage.s3_storage_id"
                   filterable
                   size="small">
          <el-option v-for="(storage,index) of storageList"
                     :key="index"
                     :label="`${storage.endpoint}/${storage.bucket}`"
                     :value="storage.id"></el-option>
        </el-select>
      </div>

    </el-card>
  </div>
</template>

<script type="text/javascript">
import bus from '@utils/event_bus';
import { getStorageListAPI, imageReposAPI } from '@api';
import _ from 'lodash';
export default {
  data() {
    return {
      imageRepos: [],
      storageList: [],
      serviceToAdd: null
    };
  },
  computed: {
    serviceDists: {
      get() {
        return this.distribute_stage.distributes;
      },
      set(val) {
        this.distribute_stage.distributes = val;
      }
    },
    serviceDistMap() {
      return _.keyBy(this.serviceDists, (i) => {
        return i.target.service_name + '/' + i.target.service_module;
      });
    },
    allTargets() {
      const targets = this.presets.map(p => p.target);
      targets.forEach(t => {
        t.key = t.service_name + '/' + t.service_module;
      });
      return targets;
    },
    unconfiguredTargetsDisplayed() {
      return [{ service_name: '同步部署模块服务' }].concat(this.unconfiguredTargets);
    },
    unconfiguredTargets() {
      const rest = this.allTargets.filter(t => !((t.service_name + '/' + t.service_module) in this.serviceDistMap));
      return rest;
    },
    imageChecksStatus() {
      return this.getCheckStatus('image_distribute');
    },
    storageChecksStatus() {
      return this.getCheckStatus('qstack_distribute');
    },

    preferImageEverywhere: {
      get() {
        return this.imageChecksStatus.allChecked;
      },
      set(val) {
        this.toggleAKindOfDist('image_distribute', val, 'isImageIndeterminate');
      }
    },
    preferStorageEverywhere: {
      get() {
        return this.storageChecksStatus.allChecked;
      },
      set(val) {
        this.toggleAKindOfDist('qstack_distribute', val, 'isStorageIndeterminate');
      }
    },
    isImageIndeterminate() {
      return this.imageChecksStatus.hasSomeChecked && this.imageChecksStatus.hasSomeUnchecked;
    },
    isStorageIndeterminate() {
      return this.storageChecksStatus.hasSomeChecked && this.storageChecksStatus.hasSomeUnchecked;
    },
    showImageRepos() {
      return this.serviceDists.some(d => d.image_distribute);
    },
    showStorageList() {
      return this.serviceDists.some(d => d.qstack_distribute);
    }
  },
  watch: {
    product_tmpl_name(newVal, oldVal) {
      if (oldVal) {
        this.serviceDists = [];
      }
    }
  },
  props: {
    distribute_stage: {
      required: true,
      type: Object
    },
    editMode: {
      required: true,
      type: Boolean
    },
    product_tmpl_name: {
      required: true,
      type: String
    },
    presets: {
      required: true,
      type: Array
    },
    buildTargets: {
      required: true,
      type: Array
    },
  },
  methods: {
    toggleAKindOfDist(backendFieldName, checked) {
      if (this.serviceDists && this.serviceDists.length > 0) {
        for (const dist of this.serviceDists) {
          dist[backendFieldName] = checked;
        }
      }
    },
    getCheckStatus(backendFieldName) {
      let hasSomeUnchecked = false;
      let hasSomeChecked = false;
      let allChecked = true;
      if (this.serviceDists && this.serviceDists.length > 0) {
        for (const dist of this.serviceDists) {
          if (dist[backendFieldName]) {
            hasSomeChecked = true;
          } else {
            hasSomeUnchecked = true;
            allChecked = false;
          }
        }
      }
      else {
        allChecked = false;
      }
      return {
        hasSomeUnchecked,
        hasSomeChecked,
        allChecked
      };
    },

    addServiceDist() {
      if (this.serviceToAdd.service_name === '同步部署模块服务') {
        for (const tar of this.buildTargets) {
          if (!((tar.service_name + '/' + tar.service_module) in this.serviceDistMap)) {
            this._addSingleServiceDist(tar);
          }
        }
      } else {
        this._addSingleServiceDist(this.serviceToAdd);
        this.serviceToAdd = null;
      }
    },
    _addSingleServiceDist(target) {
      if (target) {
        this.serviceDists.push({
          target: target,
          image_distribute: false,
          jump_box_distribute: false,
          qstack_distribute: false
        });
      }
    },

    removeServiceDist(index) {
      this.serviceDists.splice(index, 1);
    },
    checkDistribute() {
      if (this.preferImageEverywhere) {
        if (!this.distribute_stage.releaseIds || this.distribute_stage.releaseIds.length === 0) {
          this.$message({
            message: '尚未选择镜像仓库，请检查',
            type: 'warning'
          });
          bus.$emit('receive-tab-check:distribute', false);
        }
        else {
          bus.$emit('receive-tab-check:distribute', true);
        }
      }
      if (this.preferStorageEverywhere) {
        if (!this.distribute_stage.s3_storage_id || this.distribute_stage.s3_storage_id === '') {
          this.$message({
            message: '尚未选择对象存储，请检查',
            type: 'warning'
          });
          bus.$emit('receive-tab-check:distribute', false);
        }
        else {
          bus.$emit('receive-tab-check:distribute', true);
        }
      }
      else {
        bus.$emit('receive-tab-check:distribute', true);
      }
    }
  },
  created() {
    imageReposAPI().then(res => {
      this.imageRepos = res;
    });
    getStorageListAPI().then(res => {
      this.storageList = res;
    });
    bus.$on('check-tab:distribute', () => { this.checkDistribute() });
  },
  beforeDestroy() {
    bus.$off('check-tab:distribute');
  },
};
</script>

<style lang="less">
.product-distribute {
  .service-adder {
    margin-top: 20px;
  }

  .el-checkbox.head {
    color: #909399;
  }

  .where-to-dist {
    .title {
      display: inline-block;
      color: #606266;
      width: 97px;
      font-size: 14px;
    }
    margin-top: 10px;
  }
}
</style>
