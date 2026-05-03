<script>
  import { Router, Route } from 'svelte-routing'
  import Login from './routes/Login.svelte'
  import Files from './routes/Files.svelte'
</script>

<Router>
  <Route path="/login" component={Login} />
  <Route path="/" component={Files} />
  <Route path="/*" component={Files} />
</Router>
