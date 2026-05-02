<script>
  import { Router, Route } from 'svelte-routing'
</script>

<Router>
  <Route path="/" component={() => import('./routes/Files.svelte')} />
</Router>
