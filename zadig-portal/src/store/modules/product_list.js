import * as types from '../mutations';
import { listProductSSEAPI } from '../../api';

const state = {
  productList: []
};

const getters = {
  productList: state => {
    return state.productList;
  }
};

const mutations = {
  [types.SET_PRODUCT_LIST](state, payload) {
    state.productList = payload;
  }
};

let theSSEPromise;

const actions = {
  getProductListSSE ({ commit, state }) {
    if(theSSEPromise && theSSEPromise.isAlive()) {
      return theSSEPromise;
    } else {
      theSSEPromise = listProductSSEAPI().then(res => {
        commit(types.SET_PRODUCT_LIST, res.data);
      });
      return theSSEPromise;
    }
  }
};

export default {
  state,
  getters,
  actions,
  mutations
};
