import { saveOnboardingStatusAPI } from '@api';
export default {
  methods: {
    saveOnboardingStatus(projectName, status) {
      return new Promise((resolve, reject) => {
        saveOnboardingStatusAPI(projectName, status)
          .then((res) => {
            if (res.message !== 'success') {
              this.$message.error(`${projectName}/${status}状态保存失败：${res}`);
              reject(res.message);
            }
            resolve('ok');
          })
          .catch((err) => {
            this.$message.error(`${projectName}/${status}状态保存失败：${err}`);
            reject(err);
          })
          .then(() => {
            this.$store.dispatch('refreshProjectTemplates');
          });
      });
    },
  },
  created() {
    var onboardingStatus = this.$options.onboardingStatus;
    if (onboardingStatus !== undefined) {
      this.saveOnboardingStatus(this.$route.params.project_name, onboardingStatus);
    }
  },
};
