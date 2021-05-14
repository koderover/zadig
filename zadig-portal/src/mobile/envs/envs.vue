<template>
  <div class="mobile-env">
    <van-nav-bar>
      <template #title>
        集成环境
      </template>
    </van-nav-bar>
    <van-cell v-for="(item,index) in products"
              :key="index"
              is-link
              :to="item.url"
              icon="apps-o"
              size="large">
      <template #title>
        <span>{{item.name}}</span>
      </template>
    </van-cell>
  </div>
</template>
<script>
import { NavBar, Empty, Cell, CellGroup } from 'vant';
import { mapGetters } from 'vuex';
import { uniqBy, orderBy } from 'lodash';
export default {
  components: {
    [NavBar.name]: NavBar,
    [Empty.name]: Empty,
    [Cell.name]: Cell,
    [CellGroup.name]: CellGroup
  },
  data() {
    return {
      products: []
    }
  },
  computed: {
    ...mapGetters([
      'productList'
    ]),
    filteredProducts() {
      return uniqBy(orderBy(this.productList, ['product_name', 'is_prod']), 'product_name');
    },
  },
  methods: {
    async getProducts() {
      let firstMessage = true;
      const availableProjectNames = [];

      await this.$store.dispatch('getProductListSSE').closeWhenDestroy(this);
      for (let product of this.productList) {
          availableProjectNames.push(product.product_name);
      }
      this.products = this.filteredProducts.map(element => {
        return { name: element.product_name, url: `/mobile/envs/detail/${element.product_name}?envName=${element.env_name}` }
      });
    }
  },
  mounted() {
    this.getProducts();
  },

}
</script>
<style lang="less">
</style>