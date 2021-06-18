<template>
  <span class="notification">
    <el-popover ref="popover4"
                placement="bottom"
                width="300"
                popper-class="notify-container"
                trigger="click">
      <div class="notify-header">
        <span class="msg">通知</span>
        <el-tooltip class="item"
                    effect="dark"
                    content="通知设置"
                    placement="top">
          <router-link to="/v1/profile/info"
                       class="setting pull-right">
            <i class="el-icon-setting icon"></i>
          </router-link>
        </el-tooltip>
        <el-tooltip class="item"
                    effect="dark"
                    content="全部通知设为已读"
                    placement="top">
          <span @click="notificationOperation('mark_all_as_read')"
                style="margin-right: 15px;"
                class="setread pull-right">
            <i class="el-icon-check"></i>
          </span>
        </el-tooltip>
      </div>
      <div class="notify-body">
        <div v-if="notifications.length===0"
             class="no-msg">没有通知</div>
        <div>
          <ul class="notifications-list">
            <li v-for="(notification,index) in notifications"
                :key="index"
                class="notification hasSeen level-error">
              <div v-if="notification.type===2">
                <span class="icon"
                      :class="colorTranslation(notification.content.status,'pipeline','task')"
                      :title="notification.content.status"></span>
                <h3 class="truncate">
                  <span>
                    <span class="status"
                          style="margin-right: 10px;">{{wordTranslation(notification.content.status,'pipeline','task')}}</span>
                    <router-link @click.native="markAsRead(notification, index)"
                                 :to="`/v1/projects/detail/${notification.content.product_name}/pipelines/${notification.content.type==='single'?notification.content.type:'multi'}/${notification.content.pipeline_name}/${notification.content.task_id}`">
                      <em>{{notification.content.pipeline_name}}
                        <span style="color: #1989fa; font-size: 15px; cursor: pointer;">{{'#' +
                          notification.content.task_id}}</span>
                      </em><br>
                    </router-link>
                  </span>
                </h3>
                <div class="event-extra">
                  <span :class="{'is-read':notification.is_read,'unread':!notification.is_read}">
                    {{notification.is_read?'已读':'未读'}}
                  </span>
                  <span class="time">{{$utils.convertTimestamp(notification.create_time)}}</span>
                </div>
                <span @click="notificationOperation('mark_as_read', notification, index)"
                      class="operation read">
                  <el-tooltip class="item"
                              effect="dark"
                              content="设为已读"
                              placement="top">
                    <i class="el-icon-check"></i>
                  </el-tooltip>
                </span>
                <span @click="notificationOperation('delete', notification, index)"
                      class="operation delete">
                  <el-tooltip class="item"
                              effect="dark"
                              content="删除该通知"
                              placement="top">
                    <i class="el-icon-delete"></i>
                  </el-tooltip>
                </span>
              </div>
              <div v-if="notification.type===3">
                <h3 class="truncate">
                  <span class="status"
                        style="margin-right: 10px;">{{notification.content.title}}</span>
                </h3>
                <div class="announcement-content">
                  <p>{{notification.content.content}}{{"("+notification.content.req_id+")"}}</p>
                </div>
                <div class="event-extra">
                  <span class="is-read">
                    {{notification.is_read?'已读':'未读'}}
                  </span>
                  <span class="time">{{$utils.convertTimestamp(notification.create_time)}}</span>
                </div>
                <span @click="notificationOperation('mark_as_read', notification, index)"
                      class="operation  read">
                  <el-tooltip class="item"
                              effect="dark"
                              content="设为已读"
                              placement="top">
                    <i class="el-icon-check"></i>
                  </el-tooltip>
                </span>
                <span @click="notificationOperation('delete', notification, index)"
                      class="operation delete">
                  <el-tooltip class="item"
                              effect="dark"
                              content="删除该通知"
                              placement="top">
                    <i class="el-icon-delete"></i>
                  </el-tooltip>
                </span>

              </div>
            </li>
          </ul>
        </div>
      </div>
    </el-popover>
    <div class="notify">
      <el-badge :value="unreadMsgs.length"
                :max="99"
                :hidden="unreadMsgs.length===0"
                class="item">
        <span v-popover:popover4>
          <i class="el-icon-bell icon"></i>
        </span>
      </el-badge>
    </div>
  </span>
