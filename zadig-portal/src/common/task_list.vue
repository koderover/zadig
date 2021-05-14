<template>
  <div class="task-list-container">
    <el-table :data="taskList"
              stripe
              style="width: 100%">
      <el-table-column prop="task_id"
                       label="ID"
                       width="80"
                       sortable>
        <template slot-scope="scope">
          <router-link :to="`${baseUrl}/${scope.row.task_id}?status=${scope.row.status}`"
                       class="task-id">
            {{ '#' +scope.row.task_id }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="创建信息">
        <template slot-scope="scope">
          {{ convertTimestamp(scope.row.create_time)+' '+ scope.row.task_creator}}
        </template>
      </el-table-column>
      <el-table-column v-if="showEnv"
                       width="100"
                       label="集成环境">
        <template slot-scope="scope">
          <span v-if="scope.row.workflow_args">
            {{ scope.row.workflow_args.namespace}}
          </span>
          <span v-else-if="scope.row.sub_tasks">
            {{ getDeployEnv(scope.row.sub_tasks)}}
          </span>
        </template>
      </el-table-column>
      <el-table-column v-if="showServiceNames"
                       width="180"
                       label="服务名称"
                       show-overflow-tooltip>
        <template slot-scope="{ row }">
          <span>{{ row.build_services && row.build_services.length > 0 && row.build_services.slice().sort().join(', ') || 'N/A' }}</span>
        </template>
      </el-table-column>
      <el-table-column width="120"
                       label="持续时间">
        <template slot-scope="scope">
          <el-icon name="time"></el-icon>
          <span v-if="scope.row.status!=='running'"
                style="margin-left: 5px;font-size:13px">
            {{ $utils.timeFormat(scope.row.end_time - scope.row.start_time) }}
          </span>
          <span v-else
                style="margin-left: 5px;font-size:13px">
            {{ taskDuration(scope.row.task_id,scope.row.start_time) +
              $utils.timeFormat(durationSet[scope.row.task_id]) }}
            <el-tooltip v-if="durationSet[scope.row.task_id]<0"
                        content="本地系统时间和服务端可能存在不一致，请同步。"
                        placement="top">
              <i class="el-icon-warning"
                 style="color:red"></i>
            </el-tooltip>
          </span>
        </template>
      </el-table-column>

      <el-table-column width="120"
                       prop="status"
                       label="状态"
                       :filters="[{ text: '失败', value: 'failed' }, { text: '成功', value: 'passed' },{ text: '超时', value: 'timeout' },{ text: '取消', value: 'cancelled' }]"
                       :filter-method="filterStatusTag"
                       filter-placement="bottom-end">
        <template slot-scope="scope">
          <el-tag :type="$utils.taskElTagType(scope.row.status)"
                  size="small"
                  effect="dark"
                  close-transition>
            {{ wordTranslation(scope.row.status,'pipeline','task') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column v-if="showOperation"
                       width="180"
                       label="操作">
        <template slot-scope="scope">
            <el-button @click="rerun(scope.row)" v-if="scope.row.workflow_args"
                       type="default"
                       icon="el-icon-copy-document"
                       size="mini">
              克隆任务
            </el-button>
            <el-button @click="rerun(scope.row)" v-else-if="scope.row.task_args"
                       type="default"
                       icon="el-icon-copy-document"
                       size="mini">
              克隆任务
            </el-button>
        </template>
      </el-table-column>
    </el-table>
    <div class="pagination">
      <el-pagination layout="prev, pager, next"
                     @current-change="changeTaskPage"
                     :page-size="pageSize"
                     :total="total">
      </el-pagination>
    </div>
  </div>
</template>

<script>
import { wordTranslate } from '@utils/word_translate.js';
import moment from 'moment';
export default {
  data() {
    return {
      durationSet: {},
    };
  },
  methods: {
    getDeployEnv(sub_tasks) {
      let env_name = '-';
      sub_tasks.some(task => {
        if (task.type === 'deploy' && task.enabled) {
          env_name = task.env_name;
          return true;
        }
      });
      return env_name;
    },
    convertTimestamp(value) {
      return moment.unix(value).format('MM-DD HH:mm');
    },
    wordTranslation(word, category, subitem) {
      return wordTranslate(word, category, subitem);
    },
    filterStatusTag(value, row) {
      return row.status === value;
    },
    taskDuration(task_id, started) {
      let refresh = () => {
        let duration = Math.floor(Date.now() / 1000) - started;
        this.$set(this.durationSet, task_id, duration);
      };
      setInterval(refresh, 1000);
      return '';
    },
    changeTaskPage(val) {
      this.$emit('currentChange', val);
    },
    rerun(task) {
      this.$emit('cloneTask', task);
    }
  },

  props: {
    taskList: {
      required: true,
      type: Array
    },
    baseUrl: {
      required: true,
      type: String
    },
    showEnv: {
      required: false,
      default: false,
      type: Boolean
    },
    showOperation: {
      required: false,
      default: false,
      type: Boolean
    },
    showServiceNames: {
      required: false,
      default: false,
      type: Boolean
    },
    workflowName: {
      required: false,
      default: '',
      type: String
    },
    total: {
      required: false,
      default: 0,
      type: Number
    },
    projectName: {
      required: false,
      default: '',
      type: String
    },
    pageSize: {
      required: false,
      default: 50,
      type: Number
    }

  }
};
</script>

