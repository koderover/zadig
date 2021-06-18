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
import { setFavoriteAPI, deleteFavoriteAPI } from '@api'
export default {
  data () {
    return {
    }
  },
  props: {
    productName: {
      type: String,
      required: true
    },

    type: {
      type: String,
      required: true
    },

    isFavorite: {
      type: Boolean,
      required: true
    },

    name: {
      type: String,
      required: true
    },

    pipelineLink: {
      type: String,
      required: true
    },

    latestTaskStatus: {
      type: String,
      required: true
    },

    recentSuccessID: {
      type: null,
      required: true
    },

    recentSuccessLink: {
      type: String,
      required: true
    },
    recentFailID: {
      type: null,
      required: true
    },
    recentFailLink: {
      type: String,
      required: true
    },

    updateBy: {
      type: String,
      required: true
    },
    updateTime: {
      type: String,
      required: true
    },
    avgRuntime: {
      type: String,
      required: true
    },
    avgSuccessRate: {
      type: String,
      required: false
    }
  },
  methods: {
    setFavorite (productName, workflowName, type) {
      const payload = {
        product_name: productName,
        name: workflowName,
        type: type
      }
      if (this.isFavorite) {
        deleteFavoriteAPI(productName, workflowName, type).then((res) => {
          if (type === 'workflow') {
            this.$store.dispatch('refreshWorkflowList')
          }
          this.$message({
            type: 'success',
            message: '取消收藏成功'
          })
        })
      } else {
        setFavoriteAPI(payload).then((res) => {
          if (type === 'workflow') {
            this.$store.dispatch('refreshWorkflowList')
          }
          this.$message({
            type: 'success',
            message: '添加收藏成功'
          })
        })
      }
    }
  },
  computed: {
  }
}
</script>

<style lang="less">
.pipeline-row {
  position: relative;
  display: flex;
  flex-flow: row nowrap;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
  border: 1px solid #eaeaea;
  border-radius: 2px;

  .pipeline-name {
    margin: 8px 0;
    margin-block-start: 0;
  }

  .pipeline-name,
  .pipeline-name a {
    color: #1989fa;
    font-weight: 500;
    font-size: 16px;
  }

  .service-pipeline-tag {
    padding: 1px 3px;
    color: #1989fa;
    font-weight: 300;
    font-size: 11px;
    white-space: nowrap;
    border: 1px solid #1989fa;
    border-radius: 3px;
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
    position: relative;
    flex-grow: 1;
    padding: 0 1.5em;
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
      color: #dbdbdb;
      font-size: 25px;
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
        position: relative;
        flex: 0 0 19%;
        width: 19%;
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
          display: block;
          color: #888;
          font-weight: bold;
          font-size: 13px;
        }

        .value {
          margin-bottom: 0;
          color: #4c4c4c;
          font-weight: bold;
          font-size: 14px;
          line-height: 16px;
        }

        &.rate {
          width: 60px;
        }
      }

      .dash-process {
        width: 350px;

        .stage-tag {
          display: inline-block;
          margin-right: 4px;
          line-height: 25px;
        }
      }

      .dash-menu {
        width: 180px;
        color: #ccc;
        font-size: 23px;

        .menu-item {
          margin-right: 15px;
        }

        .el-icon-more {
          color: #ccc;
          font-size: 18px;
          cursor: pointer;
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
