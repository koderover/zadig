<template>
  <div class="git-file-container">
    <el-card class="box-card full git-file-card"
             v-loading="loading"
             :body-style="{ padding: '0px', margin: '0' }">
      <el-tree v-if="showTree"
               :props="defaultProps"
               :highlight-current="true"
               :load="loadNode"
               :default-expand-all="false"
               node-key="name"
               @node-click="clickNode"
               lazy>
        <span class="gitfile-tree-node"
              slot-scope="{ node, data }">
          <span class="folder-icon">
            <i :class="{'el-icon-folder':data.is_dir}"></i>
          </span>
          <el-tooltip v-if="!data.is_dir"
                      effect="dark"
                      :content="node.label"
                      placement="top">
            <span class="file-name">{{ $utils.tailCut(node.label,40)}}</span>
          </el-tooltip>
          <span v-else
                class="file-name">{{ node.label}}</span>
          <span class="basic-info">
            <span v-show="data.name!=='/'"
                  class="mod-time">{{$utils.convertTimestamp(data.mod_time)}}</span>
          </span>
        </span>
      </el-tree>
      <div>
        <span class="clean-workspace">
          <el-button size="small"
                     @click="selectFile"
                     type="primary"
                     :disabled="selectPath===''"
                     plain>确定</el-button>
        </span>
      </div>

    </el-card>
  </div>
</template>

<script>
import { getRepoFilesAPI, getRepoFileServiceAPI } from '@api';
export default {
  props: {
    codehostId: {
      type: Number,
      default: '',
      required: true
    },
    repoName: {
      type: String,
      default: '',
      required: true
    },
    repoOwner: {
      type: String,
      default: '',
      required: true
    },
    branchName: {
      type: String,
      default: '',
      required: true
    },
    remoteName: {
      type: String,
      default: '',
      required: true
    },
    gitType: {
      type: String,
      default: 'gitlab',
      required: true
    },
    showTree: {
      type: Boolean,
      default: false,
      required: true
    }
  },
  data() {
    return {
      fileTree: [],
      loading: true,
      innerVisible: false,
      deleteLoading: false,
      selectPath: '',
      isDir: true,
      defaultProps: {
        children: 'children',
        label: 'name',
        isLeaf: 'leaf'
      }
    };
  },
  methods: {
    loadNode(node, resolve) {
      const codehostId = this.codehostId;
      const repoOwner = this.repoOwner;
      const repoName = this.repoName;
      const branchName = this.branchName;
      const remoteName = this.remoteName;
      const type = this.gitType;
      let path = '';
      if (type === 'gerrit') {
        path = node.data ? (node.data.parent + '/' + node.data.name) : '';
      }
      else {
        path = node.data ? node.data.full_path : '';
      }
      if (path === '' && type !== "gerrit") {
        this.loading = true;
      }
      this.selectPath = '';
      getRepoFilesAPI(codehostId, repoOwner, repoName, branchName, path, type).then((res) => {
        if (path === '') {
          this.loading = false;
        }
        res.map(element => {
          if (element.is_dir) {
            element.leaf = false;
          }
          else {
            element.leaf = true;
          }
        });
        return resolve(res);
      });

    },
    clickNode(data, node) {
      const type = this.gitType;
      if (type === 'gerrit') {
        this.selectPath = (data.parent + '/' + data.name).substr(2);
      }
      else {
        this.selectPath = node.data.full_path;
      }
      this.isDir = node.data.is_dir;
    },
    selectFile() {
      const codehostId = this.codehostId;
      const repoOwner = this.repoOwner;
      const repoName = this.repoName;
      const branchName = this.branchName;
      const path = this.selectPath;
      const isDir = this.isDir;
      const remoteName = this.remoteName;
      getRepoFileServiceAPI(codehostId, repoOwner, repoName, branchName, path, isDir, remoteName).then((res) => {
        this.$emit('getPreloadServices', {
          path: path,
          services: res,
          isDir: isDir
        });
      });
    },
    append(data) { },
    remove(node, data) { }
  },
  computed: {},
  mounted() {

  },
  destroyed() {
  },
  components: {}
};
</script>

<style lang="less" >
.git-file-container {
  background-color: #fff;
  position: relative;
  overflow: auto;
  padding: 10px 5px;
  font-size: 13px;
  .git-file-card {
    margin: 0;
  }
  .el-tree--highlight-current
    .el-tree-node.is-current
    > .el-tree-node__content {
    background-color: #1989fa33;
  }
  .el-tree {
    overflow: auto;
    max-height: 400px;
  }
  .el-tree-node {
    margin: 5px 0px;
    .gitfile-tree-node {
      display: inline-block;
      position: relative;
      width: 100%;
      line-height: 22px;
      .folder-icon {
        font-size: 16px;
        display: inline-block;
      }
      .file-name {
        font-size: 15px;
        display: inline-block;
      }
      .basic-info {
        display: inline-block;
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
  .clean-workspace {
    display: inline-block;
    margin-left: 20px;
    margin-bottom: 15px;
    margin-top: 10px;
  }
}
</style>
