import * as types from '../mutations';
import { productTemplatesAPI } from '@api';
import { sortBy } from 'lodash';

const state = {
  templates: []
};

const getters = {
  getTemplates: (state) => {
    return state.templates;
  },
  getOnboardingTemplates: (state) => {
    return state.templates
      .filter((temp) => {
        return temp.onboarding_status;
      })
      .map((temp) => temp.product_name);
  },
};

const mutations = {
  [types.SET_PROJECT_TEMPLATES](state, payload) {
    state.templates = payload;
  }
};

const actions = {
  async getProjectTemplates({ commit, state, rootGetters }) {
    return await productTemplatesAPI().then(
      (res) => {
        commit(types.SET_PROJECT_TEMPLATES, sortBy(res, 'product_name'));
      },
      () => {
      }
    );
  },
  refreshProjectTemplates({ commit, state, rootGetters, dispatch }) {
    commit(types.SET_PROJECT_TEMPLATES, []);
    return dispatch('getProjectTemplates');
  },
  clearProjectTemplates({ commit, state, rootGetters }) {
    commit(types.SET_PROJECT_TEMPLATES, []);
  },
};

export default {
  state,
  getters,
  actions,
  mutations,
};
