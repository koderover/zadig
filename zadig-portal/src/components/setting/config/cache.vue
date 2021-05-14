<template>
  <div class="config-cache-container">

    <template>
      <el-alert type="info"
                :closable="false"
                description="清理系统中的组件缓存">
      </el-alert>
    </template>
    <div class="cache-container">
      <span class="item">镜像缓存清理</span>
      <el-button size="mini"
                 type="danger"
                 @click="cleanCache"
                 :disabled="cleanStatus && cleanStatus.status === 'cleaning'"
                 plain
                 round>一键清理</el-button>
      <template v-if="cleanStatus">
        <div class="desc">
          <span class="title">状态：</span>
          <el-tag size="mini"
                  :type="tagTypeMap[cleanStatus.status]">{{statusMap[cleanStatus.status]}}
          </el-tag>
        </div>
        <div v-if="cleanStatus.status!=='failed'"
             class="desc">
          <span class="title">{{descMap[cleanStatus.status]}}</span>
          <span
                v-if="cleanStatus.status!=='unStart'">{{convertTimestamp(cleanStatus.update_time)}}</span>
        </div>
        <div v-if="cleanStatus.status==='failed'"
             class="desc">
          <el-row>
            <el-col :span="1"> <span class="title">原因：</span></el-col>
            <el-col :span="23">
              <div v-for="(pod,index) in cleanStatus.dind_clean_infos"
                   :key="index">
                <span class="pod-name">{{pod.pod_name}}</span>
                <span class="error-msg">{{pod.error_message}}</span>
              </div>
            </el-col>
          </el-row>
        </div>
        <div v-else-if="cleanStatus.status==='success' && cleanStatus.dind_clean_infos.length > 0"
             class="desc">
          <el-row>
            <el-col :span="1"> <span class="title">信息：</span></el-col>
            <el-col :span="23">
              <div v-for="(pod,index) in cleanStatus.dind_clean_infos"
                   :key="index">
                <span class="pod-name">{{pod.pod_name}}</span>
                <span class="info-msg">{{pod.clean_info}}</span>
              </div>
            </el-col>
          </el-row>
        </div>
      </template>
      <el-divider></el-divider>
    </div>
  </div>
</template>
<script>
import { cleanCacheAPI, getCleanCacheStatusAPI } from '@api';
import moment from 'moment';
export default {
  data() {
    return {
      cleanStatus: null,
      timerId: null,
      finishedReq: false,
      statusMap: {
        cleaning: '清理中',
        success: '清理完成',
        failed: '清理失败',
        unStart: '未执行过清理'
      },
      tagTypeMap: {
        cleaning: '',
        success: 'success',
        failed: 'danger',
        unStart: 'info'
      },
      descMap: {
        cleaning: '开始时间：',
        success: '完成时间：'
      }
    };
  },
  methods: {
    async getCleanStatus() {
      this.cleanStatus = await getCleanCacheStatusAPI();
      if (this.cleanStatus.status !== 'cleaning') {
        return
      };
      if (!this.finishedReq) {
        this.timerId = setTimeout(this.getCleanStatus, 3000);
      }
    },
    cleanCache() {
      this.$confirm('停止的容器、所有未被容器使用的网络、无用的镜像和构建缓存镜像将被删除，确认清理？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        cleanCacheAPI().then(response => {
          this.getCleanStatus();
          this.$message({
            type: 'info',
            message: '正在进行缓存清理'
          });
        }).catch(error => {
          this.$message.error(`缓存清理失败`);
        })
      }).catch(() => {
        this.$message({
          type: 'info',
          message: '已取消清理'
        });
      });
    },
    convertTimestamp(value) {
      return moment.unix(value).format('YYYY-MM-DD HH:mm:ss');
    },
  },
  computed: {
    currentOrganizationId() {
      return this.$store.state.login.userinfo.organization.id;
    }
  },
  activated() {
    this.getCleanStatus();
  },
  beforeDestroy() {
    this.finishedReq = true;
    clearTimeout(this.timerId);
  },
}
</script>

<style lang="less">
.config-cache-container {
  flex: 1;
  position: relative;
  overflow: auto;
  font-size: 13px;
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
  .cache-container {
    padding-top: 15px;
    padding-bottom: 15px;
    .item {
      color: #303133;
      font-size: 16px;
    }
    .desc {
      margin-top: 10px;
    }
    span.title {
      color: #909399;
    }
    span.error-msg {
      color: #ff1949;
    }
    span.info-msg {
      color: #303133;
    }
  }
}
</style>