</template>
<script>
import { wordTranslate, colorTranslate } from '@utils/word_translate'
import { getNotificationAPI, deleteAnnouncementAPI, markNotiReadAPI } from '@api'
export default {
  props: {},
  data: function () {
    return {
      notifications: [],
      unreadMsgs: []
    }
  },
  methods: {
    getNotifications () {
      getNotificationAPI().then((res) => {
        this.notifications = res
        this.unreadMsgs = []
        this.notifications.forEach(element => {
          if (!element.is_read) {
            this.unreadMsgs.push(element)
          }
        })
      })
    },

    notificationOperation (operation, notify_obj, index) {
      if (operation === 'delete') {
        const payload = {
          ids: [notify_obj.id]
        }
        deleteAnnouncementAPI(payload).then((res) => {
          this.getNotifications()
        })
      } else if (operation === 'mark_as_read') {
        const payload = {
          ids: [notify_obj.id]
        }
        markNotiReadAPI(payload).then((res) => {
          this.getNotifications()
        })
      } else if (operation === 'mark_all_as_read') {
        const payload = {
          ids: []
        }
        this.notifications.forEach(element => {
          payload.ids.push(element.id)
        })
        markNotiReadAPI(payload).then((res) => {
          this.getNotifications()
        })
      }
    },
    colorTranslation (word, category, subitem) {
      return colorTranslate(word, category, subitem)
    },
    wordTranslation (word, category, subitem) {
      return wordTranslate(word, category, subitem)
    },
    markAsRead (notify_obj, index) {
      if (!notify_obj.is_read) {
        this.notificationOperation('mark_as_read', notify_obj, index)
      }
    }
  },
  created () {
    this.getNotifications()
  }
}
</script>
<style lang="less">
.el-badge__content {
  &.is-fixed {
    top: 30%;
  }
}

.notification {
  display: inline-block;
}

.notify-container {
  padding: 0 !important;

  .notify-header {
    padding: 15px 20px;
    background-color: #f5f7fa;
    border-bottom: 1px solid #dcdfe5;

    .msg {
      font-size: 13px;
    }

    .pull-right {
      float: right;
    }

    .setting,
    .setread {
      color: #000;
      font-size: 18px;
      cursor: pointer;

      &:hover {
        color: #1989fa;
      }
    }
  }

  .notify-body {
    height: 290px;
    padding: 2px 5px;
    overflow: hidden;
    overflow-y: auto;

    &::-webkit-scrollbar-track {
      background-color: #f5f5f5;
      border-radius: 6px;
      box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
    }

    &::-webkit-scrollbar {
      width: 6px;
      background-color: #f5f5f5;
    }

    &::-webkit-scrollbar-thumb {
      background-color: #555;
      border-radius: 6px;
      box-shadow: inset 0 0 6px rgba(0, 0, 0, 0.3);
    }

    .no-msg {
      padding: 15px 20px;
      color: #5e6166;
      font-size: 12px;
      line-height: 200px;
      text-align: center;
    }

    .notifications-list {
      margin: 0;
      padding-left: 0;
      list-style: none;

      .notification {
        position: relative;
        padding: 10px 20px 10px 35px;
        background: #fff;
        border-bottom: 1px solid #e2dee6;
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);

        .operation {
          display: inline-block;
          margin-left: 10px;
          font-size: 16px;
          visibility: hidden;
          cursor: pointer;
        }

        &:hover {
          background-color: #f1f8ff;

          .operation {
            visibility: visible;
          }
        }

        .icon {
          position: absolute;
          top: 16px;
          left: 10px;
          width: 10px;
          height: 10px;
          border-radius: 50%;
        }

        .color-running {
          color: #1989fa;
          font-weight: 500;
        }

        .color-failed {
          color: #ff1949;
          font-weight: 500;
        }

        .color-cancelled {
          color: #909399;
          font-weight: 500;
        }

        .color-timeout {
          color: #e6a23c;
          font-weight: 500;
        }

        .color-passed {
          color: #6ac73c;
          font-weight: 500;
        }

        h3 {
          margin: 0;
          color: #2f2936;
          font-size: 14px;

          em {
            color: #303133;
            font-weight: 400;
            font-size: 14px;
            font-style: normal;

            .task_id {
              color: #1989fa;
            }
          }
        }

        .event-extra {
          display: inline-block;

          .is-read {
            margin-right: 10px;
            color: #999;
            font-size: 13px;
          }

          .unread {
            margin-right: 10px;
            color: #606266;
            font-size: 13px;
          }

          .time {
            margin-right: 10px;
            color: #606266;
            font-size: 13px;
          }
        }

        .announcement-content {
          p {
            margin: 5px 0;
            font-size: 12px;
          }
        }

        .operation.read {
          &:hover {
            color: #1989fa;
          }
        }

        .operation.delete {
          &:hover {
            color: #ff1949;
          }
        }

        .truncate {
          display: block;
          max-width: 100%;
          overflow: hidden;
          white-space: nowrap;
          text-overflow: ellipsis;
        }
      }
    }
  }
}
</style>
