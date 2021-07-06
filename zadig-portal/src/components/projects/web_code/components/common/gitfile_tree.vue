<template>
  <div class="git-file-container">
    <el-card
      class="box-card full git-file-card"
      v-loading="loading"
      :body-style="{ padding: '0px', margin: '0' }"
    >
      <el-tree
        ref="tree"
        v-if="showTree"
        :props="defaultProps"
        :highlight-current="true"
        :load="loadNode"
        :default-expand-all="false"
        node-key="name"
        show-checkbox
        :check-strictly="true"
        @check-change="handleCheckChange"
        lazy
      >
        <span class="gitfile-tree-node" slot-scope="{ node, data }">
          <span class="folder-icon">
            <i :class="{ 'el-icon-folder': data.is_dir }"></i>
          </span>
          <el-tooltip
            v-if="!data.is_dir"
            effect="dark"
            :content="node.label"
            placement="top"
          >
            <span class="file-name">{{ $utils.tailCut(node.label, 40) }}</span>
          </el-tooltip>
          <span v-else class="file-name">{{ node.label }}</span>
          <span class="basic-info">
            <!-- <span v-show="data.name!=='/'&& !data.is_dir"
                  class="size">{{ data.size + ' Bytes'}}</span> -->
            <span v-show="data.name !== '/'" class="mod-time">{{
              $utils.convertTimestamp(data.mod_time)
            }}</span>
          </span>
        </span>
      </el-tree>
      <div>
        <span class="clean-workspace">
          <el-button size="small" @click="selectFile" type="primary" plain
            >确定</el-button
          >
        </span>
      </div>
    </el-card>
  </div>
</template>

<script>
import { getRepoFilesAPI, getPublicRepoFilesAPI } from '@api'
export default {
  props: {
    codehostId: {
      type: Number,
      default: 0
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
    showTree: {
      type: Boolean,
      default: false,
      required: true
    },
    closeFileTree: Function,
    changeSelectPath: Function,
    type: String,
    url: String
  },
  data () {
    return {
      checkedData: [],
      fileTree: [],
      loading: false,
      innerVisible: false,
      deleteLoading: false,
      selectPath: '',
      isDir: true,
      defaultProps: {
        children: 'children',
        label: 'name',
        isLeaf: 'leaf'
      }
    }
  },
  methods: {
    handleCheckChange (data, checked, indeterminate) {
      this.checkedData = this.$refs.tree.getCheckedNodes()
    },
    loadNode (node, resolve) {
      const codehostId = this.codehostId
      const repoOwner = this.repoOwner
      const repoName = this.repoName
      const branchName = this.branchName
      const url = this.url
      const type = 'gerrit'
      let path = ''
      if (node.data && node.data.parent === '/') {
        path = node.data.parent + node.data.name
      } else if (node.data) {
        path = node.data.parent + '/' + node.data.name
      }
      this.selectPath = ''
      this.loading = true
      if (this.type === 'private') {
        getRepoFilesAPI(codehostId, repoOwner, repoName, branchName, path, type)
          .then((res) => {
            res.forEach((element) => {
              if (element.is_dir) {
                element.leaf = false
              } else {
                element.leaf = true
              }
            })
            return resolve(res)
          })
          .finally(() => {
            this.loading = false
          })
      } else if (this.type === 'public') {
        getPublicRepoFilesAPI(path, url)
          .then((res) => {
            res.forEach((element) => {
              if (element.is_dir) {
                element.leaf = false
              } else {
                element.leaf = true
              }
            })
            return resolve(res)
          })
          .finally(() => {
            this.loading = false
          })
      }
    },
    async selectFile () {
      const checkList = []
      this.checkedData.forEach((item) => {
        if (item.is_dir) {
          if (item.parent === '/') {
            checkList.push(item.parent + item.name)
          } else {
            checkList.push(item.parent + '/' + item.name)
          }
        }
      })
      this.changeSelectPath(checkList)
    }
  }
}
</script>

<style lang="less" >
.git-file-container {
  position: relative;
  padding: 10px 5px;
  overflow: auto;
  font-size: 13px;
  background-color: #fff;

  .git-file-card {
    margin: 0;
  }

  .el-tree--highlight-current .el-tree-node.is-current > .el-tree-node__content {
    background-color: #1989fa33;
  }

  .el-tree {
    max-height: 400px;
    overflow: auto;
  }

  .el-tree-node {
    margin: 5px 0;

    .gitfile-tree-node {
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

  .clean-workspace {
    display: inline-block;
    margin-top: 10px;
    margin-bottom: 15px;
    margin-left: 20px;
  }
}
</style>
