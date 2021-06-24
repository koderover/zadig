<template>
  <div v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading iconbanben"
       class="version-container-list">
    <div>
      <el-select v-model="selectedService"
                 placeholder="请选择服务名称"
                 clearable
                 size="small">
        <el-option v-for="(item,index) in serviceList"
                   :key="index"
                   :label="item"
                   :value="item">
        </el-option>
      </el-select>
      <el-button type="primary"
                 @click="searchVersionByPOS"
                 plain
                 size="small"
                 icon="el-icon-search">搜索</el-button>
    </div>
    <el-table :data="versionList"
              v-show="versionList.length > 0"
              style="width: 100%;">
      <el-table-column label="版本">
        <template slot-scope="scope">
          <span class="version-link">
            <router-link
                         :to="`/v1/delivery/version/${productName}/${scope.row.versionInfo.id}?version=${scope.row.versionInfo.version}`">{{
              scope.row.versionInfo.version }}</router-link>
          </span>
        </template>
      </el-table-column>
      <el-table-column label="标签">
        <template slot-scope="scope">
          <span v-for="(label,index) in scope.row.versionInfo.labels"
                :key="index"
                style="margin-right: 3px;">
            <el-tag size="small">{{label}}</el-tag>
          </span>

        </template>
      </el-table-column>
      <el-table-column prop="create_by"
                       label="创建人">
        <template slot-scope="scope">
          <span>{{ scope.row.versionInfo.createdBy }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="create_at"
                       label="创建时间">
        <template slot-scope="scope">
          <span>{{ $utils.convertTimestamp(scope.row.versionInfo.created_at) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作">
        <template slot-scope="scope">

          <span @click="deleteVersion(scope.row.versionInfo.id)"
                class="delete-version-icon">
            <i class="el-icon-delete"></i>
          </span>

        </template>
      </el-table-column>

    </el-table>
    <div v-if="versionList.length === 0 || loading"
         class="no-version">
      <img src="@assets/icons/illustration/version_manage.svg"
           alt="" />
    </div>
  </div>
</template>

<script>
import bus from '@utils/event_bus'
import { getVersionListAPI, getVersionServiceListAPI, deleteVersionAPI } from '@api'
export default {
  data () {
    return {
      loading: false,
      versionList: [],
      productList: [],
      serviceList: [],
      selectedService: ''
    }
  },
  methods: {
    deleteVersion (versionId) {
      this.$confirm('确定删除该版本', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        deleteVersionAPI(versionId).then((res) => {
          this.$message({
            type: 'success',
            message: '删除成功'
          })
          this.getVersionServiceList()
          this.searchVersionByPOS()
        })
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '已取消删除'
        })
      })
    },
    searchVersionByPOS () {
      this.loading = true
      const orgId = this.currentOrganizationId
      getVersionListAPI(orgId, '', this.productName, '', this.selectedService).then((res) => {
        this.versionList = res
        this.loading = false
      }).catch((err) => {
        this.$message.error(`获取${this.selectedService || this.productName}版本信息出错：${err}`)
        this.loading = false
      })
    },
    getVersionServiceList () {
      const orgId = this.currentOrganizationId
      getVersionServiceListAPI(orgId, this.productName).then((res) => {
        this.serviceList = res
      })
    }
  },
  computed: {
    currentOrganizationId () {
      return this.$store.state.login.userinfo.organization.id
    },
    productName () {
      return this.$route.params.project_name
    }
  },
  watch: {
    $route (to, from) {
      if (this.productName) {
        bus.$emit(`set-topbar-title`, {
          title: '',
          breadcrumb: [{ title: '版本管理', url: `` },
            { title: this.productName, url: `` }]
        })
        this.selectedService = ''
        this.getVersionServiceList()
        this.searchVersionByPOS()
      }
    }
  },
  created () {
    bus.$emit(`set-topbar-title`, {
      title: '',
      breadcrumb: [{ title: '版本管理', url: `` },
        { title: this.productName, url: `` }]
    })
    this.getVersionServiceList()
    this.searchVersionByPOS()
  }
}
</script>

<style lang="less">
.version-container-list {
  position: relative;
  flex: 1;
  padding: 15px 30px;
  overflow: auto;
  font-size: 13px;

  .module-title h1 {
    margin-bottom: 1.5rem;
    font-weight: 200;
    font-size: 2rem;
  }

  .title {
    color: #8d9199;
    font-size: 14px;
  }

  .delete-version-icon {
    color: #ff1949;
    font-size: 18px;
    cursor: pointer;
  }

  .version-link {
    a {
      color: #1989fa;
    }
  }

  .no-version {
    display: flex;
    flex-direction: column;
    align-content: center;
    align-items: center;
    justify-content: center;
    height: 70vh;

    img {
      width: 400px;
      height: 400px;
    }
  }
}
</style>
