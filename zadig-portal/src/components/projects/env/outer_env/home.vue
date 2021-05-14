<template>
  <div class="container-home"
       v-loading="loading"
       element-loading-text="加载中..."
       element-loading-spinner="iconfont iconfont-loading iconrongqi">
    <div v-if="loading || emptyEnvs"
         class="no-show">
      <img src="@assets/icons/illustration/environment.svg"
           alt="" />
      <p v-if="emptyEnvs">暂无环境，请到项目中创建环境！</p>
    </div>
    <router-view v-else></router-view>
  </div>
</template>

<script>
import { mapGetters } from 'vuex';
import { uniqBy, orderBy } from 'lodash';
import bus from '@utils/event_bus';
export default {
  data() {
    return {
      loading: true,
      firstJumpPath: '',
      emptyEnvs: false
    }
  },
  computed: {
    ...mapGetters([
      'productList', 'getOnboardingTemplates'
    ]),
    filteredProducts() {
      return uniqBy(orderBy(this.productList, ['product_name', 'is_prod']), 'product_name');
    },
    isInProject() {
      return this.$route.path.includes('/projects/detail/');
    },
  },
  methods: {
    async getProducts() {
      const availableProjectNames = [];

      await this.$store.dispatch('getProductListSSE').closeWhenDestroy(this);
      for (let product of this.productList) {
          availableProjectNames.push(product.product_name);
      }
      const routerList = this.filteredProducts.filter(product => {
        return !this.getOnboardingTemplates.includes(product.product_name);
      }).map(element => {
        return { name: element.product_name, url: `/v1/envs/detail/${element.product_name}?envName=${element.env_name}` }
      });
      bus.$emit(`set-sub-sidebar-title`, {
        'title': '项目列表',
        'routerList': routerList
      });
      const availableProducts = this.productList.filter(item => {
        return availableProjectNames.includes(item.product_name);
      });

      this.loading = false;
      if (availableProducts.length === 0) {
        this.emptyEnvs = true;
        return;
      }
      this.firstJumpPath = `/v1/envs/detail/${availableProducts[0].product_name}?envName=${availableProducts[0].env_name}`;
      if (!this.$route.params.project_name) {
        this.$router.push(this.firstJumpPath);
      }
    }
  },
  beforeRouteUpdate(to, from, next) {
    if (!this.firstJumpPath) {
      next();
    } else if (to.path === '/v1/envs' || !to.params.project_name) {
      next(this.firstJumpPath);
    } else {
      next();
    }
  },
  mounted() {
    this.getProducts();
    bus.$emit(`set-topbar-title`, { title: '集成环境', breadcrumb: [] });
  }
};
</script>

<style lang="less" >
.container-home {
  flex: 1;
  position: relative;
  overflow: hidden;
  padding: 15px 20px;
  display: flex;
  .no-show {
    margin: auto;
    img {
      width: 460px;
      height: 460px;
    }
    p {
      text-align: center;
      color: #8d9199;
      font-size: 15px;
    }
  }
}
</style>
