export default {
  methods: {
    mobileGoback() {
      this.$router ? this.$router.back() : window.history.back();
    },
  },
};
