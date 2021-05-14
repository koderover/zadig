import * as types from '../mutations';

const state = {
  userinfo: {
    info: {
      id: 0,
      name: '',
      email: '',
      password: '',
      phone: '',
      isAdmin: true,
      isSuperUser: false,
      isTeamLeader: false,
      organization_id: 0,
      directory: '',
      teams: []
    },
    teams: [],
    organization: {
      id: 1,
      name: '',
      token: '',
      website: ''
    }
  }
};

const getters = {};

const actions = {};

const mutations = {
  [types.INJECT_PROFILE](state, profile) {
    try {
      let localStorage = window.localStorage;
      let storeBaseInfo = data => {
        localStorage.setItem('ZADIG_LOGIN_INFO', JSON.stringify(data));
      };
      let readBaseInfo = name => JSON.parse(localStorage.getItem(name));

      storeBaseInfo(profile);
      state.userinfo = readBaseInfo('ZADIG_LOGIN_INFO');
    } catch (err) {
      console.log(err);
    }
  }
};

export default {
  state,
  getters,
  actions,
  mutations
};
