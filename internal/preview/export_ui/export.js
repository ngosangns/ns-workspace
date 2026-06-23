/*
 * ns-workspace static export UI (vanilla JS, no build step).
 *
 * Hydrates the sidebar, document view, and graph entirely from the
 * `window.__NS_KB__` bundle injected by exportStaticBundle. Routing is driven
 * by `location.hash` so the file works over file:// with no server.
 *
 * Hash routes:
 *   #doc/<id>   show a document
 *   #graph      show the dependency graph
 */
(function () {
  "use strict";

  var KB = window.__NS_KB__ || {
    project: { name: "Knowledge Base", warnings: [] },
    documents: [],
    graph: { nodes: [], edges: [] },
  };

  var documents = Array.isArray(KB.documents) ? KB.documents : [];
  var docById = {};
  documents.forEach(function (d) {
    docById[d.id] = d;
  });

  var els = {};
  var cy = null; // lazily initialised cytoscape instance
  var graphRendered = false;

  /* ---------------------------------------------------------------- utils */

  function el(id) {
    return document.getElementById(id);
  }

  function escapeHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /* ------------------------------------------------------------- sidebar */

  function renderSidebar(filter) {
    var list = els.docList;
    list.innerHTML = "";
    var needle = (filter || "").trim().toLowerCase();

    documents
      .filter(function (d) {
        if (!needle) return true;
        var hay = (d.title + " " + (d.category || "") + " " + d.id).toLowerCase();
        return hay.indexOf(needle) !== -1;
      })
      .forEach(function (d) {
        var li = document.createElement("li");
        var a = document.createElement("a");
        a.href = "#doc/" + encodeURIComponent(d.id);
        a.dataset.docId = d.id;
        var cat = d.category
          ? '<span class="kb-doc-cat">' + escapeHtml(d.category) + "</span>"
          : "";
        a.innerHTML = cat + escapeHtml(d.title || d.id);
        li.appendChild(a);
        list.appendChild(li);
      });
  }

  function highlightActiveDoc(id) {
    var links = els.docList.querySelectorAll("a");
    Array.prototype.forEach.call(links, function (a) {
      a.classList.toggle("active", a.dataset.docId === id);
    });
  }

  /* --------------------------------------------------------------- views */

  function showDoc(id) {
    var doc = docById[id] || documents[0];
    setActiveTab("doc");
    els.graph.classList.remove("active");
    els.doc.classList.remove("kb-hidden");

    if (!doc) {
      els.doc.innerHTML = "<p>No documents in this export.</p>";
      return;
    }

    var meta = doc.meta || {};
    var badges = [];
    if (doc.category) badges.push(doc.category);
    Object.keys(meta).forEach(function (k) {
      if (meta[k]) badges.push(k + ": " + meta[k]);
    });
    var badgeHtml = badges
      .map(function (b) {
        return '<span class="kb-badge">' + escapeHtml(b) + "</span>";
      })
      .join("");

    var body = doc.renderedHtml;
    if (body == null || body === "") {
      // Fallback: render raw markdown client-side if available (e.g. CDN mode
      // where the server chose not to pre-render). Permissive: never throw.
      if (typeof window.marked !== "undefined" && doc.raw) {
        try {
          body = window.marked.parse(doc.raw);
        } catch (e) {
          body = "<p>(render failed)</p>";
        }
      } else {
        body = "<p>(no content)</p>";
      }
    }

    els.doc.innerHTML =
      "<h1>" +
      escapeHtml(doc.title || doc.id) +
      "</h1>" +
      (badgeHtml ? '<div class="kb-meta">' + badgeHtml + "</div>" : "") +
      '<div class="kb-doc-body">' +
      body +
      "</div>";
    els.doc.scrollTop = 0;
    highlightActiveDoc(doc.id);
  }

  function showGraph() {
    setActiveTab("graph");
    els.doc.classList.add("kb-hidden");
    els.graph.classList.add("active");
    renderGraph();
  }

  function renderGraph() {
    if (graphRendered) return;
    graphRendered = true;

    var graph = KB.graph || { nodes: [], edges: [] };
    var nodes = Array.isArray(graph.nodes) ? graph.nodes : [];
    var edges = Array.isArray(graph.edges) ? graph.edges : [];

    if (typeof window.cytoscape === "undefined") {
      els.graph.innerHTML =
        '<div class="kb-graph-empty">Graph library unavailable.</div>';
      return;
    }
    if (nodes.length === 0) {
      els.graph.innerHTML =
        '<div class="kb-graph-empty">No graph data in this export.</div>';
      return;
    }

    var nodeIds = {};
    nodes.forEach(function (n) {
      nodeIds[n.id] = true;
    });

    var elements = [];
    nodes.forEach(function (n) {
      elements.push({
        data: { id: n.id, label: n.label || n.id, type: n.type || "" },
      });
    });
    edges.forEach(function (e, i) {
      // Permissive: skip edges that reference unknown nodes.
      if (!nodeIds[e.from] || !nodeIds[e.to]) return;
      elements.push({
        data: {
          id: "e" + i,
          source: e.from,
          target: e.to,
          label: e.label || "",
        },
      });
    });

    cy = window.cytoscape({
      container: els.cy,
      elements: elements,
      style: [
        {
          selector: "node",
          style: {
            "background-color": "#5b9dff",
            label: "data(label)",
            color: "#e6e9ef",
            "font-size": "10px",
            "text-valign": "bottom",
            "text-margin-y": 4,
            width: 18,
            height: 18,
          },
        },
        {
          selector: "edge",
          style: {
            width: 1.5,
            "line-color": "#2a2f3a",
            "target-arrow-color": "#2a2f3a",
            "target-arrow-shape": "triangle",
            "curve-style": "bezier",
          },
        },
      ],
      layout: { name: "cose", animate: false, padding: 30 },
    });
  }

  function setActiveTab(tab) {
    Array.prototype.forEach.call(els.nav.querySelectorAll("button"), function (b) {
      b.classList.toggle("active", b.dataset.view === tab);
    });
  }

  /* -------------------------------------------------------------- router */

  function route() {
    var hash = location.hash.replace(/^#/, "");
    if (hash === "graph") {
      showGraph();
      return;
    }
    if (hash.indexOf("doc/") === 0) {
      showDoc(decodeURIComponent(hash.slice(4)));
      return;
    }
    // default: first document
    if (documents.length > 0) {
      location.hash = "doc/" + encodeURIComponent(documents[0].id);
    } else {
      showDoc(null);
    }
  }

  /* ----------------------------------------------------------- bootstrap */

  function init() {
    els.docList = el("kb-doc-list");
    els.doc = el("kb-doc");
    els.graph = el("kb-graph");
    els.cy = el("kb-cy");
    els.nav = el("kb-nav");
    els.search = el("kb-search-input");
    els.brand = el("kb-brand-title");
    els.sub = el("kb-brand-sub");

    var project = KB.project || {};
    if (els.brand) els.brand.textContent = project.name || "Knowledge Base";
    if (els.sub) {
      var total = project.total != null ? project.total : documents.length;
      els.sub.textContent = total + " documents";
    }

    // Warnings banner (fail-open export surface).
    var warnings = (project && project.warnings) || [];
    if (warnings.length && els.doc) {
      // stored for display when the first doc renders
    }

    renderSidebar("");

    if (els.search) {
      els.search.addEventListener("input", function () {
        renderSidebar(els.search.value);
      });
    }

    Array.prototype.forEach.call(els.nav.querySelectorAll("button"), function (b) {
      b.addEventListener("click", function () {
        if (b.dataset.view === "graph") {
          location.hash = "graph";
        } else if (documents.length) {
          location.hash = "doc/" + encodeURIComponent(documents[0].id);
        }
      });
    });

    window.addEventListener("hashchange", route);
    route();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
