import * as types from '../mutations';
import { checkOrganizationHasSignupAPI } from '@api';

const state = {
  status: null
};

const getters = {
  signupStatus: state => {
    return state.status;
  }
};

const mutations = {
  [types.SET_SIGNUP_STATUS](state, payload) {
    state.status = payload;
  }
};

const actions = {
  getSignupStatus({ commit, state, rootGetters }) {
    return checkOrganizationHasSignupAPI().then(res => {
      commit(types.SET_SIGNUP_STATUS, res);
    });
  }
};

export default {
  state,
  getters,
  actions,
  mutations
};
