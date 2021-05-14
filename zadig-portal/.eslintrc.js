module.exports = {
  parserOptions: {
    parser: "babel-eslint"
  },
  extends: [
    'standard',
    'plugin:vue/recommended'
    // 'plugin:react/recommended'
  ],
  env: {
    'browser': true,
    'node': true
 },
  globals: {
    $: true,
    jQuery: true,
    jquery: true
 },
  rules: {
    // override/add rules settings here, such as:
    // 'vue/no-unused-vars': 'error'
    "no-cond-assign": 'off',
    "comma-dangle": 'off',
    "no-delete-var": 'off',
    semi: 'off',
    "no-fallthrough": 'off',
    "no-floating-decimal": 'off',
    "no-mixed-operators": 'warn',

    eqeqeq: 'off',
    indent: ['error', 2],
    'space-before-function-paren': 'off',
    'keyword-spacing': 'warn',
    'semi-spacing': 'off',
    'space-infix-ops': 'off',
    'camelcase': 'off',
    // 'keyword-spacing': ['error', { overrides: {
    //   if: { after: false },
    //   for: { after: false },
    //   while: { after: false }
    // } }]

    'vue/max-attributes-per-line': 'off',
    'vue/attributes-order': 'off',
    'vue/html-self-closing': 'off',
    'vue/attribute-hyphenation': 'off',
    'vue/order-in-components': 'off',

    'react/prop-types': 'off'
  }
}
