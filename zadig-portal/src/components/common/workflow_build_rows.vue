<template>
  <div class="workflow-build-rows">
    <el-table :data="buildV2" v-if="buildV2.length > 0"
              empty-text="无"
              class="service-deploy-table">
      <el-table-column prop="name"
                       label="服务"
                       width="100px"
                       ></el-table-column>

      <el-table-column label="代码库"  >
        <template slot-scope="scope"  v-if="scope.row.build" >
          <el-row v-for="build of scope.row.build.repos"
                  class="build-row"
                  :key="build._id_">
            <template v-if="!build.use_default">
              <el-col :span="7">
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
                             placeholder="请选择 Tag"
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
              <el-col v-if="build.showSwitch"
                      :span="9"
                      :offset="1"
                      style="line-height:32px">
                <el-switch v-model="build.releaseMethod"
                           @change="changeReleaseMethod(build)"
                           active-text="Branch"
                           inactive-text="Tag"
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
              <el-tooltip v-if="build.errorMsg"
                          class="item"
                          effect="dark"
                          :content="build.errorMsg"
                          placement="top">
                <i class="el-icon-question repo-warning"></i>
              </el-tooltip>
            </template>
          </el-row>
        </template>
      </el-table-column>

      <el-table-column   width="250px">
        <template slot="header"
                  slot-scope="scope">
          部署
          <deploy-icons></deploy-icons>
        </template>
        <template slot-scope="scope">
          <div v-for="deploy of scope.row.deploy"
               :key="`${deploy.env}|${deploy.type}`">
            <el-checkbox v-if="deploy.type === 'k8s'|| deploy.type === 'helm'"
                         v-model="deploy.picked"
                         size="small">
              <i v-if="deploy.type === 'k8s'"
                 class="iconfont iconrongqifuwu"></i>
              <i v-if="deploy.type === 'helm'"
                 class="iconfont iconhelmrepo"></i>
              {{ deploy.env }}
            </el-checkbox>
            <template v-else-if="deploy.type === 'pm'">
              <i class="iconfont iconwuliji"></i>
              {{ deploy.env }}
            </template>

          </div>
        </template>
      </el-table-column>
      <el-table-column width="100px"
                       :label="pickedTargets[0].jenkins_build_args ? '变量': '环境变量'">
        <template slot-scope="scope">
          <el-popover placement="left"
                      width="450"
                      trigger="click">
            <!-- 非jenkins构建 -->
            <el-table :data="scope.row.envs" v-if="!scope.row.jenkins_build_args">
              <el-table-column property="key"
                               label="Key"></el-table-column>
              <el-table-column label="Value">
                <template slot-scope="scope">
                  <el-input size="small"
                            v-model="scope.row.value"
                            placeholder="请输入 value"></el-input>
                </template>
              </el-table-column>
            </el-table>
            <!-- jenkins构建 -->
            <el-table :data="scope.row.jenkins_build_args.jenkins_build_params" v-if="scope.row.jenkins_build_args">
              <el-table-column property="name"
                               label="name"></el-table-column>
              <el-table-column label="Value">
                <template slot-scope="scope">
                  <el-input size="small"
                            v-model="scope.row.value"
                            placeholder="请输入 value"></el-input>
                </template>
              </el-table-column>
            </el-table>
            <el-button style="padding:5px 0px"
                       slot="reference"
                       type="text">设置</el-button>
          </el-popover>
        </template>
      </el-table-column>
    </el-table>
    <el-table :data="jenkinsBuild" v-if="jenkinsBuild.length > 0"
              empty-text="无"
              class="service-deploy-table">
      <el-table-column prop="name"
                       label="服务"
                       width="100px"
                       ></el-table-column>
      <el-table-column 
                       label="Jenkins Job Name"
                       >
          <div slot-scope="scope">
            {{scope.row.jenkins_build_args.job_name}}
          </div>
      </el-table-column>

      <el-table-column   width="250px">
        <template slot="header"
                  slot-scope="scope">
          部署
          <deploy-icons></deploy-icons>
        </template>
        <template slot-scope="scope">
          <div v-for="deploy of scope.row.deploy"
               :key="`${deploy.env}|${deploy.type}`">
            <el-checkbox v-if="deploy.type === 'k8s'|| deploy.type === 'helm'"
                         v-model="deploy.picked"
                         size="small">
              <i v-if="deploy.type === 'k8s'"
                 class="iconfont iconrongqifuwu"></i>
              <i v-if="deploy.type === 'helm'"
                 class="iconfont iconhelmrepo"></i>
              {{ deploy.env }}
            </el-checkbox>
            <template v-else-if="deploy.type === 'pm'">
              <i class="iconfont iconwuliji"></i>
              {{ deploy.env }}
            </template>

          </div>
        </template>
      </el-table-column>
      <el-table-column width="100px"
                       label="变量">
        <template slot-scope="scope">
          <el-popover placement="left"
                      width="450"
                      trigger="click">
            <!-- jenkins构建 -->
            <el-table :data="scope.row.jenkins_build_args.jenkins_build_params">
              <el-table-column property="name"
                               label="name"></el-table-column>
              <el-table-column label="Value">
                <template slot-scope="scope">
                  <el-input size="small"
                            v-model="scope.row.value"
                            placeholder="请输入 value"></el-input>
                </template>
              </el-table-column>
            </el-table>
            <el-button style="padding:5px 0px"
                       slot="reference"
                       type="text">设置</el-button>
          </el-popover>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
import deployIcons from './deploy_icons';

export default {
  data() {
    return {
      buildV2: [],
      jenkinsBuild: []
    };
  },
  methods: {
    changeReleaseMethod(repo) {
      repo.tag = '';
      repo.branch = '';
    },
  },
  props: {
    pickedTargets: {
      type: Array,
      required: true,
    },
  },
  components: {
    deployIcons
  },
  watch: {
    pickedTargets:{
      handler(value) {
      this.buildV2 = value.filter(item => !item.jenkins_build_args)
      this.jenkinsBuild = value.filter(item => item.jenkins_build_args)
      },
      immediate: true

    }
  }
};
</script>

<style lang="less">
@import '~@assets/css/common/build-row.less';
</style>
