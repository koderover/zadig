import * as types from '../mutations';

const state = {
  showSidebar: true
};

const getters = {
  showSidebar: state => {
    return state.showSidebar;
  }
};

const mutations = {
  [types.SET_SIDEBAR_STATUS](state, payload) {
    state.showSidebar = payload;
  }
};

const actions = {

};

export default {
  state,
  getters,
  actions,
  mutations
};
