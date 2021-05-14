import * as types from '../mutations';
import { listWorkflowAPI } from '@api';

const state = {
  workflowList: [],
};

const getters = {
  workflowList: (state) => {
    return state.workflowList;
  },
};

const mutations = {
  [types.SET_WORKFLOW_LIST](state, payload) {
    state.workflowList = payload;
  },
};

let requestStarted = false;
let resolveGet;
let theGetPromise = new Promise((resolve, reject) => {
  resolveGet = resolve;
});

const actions = {
  getWorkflowList({ commit, state }) {
    if (requestStarted) {
      return theGetPromise;
    } else {
      requestStarted = true;
      return doGet({ commit, state });
    }
  },
  refreshWorkflowList({ commit, state }) {
    return doGet({ commit, state });
  },
};

function doGet({ commit, state }) {
  return listWorkflowAPI().then((res) => {
    res.map((element) => {
      element.type = 'product';
    });
    commit(types.SET_WORKFLOW_LIST, res);
    resolveGet();
  });
}

export default {
  state,
  getters,
  actions,
  mutations,
};
