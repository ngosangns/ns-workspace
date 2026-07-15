/* @refresh reload */
import { render } from "solid-js/web";
import { Route, HashRouter } from "@solidjs/router";
import "./style.css";
import App from "./App";
import Dashboard from "./views/Dashboard";
import Skills from "./views/Skills";
import MCPs from "./views/MCPs";
import Registry from "./views/Registry";
import Adapters from "./views/Adapters";

const root = document.getElementById("app");
if (!root) throw new Error("#app not found");

render(
  () => (
    <HashRouter root={App}>
      <Route path="/" component={Dashboard} />
      <Route path="/skills" component={Skills} />
      <Route path="/mcps" component={MCPs} />
      <Route path="/registry" component={Registry} />
      <Route path="/adapters" component={Adapters} />
    </HashRouter>
  ),
  root,
);
