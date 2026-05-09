<script>
  import { Router, Route } from 'svelte-routing'
  import Login from './routes/Login.svelte'
  import Files from './routes/Files.svelte'
  import Share from './routes/Share.svelte'
  import Trash from './routes/Trash.svelte'
  import Admin from './routes/Admin.svelte'
</script>

<Router>
  <Route path="/login" component={Login} />
  <Route path="/trash" component={Trash} />
  <Route path="/admin" component={Admin} />
  <Route path="/s/:token" let:params>
    <Share token={params.token} />
  </Route>
  <Route path="/" component={Files} />
  <Route path="/*" component={Files} />
</Router>
