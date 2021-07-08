<template>
  <div class="hello">
    <h1>{{ msg }}</h1>
    <h2>{{ description }} </h2>
    <ul>
      <li>
        前端构建时间： 2021-07-08.11:03:25
      </li>
    </ul>
    <ul>
      <li>
        后端构建时间： {{ buildstamp.data }}
      </li>
    </ul>
    <h3>推荐链接：</h3>
    <ul>
      <li>
        <a
          href="https://www.koderover.com/"
          target="_blank"
        >
          KodeRover 官网
        </a>
      </li>
      <li>
        <a
          href="https://github.com/koderover/zadig"
          target="_blank"
        >
          GitHub
        </a>
      </li>
      <br>
    </ul>
  </div>
</template>

<script>
import axios from 'axios'
export default {
  name: 'HelloWorld',
  data () {
    return {
      msg: '欢迎来到 microservice-demo 项目',
      description: '前端采用 Vue.js 实现，后端采用 Golang 实现',
      buildstamp: ""
    }
  },
  mounted () {
    axios
      .get('/api/buildstamp')
      .then(response => (this.buildstamp= response))
  }
}

</script>

<!-- Add "scoped" attribute to limit CSS to this component only -->
<style scoped>
h1, h2 {
  font-weight: normal;
}
ul {
  list-style-type: none;
  padding: 0;
}
li {
  display: inline-block;
  margin: 0 10px;
}
a {
  color: #42b983;
}
</style>
