<script>
  import { Router, Route } from 'svelte-routing'
  import Login from './routes/Login.svelte'
  import Files from './routes/Files.svelte'
  import Share from './routes/Share.svelte'
</script>

<Router>
  <Route path="/login" component={Login} />
  <Route path="/s/:token" let:params>
    <Share token={params.token} />
  </Route>
  <Route path="/" component={Files} />
  <Route path="/*" component={Files} />
</Router>
