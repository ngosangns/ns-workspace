#!/usr/bin/env node
// Lint HTML files under docs/, skipping gracefully when none exist.
// html-validate exits with code 1 when a glob matches no files, which breaks
// lint:doc/preview in repositories that currently have no HTML docs.
const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");

function findHtmlFiles(dir, out = []) {
	if (!fs.existsSync(dir)) {
		return out;
	}
	for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
		const full = path.join(dir, entry.name);
		if (entry.isDirectory()) {
			findHtmlFiles(full, out);
		} else if (entry.isFile() && entry.name.endsWith(".html")) {
			out.push(full);
		}
	}
	return out;
}

const files = findHtmlFiles("docs");
if (files.length === 0) {
	process.exit(0);
}
execFileSync("npx", ["html-validate", ...files], { stdio: "inherit" });
