<script>
export default {
  name: 'render',
  functional: true,
  props: {
    scope: Object,
    render: Function
  },
  render: (h, ctx) => {
    return ctx.props.render ? ctx.props.render(ctx.props.scope) : ''
  }
}
</script>
