<template>
  <div v-loading="loading"
       class="download-artifact-container">

    <el-table :data="fileList"
              style="width: 100%;">
      <el-table-column label="文件列表">
        <template slot-scope="scope">
          <span>{{ scope.row }}</span>
        </template>
      </el-table-column>
    </el-table>
    <div>
      <span class="download">
        <a :href="`/api/aslan/workflow/v2/tasks/workflow/${workflowName}/taskId/${taskId}`"
           download>
          <el-button size="small"
                     type="primary"
                     :disabled="fileList.length===0"
                     plain>下载</el-button>
        </a>

      </span>
    </div>
  </div>
</template>

<script>
import { getArtifactWorkspaceAPI } from '@api'
export default {
  props: {
    workflowName: {
      type: String,
      required: true
    },

    taskId: {
      type: String,
      required: true
    },
    showArtifact: {
      type: Boolean,
      default: false,
      required: true
    }
  },
  data () {
    return {
      fileTree: [],
      fileList: [],
      loading: true,
      innerVisible: false,
      deleteLoading: false,
      selectPath: '',
      defaultProps: {
        children: 'children',
        label: 'name',
        isLeaf: 'leaf'
      }
    }
  },
  methods: {
    getArtifactWorkspace () {
      this.loading = true
      const workflowName = this.workflowName
      const taskId = this.taskId
      getArtifactWorkspaceAPI(workflowName, taskId).then((res) => {
        this.loading = false
        this.fileList = res
      })
    }
  },
  computed: {},
  mounted () {
    this.getArtifactWorkspace()
  },
  components: {}
}
</script>

<style lang="less" >
.download-artifact-container {
  position: relative;
  padding: 0 10px;
  overflow: auto;
  font-size: 13px;
  background-color: #fff;

  .el-tree--highlight-current .el-tree-node.is-current > .el-tree-node__content {
    background-color: #1989fa33;
  }

  .el-tree-node {
    margin: 5px 0;

    .artifact-tree-node {
      position: relative;
      display: inline-block;
      width: 100%;
      line-height: 22px;

      .folder-icon {
        display: inline-block;
        font-size: 16px;
      }

      .file-name {
        display: inline-block;
        font-size: 15px;
      }

      .basic-info {
        float: right;
        padding-right: 40px;

        .mod-time,
        .size {
          padding-left: 35px;
          color: #c0c4cc;
        }
      }
    }
  }

  .download {
    display: inline-block;
    margin-top: 10px;
    margin-bottom: 15px;
  }
}
</style>
