<template>
  <li class="pipeline-row">
    <div class="dash-body"
         :class="latestTaskStatus">
      <div class="dash-main">
        <span @click="setFavorite(productName,name,type)"
              class="favorite el-icon-star-on"
              :class="{'liked':isFavorite}"></span>
        <header class="dash-header">
          <h2 class="pipeline-name">
            <router-link :to="pipelineLink">{{ name }}</router-link>
          </h2>
          <h2 class="recent-task">
            <span class="task-type">
              <i class="el-icon-success passed"></i>
              最近成功
              <router-link v-if="recentSuccessID"
                           :to="recentSuccessLink"
                           class="passed">#{{ recentSuccessID }}</router-link>
              <span v-else>*</span>
            </span>
            <span class="task-type">
              <i class="el-icon-error failed"></i>
              最近失败
              <router-link v-if="recentFailID"
                           :to="recentFailLink"
                           class="failed">#{{ recentFailID }}</router-link>
              <span v-else>*</span>
            </span>
          </h2>
        </header>

        <slot name="more"></slot>
        <section class="dash-basicinfo">
          <span class="avg-run-time">平均执行时间</span>
          <span class="value">{{avgRuntime}}</span>
        </section>
        <section class="dash-basicinfo rate">
          <span class="avg-success-rate">成功率</span>
          <span class="value">{{avgSuccessRate}}</span>
        </section>
        <section class="dash-menu">
          <slot name="operations"></slot>
        </section>
      </div>
    </div>
  </li>
</template>

<script>
import { setFavoriteAPI, deleteFavoriteAPI } from '@api';
export default {
  data() {
    return {
    };
  },
  props: {
    productName: {
      type: String,
      required: true,
    },

    type: {
      type: String,
      required: true,
    },

    isFavorite: {
      type: Boolean,
      required: true,
    },

    name: {
      type: String,
      required: true,
    },

    pipelineLink: {
      type: String,
      required: true,
    },

    latestTaskStatus: {
      type: String,
      required: true,
    },

    recentSuccessID: {
      type: null,
      required: true,
    },

    recentSuccessLink: {
      type: String,
      required: true,
    },
    recentFailID: {
      type: null,
      required: true,
    },
    recentFailLink: {
      type: String,
      required: true,
    },

    updateBy: {
      type: String,
      required: true,
    },
    updateTime: {
      type: String,
      required: true,
    },
    avgRuntime: {
      type: String,
      required: true,
    },
    avgSuccessRate: {
      type: String,
      required: false,
    }
  },
  methods: {
    setFavorite(productName, workflowName, type) {
      const payload = {
        product_name: productName,
        name: workflowName,
        type: type
      };
      if (this.isFavorite) {
        deleteFavoriteAPI(productName, workflowName, type).then((res) => {
          if (type === 'workflow') {
            this.$store.dispatch('refreshWorkflowList');
          }
          this.$message({
            type: 'success',
            message: '取消收藏成功'
          });
        })
      }
      else {
        setFavoriteAPI(payload).then((res) => {
          if (type === 'workflow') {
            this.$store.dispatch('refreshWorkflowList');
          }
          this.$message({
            type: 'success',
            message: '添加收藏成功'
          });
        })
      }

    }
  },
  computed: {
  },
};
</script>

<style lang="less">
.pipeline-row {
  border-radius: 2px;
  border: 1px solid #eaeaea;
  display: flex;
  margin-bottom: 1rem;
  position: relative;
  flex-flow: row nowrap;
  justify-content: space-between;
  align-items: center;
  .pipeline-name {
    margin: 8px 0;
    margin-block-start: 0;
  }
  .pipeline-name,
  .pipeline-name a {
    font-size: 16px;
    color: #1989fa;
    font-weight: 500;
  }
  .service-pipeline-tag {
    color: #1989fa;
    font-size: 11px;
    border: 1px solid #1989fa;
    border-radius: 3px;
    padding: 1px 3px;
    font-weight: 300;
    white-space: nowrap;
  }
  .recent-task {
    margin: 0;
    font-size: 12px;
    white-space: nowrap;
    .task-type {
      margin-right: 5px;
      .failed {
        color: #f56c6c;
      }
      .passed {
        color: #67c23a;
      }
    }

    span {
      color: #7987a8;
    }
  }
  .dash-body {
    flex-grow: 1;
    padding: 0 1.5em;
    position: relative;
    border-left: 4px solid #77797d;
    &.running,
    &.elected {
      border-left: 4px solid #1989fa;
      animation: blink 1.5s infinite;
    }
    &.passed,
    &.success {
      border-left: 4px solid #67c23a;
    }
    &.failed,
    &.failure,
    &.timeout {
      border-left: 4px solid #ff1949;
    }
    &.cancelled,
    &.terminated {
      border-left: 4px solid #77797d;
    }
    .favorite {
      display: inline-block;
      font-size: 25px;
      color: #dbdbdb;
      cursor: pointer;
      &.liked {
        color: #f4e118;
      }
      &:hover {
        color: #f4e118;
      }
    }
    .dash-main {
      display: flex;
      flex-flow: row nowrap;
      align-items: center;
      justify-content: space-between;
      height: 70px;
      .dash-header {
        width: 19%;
        flex: 0 0 19%;
        position: relative;
      }
      .dash-basicinfo {
        width: 80px;
        .author,
        .time {
          margin: 4px 0;
          color: #666;
          font-size: 15px;
        }
        .avg-run-time,
        .avg-success-rate {
          font-weight: bold;
          display: block;
          color: #888888;
          font-size: 13px;
        }
        .value {
          font-size: 14px;
          font-weight: bold;
          line-height: 16px;
          color: #4c4c4c;
          margin-bottom: 0;
        }
        &.rate {
          width: 60px;
        }
      }
      .dash-process {
        width: 350px;
        .stage-tag {
          margin-right: 4px;
          line-height: 25px;
          display: inline-block;
        }
      }
      .dash-menu {
        color: #ccc;
        font-size: 23px;
        width: 180px;
        .menu-item {
          margin-right: 15px;
        }
        .el-icon-more {
          font-size: 18px;
          cursor: pointer;
          color: #ccc;
        }
      }
    }
  }

  .start-build {
    display: inline-block;
    cursor: pointer;
  }
}
</style>
