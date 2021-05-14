import 'normalize.css';
import 'ionicons/css/ionicons.css';
import 'loaders.css';
import 'element-ui/lib/theme-chalk/index.css';
import 'vant/lib/index.css';
import Vue from 'vue';
import VueResource from 'vue-resource';
import Element from 'element-ui';
import router from './router/index.js';
import store from './store';
import sse from './common/vue_sse';
import VueClipboard from 'vue-clipboard2';
import utils from '@utils/utilities';
import mobileGoback from '@utils/goback_mixin';
import onboardingStatusMixin from '@utils/save_onboarding_status_mixin';
import translate from '@utils/word_translate';
import '@utils/traversal';

import App from './App.vue';
import VueIntro from 'vue-introjs';
Vue.use(VueIntro);
import 'intro.js/introjs.css';
import { analyticsRequestAPI } from '@api';
import { JSEncrypt } from 'jsencrypt';

Vue.prototype.$utils = utils;
Vue.prototype.$translate = translate;
Vue.use(sse);

Vue.config.debug = true;
Vue.use(VueClipboard);
Vue.use(VueResource);
Vue.use(Element);
Vue.mixin(mobileGoback);
Vue.mixin(onboardingStatusMixin);
Vue.mixin({
  beforeDestroy() {
    const arr = window.__spockEventSources[this._uid];
    if (arr) {
      arr.forEach((src) => {
        src.userCount--;
        if (src.userCount === 0) {
          src.close();
        }
      });
    }
  },
});

const isSuperAdmin = () => {
  return utils.roleCheck().superAdmin;
};
const userName = () => {
  return utils.getUsername();
};
const analyticsRequest = (to, from) => {
  const hostname = window.location.hostname;
  if (to.path !== from.path && !utils.isPrivateIP(hostname)) {
    const publicKey = `-----BEGIN RSA PUBLIC KEY-----
    MIIBpTANBgkqhkiG9w0BAQEFAAOCAZIAMIIBjQKCAYQAz5IqagSbovHGXmUf7wTB
    XrR+DZ0u3p5jsgJW08ISJl83t0rCCGMEtcsRXJU8bE2dIIfndNwvmBiqh13/WnJd
    +jgyIm6i1ZfNmf/R8pEqVXpOAOuyoD3VLT9tfWvz9nPQbjVI+PsUHH7nVR0Jwxem
    NsH/7MC2O15t+2DVC1533UlhjT/pKFDdTri0mgDrLZHp6gPF5d7/yQ7cPbzv6/0p
    0UgIdStT7IhkDfsJDRmLAz09znv5tQQtHfJIMdAKxwHw9mExcL2gE40sOezrgj7m
    srOnJd65N8anoMGxQqNv+ycAHB9aI1Yrtgue2KKzpI/Fneghd/ZavGVFWKDYoFP3
    531Ga/CiCwtKfM0vQezfLZKAo3qpb0Edy2BcDHhLwidmaFwh8ZlXuaHbNaF/FiVR
    h7uu0/9B/gn81o2f+c8GSplWB5bALhQH8tJZnvmWZGI9OnrIlWmQZsuUBooTul9Q
    ZJ/w3sE1Zoxa+Or1/eWijqtIfhukOJBNyGaj+esFg6uEeBgHAgMBAAE=
    -----END RSA PUBLIC KEY-----`;
    let data = {
      domain: hostname,
      username: userName(),
      url: to.path,
      createdAt: Math.floor(Date.now() / 1000),
    };
    let encryptor = new JSEncrypt();
    encryptor.setPublicKey(publicKey);
    const encryptData = encryptor.encrypt(JSON.stringify(data));
    const payload = {
      data: encryptData,
    };
    analyticsRequestAPI(payload)
      .then((res) => {})
      .catch((err) => {
        console.log(err);
      });
  }
};

router.beforeEach((to, from, next) => {
  if (to.meta.title) {
    document.title = to.meta.title;
  } else {
    document.title = 'Zadig';
  }
  if (to.meta.requiresSuperAdmin) {
    if (!isSuperAdmin()) {
      Element.Notification({
        title: '非超级管理员',
        message: '无权访问',
        type: 'error',
      });
      next({
        path: from.fullPath,
      });
    } else {
      next();
    }
  } else {
    analyticsRequest(to, from);
    next();
  }
});

function mountApp() {
  const app = new Vue({
    router,
    store,
    components: { App },
    render: (h) => h(App),
  }).$mount('#app');

}
mountApp();
