<template>
  <div class="workflow-build-rows">
    <el-row v-for="(build,index) of builds"
            class="build-row"
            :key="build._id_">
      <template v-if="!build.use_default">
        <el-col :span="6">
          <div class="repo-name-container">
            <span :class="{'repo-name': true}"> {{
              $utils.tailCut(build.repo_name,20) }}</span>
          </div>
        </el-col>
        <template v-if="build.showBranch">
          <el-col :span="7">
            <el-select v-if="build.branchNames && build.branchNames.length > 0"
                       v-model="build.branch"
                       filterable
                       clearable
                       allow-create
                       size="small"
                       placeholder="请选择分支">
              <el-option v-for="branch of build.branchNames"
                         :key="branch"
                         :label="branch"
                         :value="branch"></el-option>
            </el-select>
            <el-tooltip v-else
                        content="请求分支失败，请手动输入分支"
                        placement="top"
                        popper-class="gray-popper">
              <el-input v-model="build.branch"
                        class="short-input"
                        size="small"
                        placeholder="请填写分支"></el-input>
            </el-tooltip>
          </el-col>
        </template>

        <template v-if="build.showTag">
          <el-col :span="7">
            <el-select v-if="build.tags && build.tags.length > 0"
                       v-model="build.tag"
                       size="small"
                       placeholder="请选择标签"
                       filterable
                       clearable>
              <el-option v-for="(item,index) in build.tags"
                         :key="index"
                         :label="item.name"
                         :value="item.name">
              </el-option>
            </el-select>
            <el-tooltip v-else
                        content="请求 Release Tag 失败，支持手动输入 Release Tag"
                        placement="top"
                        popper-class="gray-popper">
              <el-input v-model="build.tag"
                        class="short-input"
                        size="small"
                        placeholder="请填写 Tag"></el-input>
            </el-tooltip>
          </el-col>
        </template>
        <el-col :span="7"
                :offset="1"
                style="line-height:32px">
          <el-switch v-if="build.showSwitch"
                     v-model="build.releaseMethod"
                     @change="changeReleaseMethod(build)"
                     active-text="分支"
                     inactive-text="标签"
                     active-value="branch"
                     inactive-value="tag"
                     active-color="#dcdfe6"
                     inactive-color="#dcdfe6">
          </el-switch>
        </el-col>
        <template v-if="build.showPR">
          <el-col :span="7"
                  :offset="1">
            <el-select v-if="!$utils.isEmpty(build.branchPRsMap)"
                       v-model.number="build[build.prNumberPropName]"
                       size="small"
                       placeholder="请选择 PR"
                       filterable
                       clearable>

              <el-tooltip v-for="item in build.branchPRsMap[build.branch]"
                          :key="item[build.prNumberPropName]"
                          placement="left"
                          popper-class="gray-popper">

                <div slot="content">{{`创建人: ${$utils.tailCut(item.authorUsername,10)}`}}
                  <br />{{`时间: ${$utils.convertTimestamp(item.createdAt)}`}}
                  <br />{{`源分支: ${item.sourceBranch}`}}
                  <br />{{`目标分支: ${item.targetBranch}`}}
                </div>
                <el-option :label="`#${item[build.prNumberPropName]} ${item.title}`"
                           :value="item[build.prNumberPropName]">
                </el-option>
              </el-tooltip>
            </el-select>
            <el-tooltip v-else
                        content="PR 不存在，支持手动输入 PR 号"
                        placement="top"
                        popper-class="gray-popper">
              <el-input v-model.number="build[build.prNumberPropName]"
                        class="short-input"
                        size="small"
                        placeholder="请填写 PR 号"></el-input>
            </el-tooltip>
          </el-col>
        </template>
      </template>
    </el-row>
  </div>
</template>

<script>
export default {
  data() {
    return {
    };
  },
  methods: {
    changeReleaseMethod(repo) {
      repo.tag = '';
      repo.branch = '';
    },
  },
  props: {
    builds: {
      type: Array,
      required: true,
    },
  },
};
</script>

<style lang="less">
.gray-popper {
  background-color: rgb(92, 92, 92) !important;
  &[x-placement^="top"] .popper__arrow::after {
    border-top-color: rgb(92, 92, 92) !important;
  }
  &[x-placement^="bottom"] .popper__arrow::after {
    border-bottom-color: rgb(92, 92, 92) !important;
  }
  &[x-placement^="left"] .popper__arrow::after {
    border-left-color: rgb(92, 92, 92) !important;
  }
  &[x-placement^="right"] .popper__arrow::after {
    border-right-color: rgb(92, 92, 92) !important;
  }
}
.workflow-build-rows {
  .build-row {
    padding: 5px 0px;
  }
  .repo-name-container {
    .repo-name {
      max-width: 100%;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
      line-height: 32px;
      &.adjust {
        line-height: 57px;
      }
    }
    .namespace {
      line-height: 32px;
    }
  }
  .build-row {
    &:not(:first-child) {
      margin-top: 5px;
    }
  }
}
</style>
