module.exports = {
  env: {
    browser: true,
    es2021: true
  },
  extends: ['plugin:vue/essential', 'standard'],
  parserOptions: {
    ecmaVersion: 12,
    sourceType: 'module'
  },
  plugins: ['vue'],
  rules: {
    camelcase: 0,
    'prefer-promise-reject-errors': [
      0,
      {
        allowEmptyReject: true
      }
    ],
    'no-mixed-operators': 1,
    'no-duplicate-case': 2,
    'no-unreachable': 2,
    'no-dupe-args': 2,
    'no-fallthrough': 2,
    'no-dupe-class-members': 2,
    'no-delete-var': 2,
    'no-unused-vars': [1, { vars: 'local', args: 'none' }],
    'no-empty-function': 1,
    'no-lone-blocks': 2,
    'vue/v-on-style': [2, 'shorthand'],
    curly: [2, 'multi-line'],
    'brace-style': [2, '1tbs', { allowSingleLine: true }],
    'comma-dangle': [2, 'never'],
    quotes: [2, 'single', {
      avoidEscape: true,
      allowTemplateLiterals: true
    }],
    'no-extra-parens': [2, 'functions'],
    eqeqeq: [1, 'allow-null'],
    indent: [2, 2, { SwitchCase: 1 }],
    'no-whitespace-before-property': 2,
    'generator-star-spacing': [2, { before: true, after: true }],
    'comma-spacing': [2, { before: false, after: true }],
    'comma-style': [2, 'last'],
    'func-call-spacing': 2,
    'space-before-blocks': [2, 'always'],
    'space-infix-ops': 2,
    'spaced-comment': [2, 'always', { markers: ['global', 'globals', 'eslint', 'eslint-disable', '*package', '!', ','] }],
    'no-irregular-whitespace': 2,
    'no-var': 2,
    'consistent-this': [2, '_this'],
    'no-label-var': 2,
    'no-shadow-restricted-names': 2,
    'no-undef': 0,
    'no-warning-comments': 0,
    'vue/no-unused-properties': 0,
    'no-restricted-imports': 0,
    'vue/no-unused-components': 0,
    'vue/component-name-in-template-casing': 0,
    'vue/html-self-closing': 0,
    'vue/no-mutating-props': 0,
    'vue/no-side-effects-in-computed-properties': 0,
    'vue/custom-event-name-casing': 0,
    'vue/no-use-v-if-with-v-for': 2,
    'node/handle-callback-err': 1,
    'no-case-declarations': 0,
    'no-invalid-regexp': 0,
    'no-useless-escape': 0,
    'no-template-curly-in-string': 1
  }
}
