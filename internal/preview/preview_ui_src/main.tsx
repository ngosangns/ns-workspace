/* @refresh reload */
import { render } from "solid-js/web";
import { HashRouter, Route } from "@solidjs/router";
import "./style.css";
import App from "./App";
import Docs from "./views/Docs";
import Search from "./views/Search";
import GraphView from "./views/Graph";

const root = document.getElementById("app");
if (!root) throw new Error("#app not found");

render(
  () => (
    <HashRouter root={App}>
      <Route path="/" component={Docs} />
      <Route path="/docs/:id" component={Docs} />
      <Route path="/search" component={Search} />
      <Route path="/graph" component={GraphView} />
    </HashRouter>
  ),
  root,
);
