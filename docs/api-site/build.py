#!/usr/bin/env python3
"""Build the public API documentation site.

One source, two readers: every page in ``pages/`` is rendered twice, as an
HTML page for integrators and as Markdown for agents, plus ``llms.txt`` and
``llms-full.txt`` indexes. The site is served by Nginx under ``/docs/``.

Usage: python3 -I docs/api-site/build.py [--out DIR]
Only the Python standard library is used.
"""

from __future__ import annotations

import argparse
import html
import json
import re
import shlex
import shutil
import sys
from html.parser import HTMLParser
from pathlib import Path

ROOT = Path(__file__).resolve().parent
VOID = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"}


# --------------------------------------------------------------------------
# Minimal DOM
# --------------------------------------------------------------------------


class Node:
    __slots__ = ("tag", "attrs", "children", "parent", "text")

    def __init__(self, tag=None, attrs=None, text=""):
        self.tag = tag
        self.attrs = list(attrs or [])
        self.children: list[Node] = []
        self.parent: Node | None = None
        self.text = text

    def get(self, name, default=None):
        for key, value in self.attrs:
            if key == name:
                return value if value is not None else ""
        return default

    def set(self, name, value):
        for i, (key, _) in enumerate(self.attrs):
            if key == name:
                self.attrs[i] = (name, value)
                return
        self.attrs.append((name, value))

    def remove_attr(self, name):
        self.attrs = [(k, v) for k, v in self.attrs if k != name]

    def classes(self):
        return (self.get("class") or "").split()

    def append(self, child):
        child.parent = self
        self.children.append(child)
        return child

    def elements(self):
        return [c for c in self.children if c.tag]

    def iter(self):
        yield self
        for child in self.children:
            yield from child.iter()

    def find_all(self, tag=None, cls=None):
        for node in self.iter():
            if not node.tag or node is self:
                continue
            if tag and node.tag != tag:
                continue
            if cls and cls not in node.classes():
                continue
            yield node

    def find(self, tag=None, cls=None):
        return next(self.find_all(tag, cls), None)

    def child(self, tag):
        return next((c for c in self.children if c.tag == tag), None)


class TreeBuilder(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=False)
        self.root = Node("#root")
        self.stack = [self.root]

    def handle_starttag(self, tag, attrs):
        node = self.stack[-1].append(Node(tag, attrs))
        if tag not in VOID:
            self.stack.append(node)

    def handle_startendtag(self, tag, attrs):
        node = self.stack[-1].append(Node(tag, attrs))
        if tag not in VOID:
            pass

    def handle_endtag(self, tag):
        for i in range(len(self.stack) - 1, 0, -1):
            if self.stack[i].tag == tag:
                del self.stack[i:]
                return

    def handle_data(self, data):
        self.stack[-1].append(Node(text=data))

    def handle_entityref(self, name):
        self.stack[-1].append(Node(text=f"&{name};"))

    def handle_charref(self, name):
        self.stack[-1].append(Node(text=f"&#{name};"))


def parse(source: str) -> Node:
    builder = TreeBuilder()
    builder.feed(source)
    builder.close()
    return builder.root


def to_html(node: Node) -> str:
    if node.tag is None:
        return node.text
    inner = "".join(to_html(c) for c in node.children)
    if node.tag == "#root":
        return inner
    attrs = "".join(f" {k}" if v is None else f' {k}="{html.escape(v, quote=True)}"' for k, v in node.attrs)
    if node.tag in VOID:
        return f"<{node.tag}{attrs}>"
    return f"<{node.tag}{attrs}>{inner}</{node.tag}>"


def inner_html(node: Node | None) -> str:
    return "".join(to_html(c) for c in node.children) if node else ""


def text_of(node: Node) -> str:
    if node.tag is None:
        return html.unescape(node.text)
    return "".join(text_of(c) for c in node.children)


def esc(value: str) -> str:
    return html.escape(value, quote=True)


# --------------------------------------------------------------------------
# Source model
# --------------------------------------------------------------------------


class Endpoint:
    def __init__(self, node: Node):
        self.id = node.get("id") or ""
        self.title = node.get("title") or ""
        self.returns = node.get("returns") or ""
        self.routes = [(r.get("method", "GET").upper(), r.get("path", "")) for r in node.find_all("doc-route")]
        self.main = node.child("doc-main") or Node("doc-main")
        self.examples = node.child("doc-examples")


class Page:
    def __init__(self, path: Path):
        root = parse(path.read_text(encoding="utf-8"))
        page = root.find("doc-page")
        if page is None:
            raise SystemExit(f"{path}: missing <doc-page>")
        self.path = path
        self.slug = page.get("slug") or path.stem
        self.kind = page.get("kind") or "reference"
        self.title = page.get("title") or self.slug
        self.nav = page.get("nav") or self.title
        self.summary = page.child("doc-summary")
        self.body = page.child("doc-body")
        self.simple = page.child("doc-simple")
        self.agent = page.child("doc-agent")
        flow = page.child("doc-flow")
        self.flow = [(s.get("title") or "", s) for s in flow.find_all("step")] if flow else []
        self.endpoints = [Endpoint(n) for n in page.find_all("doc-endpoint")]
        self.group = ""

    @property
    def href(self):
        return "./" if self.slug == "index" else self.slug

    @property
    def md_name(self):
        return f"{self.slug}.md"

    def summary_text(self):
        return " ".join(text_of(self.summary).split()) if self.summary else ""

    def ids(self):
        found = set()
        for node in self.iter_content():
            for el in node.iter():
                if el.tag and el.get("id"):
                    found.add(el.get("id"))
        found.update(e.id for e in self.endpoints if e.id)
        found.add(self.slug)
        return found

    def iter_content(self):
        for node in (self.summary, self.body, self.simple, self.agent):
            if node is not None:
                yield node
        for _, step in self.flow:
            yield step
        for endpoint in self.endpoints:
            yield endpoint.main
            if endpoint.examples is not None:
                yield endpoint.examples


class Site:
    def __init__(self):
        self.config = json.loads((ROOT / "site.json").read_text(encoding="utf-8"))
        self.pages: dict[str, Page] = {}
        for path in sorted((ROOT / "pages").glob("*.html")):
            page = Page(path)
            if page.slug in self.pages:
                raise SystemExit(f"duplicate page slug {page.slug}")
            self.pages[page.slug] = page
        self.groups = []
        listed = set()
        for group in self.config["groups"]:
            pages = []
            for slug in group["pages"]:
                if slug not in self.pages:
                    raise SystemExit(f"site.json lists unknown page {slug}")
                self.pages[slug].group = group["label"]
                pages.append(self.pages[slug])
                listed.add(slug)
            self.groups.append((group["label"], pages))
        missing = set(self.pages) - listed - {"index"}
        if missing:
            raise SystemExit(f"pages not listed in site.json: {sorted(missing)}")
        if "index" not in self.pages:
            raise SystemExit("pages/index.html is required")
        self.id_owner: dict[str, str] = {}
        for page in self.pages.values():
            for node_id in page.ids():
                owner = self.id_owner.get(node_id)
                if owner and owner != page.slug:
                    raise SystemExit(f"id {node_id!r} used on both {owner} and {page.slug}")
                self.id_owner[node_id] = page.slug

    def ordered(self):
        yield self.pages["index"]
        for _, pages in self.groups:
            yield from pages


# --------------------------------------------------------------------------
# curl -> Python / Node.js
# --------------------------------------------------------------------------

VAR_RE = re.compile(r"\$([A-Z_][A-Z0-9_]*)")


class Curl:
    def __init__(self, method, url, headers, data, form, output):
        self.method = method
        self.url = url
        self.headers = headers
        self.data = data
        self.form = form
        self.output = output

    def variables(self):
        names = []
        values = [self.url, *[v for _, v in self.headers], *(self.form or [])]
        for value in values:
            for name in VAR_RE.findall(value or ""):
                if name not in names:
                    names.append(name)
        return names


def parse_curl(source: str) -> Curl | None:
    try:
        tokens = shlex.split(source.replace("\\\n", " "))
    except ValueError:
        return None
    if not tokens or tokens[0] != "curl":
        return None
    method = url = data = output = None
    headers: list[tuple[str, str]] = []
    form: list[str] = []
    i = 1
    while i < len(tokens):
        token = tokens[i]
        value = tokens[i + 1] if i + 1 < len(tokens) else None
        if token in ("-X", "--request") and value:
            method = value.upper()
        elif token in ("-H", "--header") and value:
            name, _, val = value.partition(":")
            headers.append((name.strip(), val.strip()))
        elif token in ("-d", "--data", "--data-raw", "--data-binary") and value is not None:
            data = value
        elif token in ("-F", "--form") and value:
            form.append(value)
        elif token in ("-o", "--output") and value:
            output = value
        elif token.startswith("-"):
            return None
        else:
            if url is not None:
                return None
            url = token
            i += 1
            continue
        i += 2
    if url is None:
        return None
    method = method or ("POST" if data is not None or form else "GET")
    return Curl(method, url, headers, data, form, output)


def py_str(value: str) -> str:
    if not VAR_RE.search(value):
        return json.dumps(value, ensure_ascii=False)
    out, last = [], 0
    for m in VAR_RE.finditer(value):
        out.append(json.dumps(value[last:m.start()], ensure_ascii=False)[1:-1].replace("{", "{{").replace("}", "}}"))
        out.append("{" + m.group(1) + "}")
        last = m.end()
    out.append(json.dumps(value[last:], ensure_ascii=False)[1:-1].replace("{", "{{").replace("}", "}}"))
    return 'f"' + "".join(out) + '"'


def js_str(value: str) -> str:
    if not VAR_RE.search(value):
        return json.dumps(value, ensure_ascii=False)
    out, last = [], 0
    for m in VAR_RE.finditer(value):
        out.append(value[last:m.start()].replace("\\", "\\\\").replace("`", "\\`").replace("${", "\\${"))
        out.append("${" + m.group(1) + "}")
        last = m.end()
    out.append(value[last:].replace("\\", "\\\\").replace("`", "\\`").replace("${", "\\${"))
    return "`" + "".join(out) + "`"


def py_literal(value, depth=0) -> str:
    if isinstance(value, bool):
        return "True" if value else "False"
    if value is None:
        return "None"
    if isinstance(value, (int, float)):
        return json.dumps(value)
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False)
    compact = json.dumps(value, ensure_ascii=False, separators=(", ", ": "))
    flat = not any(isinstance(v, (dict, list)) for v in (value.values() if isinstance(value, dict) else value))
    if flat and len(compact) <= 64:
        if isinstance(value, dict):
            return "{" + ", ".join(f"{json.dumps(k, ensure_ascii=False)}: {py_literal(v)}" for k, v in value.items()) + "}"
        return "[" + ", ".join(py_literal(v) for v in value) + "]"
    pad, end = "    " * (depth + 1), "    " * depth
    if isinstance(value, dict):
        items = [f"{pad}{json.dumps(k, ensure_ascii=False)}: {py_literal(v, depth + 1)}," for k, v in value.items()]
        return "{\n" + "\n".join(items) + "\n" + end + "}"
    items = [f"{pad}{py_literal(v, depth + 1)}," for v in value]
    return "[\n" + "\n".join(items) + "\n" + end + "]"


def indent_tail(text: str, prefix: str) -> str:
    lines = text.split("\n")
    return "\n".join([lines[0], *[prefix + line for line in lines[1:]]])


def form_parts(curl: Curl):
    for item in curl.form:
        name, _, value = item.partition("=")
        if value.startswith("@"):
            yield name, value[1:], True
        else:
            yield name, value, False


def to_python(curl: Curl) -> str:
    variables = curl.variables()
    body_json = None
    if curl.data is not None:
        try:
            body_json = json.loads(curl.data)
        except ValueError:
            body_json = None
    lines = ["import os", "", "import requests", ""]
    for name in variables:
        lines.append(f'{name} = os.environ["{name}"]')
    if variables:
        lines.append("")
    headers = [(k, v) for k, v in curl.headers
               if not (k.lower() == "content-type" and (body_json is not None or curl.form))]
    args = [f"    {py_str(curl.url)},"]
    if headers:
        args.append("    headers={")
        args.extend(f"        {py_str(k)}: {py_str(v)}," for k, v in headers)
        args.append("    },")
    if body_json is not None:
        args.append(f"    json={indent_tail(py_literal(body_json, 1), '')},")
    elif curl.data is not None:
        args.append(f"    data={py_str(curl.data)},")
    if curl.form:
        fields = [(n, v) for n, v, is_file in form_parts(curl) if not is_file]
        files = [(n, v) for n, v, is_file in form_parts(curl) if is_file]
        if fields:
            args.append("    data={" + ", ".join(f"{py_str(n)}: {py_str(v)}" for n, v in fields) + "},")
        if files:
            args.append("    files={" + ", ".join(f'{py_str(n)}: open({py_str(v)}, "rb")' for n, v in files) + "},")
    method = curl.method.lower()
    call = f"requests.{method}(" if method in ("get", "post", "put", "delete", "patch") else f'requests.request("{curl.method}", '
    lines.append(f"resp = {call}")
    lines.extend(args)
    lines.append(")")
    lines.append("resp.raise_for_status()")
    if curl.output:
        lines.append(f"with open({py_str(curl.output)}, \"wb\") as f:")
        lines.append("    f.write(resp.content)")
    else:
        lines.append("print(resp.json())")
    return "\n".join(lines)


def to_node(curl: Curl) -> str:
    variables = curl.variables()
    body_json = None
    if curl.data is not None:
        try:
            body_json = json.loads(curl.data)
        except ValueError:
            body_json = None
    imports = []
    if curl.form and any(is_file for _, _, is_file in form_parts(curl)):
        imports.append("readFile")
    if curl.output:
        imports.append("writeFile")
    lines = []
    if imports:
        lines += [f'import {{ {", ".join(imports)} }} from "node:fs/promises";', ""]
    for name in variables:
        lines.append(f"const {name} = process.env.{name};")
    if variables:
        lines.append("")
    if curl.form:
        lines.append("const form = new FormData();")
        for name, value, is_file in form_parts(curl):
            if is_file:
                filename = value.rsplit("/", 1)[-1]
                lines.append(f"form.append({js_str(name)}, new Blob([await readFile({js_str(value)})]), {js_str(filename)});")
            else:
                lines.append(f"form.append({js_str(name)}, {js_str(value)});")
        lines.append("")
    headers = [(k, v) for k, v in curl.headers if not (k.lower() == "content-type" and curl.form)]
    opts = []
    if curl.method != "GET":
        opts.append(f'  method: "{curl.method}",')
    if headers:
        opts.append("  headers: {")
        opts.extend(f"    {json.dumps(k)}: {js_str(v)}," for k, v in headers)
        opts.append("  },")
    if body_json is not None:
        body = json.dumps(body_json, ensure_ascii=False, indent=2)
        opts.append(f"  body: JSON.stringify({indent_tail(body, '  ')}),")
    elif curl.data is not None:
        opts.append(f"  body: {js_str(curl.data)},")
    if curl.form:
        opts.append("  body: form,")
    if opts:
        lines.append(f"const resp = await fetch({js_str(curl.url)}, {{")
        lines.extend(opts)
        lines.append("});")
    else:
        lines.append(f"const resp = await fetch({js_str(curl.url)});")
    if curl.output:
        lines.append(f"await writeFile({js_str(curl.output)}, Buffer.from(await resp.arrayBuffer()));")
    else:
        lines.append("console.log(await resp.json());")
    return "\n".join(lines)


# --------------------------------------------------------------------------
# HTML content transforms
# --------------------------------------------------------------------------

COPY_ICON = ('<svg aria-hidden="true" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" '
             'stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="11" height="11" '
             'rx="2"></rect><path d="M5 15V5a2 2 0 0 1 2-2h10"></path></svg>')
LANGS = [("curl", "cURL"), ("python", "Python"), ("node", "Node.js")]


class PageRenderer:
    """Turns one page's content nodes into final HTML, sharing an id counter."""

    def __init__(self, site: Site, page: Page):
        self.site = site
        self.page = page
        self.counter = 0
        self.sim_steps: list[dict] = []

    def next_id(self, prefix):
        self.counter += 1
        return f"{prefix}-{self.counter}"

    def link(self, href: str) -> str:
        if not href.startswith("#"):
            return href
        target = href[1:]
        owner = self.site.id_owner.get(target)
        if owner and owner != self.page.slug:
            page = self.site.pages[owner]
            return f"{page.href}#{target}" if target != owner else page.href
        return href

    def code_block(self, pre: Node, record_sim=True) -> Node:
        source = text_of(pre).strip("\n")
        curl = parse_curl(source) if source.lstrip().startswith("curl") else None
        sim = pre.get("data-sim")
        if sim and record_sim:
            self.sim_steps.append({"kind": sim, "label": sim_label(sim, source), "body": source})
        block_id = self.next_id("code")
        if curl is None:
            label = "JSON" if source.lstrip()[:1] in "{[" else ("Shell" if source.lstrip().startswith("curl") else "代码")
            markup = (f'<div class="code" data-code><div class="code-bar"><span class="code-label">{label}</span>'
                      f'<button type="button" class="code-copy" aria-label="复制代码" title="复制">{COPY_ICON}</button></div>'
                      f'<pre><code>{esc(source)}</code></pre></div>')
            return parse(markup).elements()[0]
        variants = {"curl": source, "python": to_python(curl), "node": to_node(curl)}
        tabs, panes = [], []
        for i, (lang, label) in enumerate(LANGS):
            tab_id, pane_id = f"{block_id}-{lang}-tab", f"{block_id}-{lang}"
            selected = "true" if i == 0 else "false"
            tabs.append(f'<button type="button" role="tab" id="{tab_id}" data-lang="{lang}" aria-controls="{pane_id}" '
                        f'aria-selected="{selected}" tabindex="{0 if i == 0 else -1}">{label}</button>')
            hidden = "" if i == 0 else " hidden"
            panes.append(f'<pre id="{pane_id}" role="tabpanel" aria-labelledby="{tab_id}" data-lang="{lang}"{hidden}>'
                         f'<code>{esc(variants[lang])}</code></pre>')
        markup = (f'<div class="code" data-code data-langs><div class="code-bar">'
                  f'<div class="code-tabs" role="tablist" aria-label="代码语言">{"".join(tabs)}</div>'
                  f'<button type="button" class="code-copy" aria-label="复制代码" title="复制">{COPY_ICON}</button></div>'
                  f'{"".join(panes)}</div>')
        return parse(markup).elements()[0]

    def transform(self, node: Node | None) -> str:
        if node is None:
            return ""
        node = parse(inner_html(node))  # work on a copy
        for el in list(node.iter()):
            if el.tag == "a" and el.get("href"):
                el.set("href", self.link(el.get("href")))
        for pre in [p for p in node.iter() if p.tag == "pre"]:
            replacement = self.code_block(pre)
            parent = pre.parent
            index = parent.children.index(pre)
            parent.children[index] = replacement
            replacement.parent = parent
        for table in [t for t in node.iter() if t.tag == "table"]:
            wrapper = Node("div", [("class", "table-wrap")])
            parent = table.parent
            index = parent.children.index(table)
            parent.children[index] = wrapper
            wrapper.parent = parent
            wrapper.append(table)
        return to_html(node)


def sim_label(kind: str, body: str) -> str:
    try:
        data = json.loads(body)
    except ValueError:
        data = None
    if isinstance(data, dict):
        for holder in (data, data.get("data") if isinstance(data.get("data"), dict) else None):
            if not holder:
                continue
            for key in ("status", "task_status", "state"):
                if isinstance(holder.get(key), str):
                    return holder[key]
    return "已提交" if kind == "submit" else "已完成"


# --------------------------------------------------------------------------
# Markdown
# --------------------------------------------------------------------------


class MarkdownWriter:
    def __init__(self, site: Site, page: Page):
        self.site = site
        self.page = page

    def link(self, href: str) -> str:
        if href.startswith("#"):
            owner = self.site.id_owner.get(href[1:])
            if owner and owner != self.page.slug:
                return f"/docs/{self.site.pages[owner].md_name}"
            return href
        return href

    def inline(self, node: Node) -> str:
        if node.tag is None:
            return re.sub(r"\s+", " ", html.unescape(node.text))
        tag = node.tag
        if tag == "br":
            return "<br>"
        inner = "".join(self.inline(c) for c in node.children)
        if tag == "code":
            value = text_of(node)
            fence = "``" if "`" in value else "`"
            return f"{fence}{value}{fence}"
        if tag in ("b", "strong"):
            return f"**{inner.strip()}**" if inner.strip() else ""
        if tag in ("i", "em"):
            return f"*{inner.strip()}*" if inner.strip() else ""
        if tag == "a":
            return f"[{inner.strip()}]({self.link(node.get('href') or '')})"
        if tag == "button":
            return ""
        return inner

    def table(self, node: Node) -> str:
        rows = []
        for tr in node.find_all("tr"):
            cells = [c for c in tr.children if c.tag in ("td", "th")]
            rows.append([self.inline(c).strip().replace("|", "\\|") for c in cells])
        if not rows:
            return ""
        width = max(len(r) for r in rows)
        rows = [r + [""] * (width - len(r)) for r in rows]
        out = ["| " + " | ".join(rows[0]) + " |", "|" + "|".join(["---"] * width) + "|"]
        out.extend("| " + " | ".join(r) + " |" for r in rows[1:])
        return "\n".join(out)

    def blocks(self, node: Node, depth=0) -> list[str]:
        out: list[str] = []
        inline_buf: list[str] = []

        def flush():
            text = "".join(inline_buf).strip()
            inline_buf.clear()
            if text:
                out.append(text)

        for child in node.children:
            tag = child.tag
            if tag is None:
                inline_buf.append(self.inline(child))
                continue
            if tag in ("code", "b", "strong", "a", "span", "i", "em", "br") and "pill" not in child.classes():
                inline_buf.append(self.inline(child))
                continue
            flush()
            classes = child.classes()
            if tag == "doc-endpoint-index":
                out.append(endpoint_index_markdown(self.site))
            elif tag in ("h1", "h2", "h3", "h4", "h5"):
                level = {"h1": 1, "h2": 2, "h3": 3, "h4": 4, "h5": 5}[tag] + depth
                out.append("#" * min(level, 6) + " " + self.inline(child).strip())
            elif tag == "p":
                text = self.inline(child).strip()
                if text:
                    out.append(text)
            elif tag in ("ul", "ol"):
                out.append(self.list_md(child, 0))
            elif tag == "table":
                out.append(self.table(child))
            elif tag == "pre":
                source = text_of(child).strip("\n")
                lang = "json" if source.lstrip()[:1] in "{[" else ("bash" if source.lstrip().startswith("curl") else "")
                out.append(f"```{lang}\n{source}\n```")
            elif tag == "dl":
                items = []
                for dt in child.find_all("dt"):
                    dd = dt.parent.children[dt.parent.children.index(dt) + 1:]
                    dd = next((d for d in dd if d.tag == "dd"), None)
                    items.append(f"- **{self.inline(dt).strip()}**：{self.inline(dd).strip() if dd else ''}")
                out.append("\n".join(items))
            elif "callout" in classes:
                inner = "\n\n".join(self.blocks(child, depth)) or self.inline(child).strip()
                out.append("\n".join("> " + line if line else ">" for line in inner.split("\n")))
            elif "pill" in classes:
                inline_buf.append(f" ({text_of(child).strip()})")
            elif child.get("data-example-switcher") is not None:
                labels = {t.get("id"): text_of(t).strip() for t in child.find_all("button") if t.get("role") == "tab"}
                for panel in child.find_all():
                    if panel.get("role") != "tabpanel":
                        continue
                    label = labels.get(panel.get("aria-labelledby") or "", "")
                    if label:
                        out.append(f"#### {label}")
                    out.extend(self.blocks(panel, depth))
            else:
                out.extend(self.blocks(child, depth))
        flush()
        return [b for b in out if b.strip()]

    def list_md(self, node: Node, level: int) -> str:
        lines = []
        ordered = node.tag == "ol"
        for i, li in enumerate([c for c in node.children if c.tag == "li"], 1):
            nested = [c for c in li.children if c.tag in ("ul", "ol")]
            text_nodes = Node("li")
            text_nodes.children = [c for c in li.children if c.tag not in ("ul", "ol")]
            text = "".join(self.inline(c) for c in text_nodes.children).strip()
            marker = f"{i}." if ordered else "-"
            lines.append("  " * level + f"{marker} {text}")
            for sub in nested:
                lines.append(self.list_md(sub, level + 1))
        return "\n".join(lines)

    def md(self, node: Node | None, depth=0) -> str:
        if node is None:
            return ""
        return "\n\n".join(self.blocks(node, depth))


def page_markdown(site: Site, page: Page) -> str:
    writer = MarkdownWriter(site, page)
    cfg = site.config
    lines = ["---", f'title: "{page.title}"', f'url: "/docs/{"" if page.slug == "index" else page.slug}"']
    lines.append("base_url:")
    lines.extend(f'  - "{b["url"]}"  # {b["name"]}' for b in cfg["base_urls"])
    lines.append(f'auth: "{cfg["auth"]}"')
    routes = [f"{m} {p}" for e in page.endpoints for m, p in e.routes]
    if routes:
        lines.append("endpoints:")
        lines.extend(f'  - "{r}"' for r in dict.fromkeys(routes))
    lines += ["---", "", f"# {page.title}", ""]
    if page.summary is not None:
        lines += [writer.md(page.summary), ""]
    if page.agent is not None:
        lines += ["## Agent 调用规则", "", writer.md(page.agent), ""]
    if page.simple is not None:
        lines += ["## 简明说明", "", writer.md(page.simple), ""]
    if page.body is not None:
        lines += [writer.md(page.body), ""]
    for endpoint in page.endpoints:
        lines += [f"## {endpoint.title}", ""]
        if endpoint.routes:
            lines += ["\n".join(f"`{m} {p}`" for m, p in endpoint.routes), ""]
        main = writer.md(endpoint.main, 0)
        if main:
            lines += [main, ""]
        examples = writer.md(endpoint.examples, 0)
        if examples:
            lines += [examples, ""]
    text = "\n".join(lines)
    return re.sub(r"\n{3,}", "\n\n", text).strip() + "\n"


def endpoint_index_markdown(site: Site) -> str:
    rows = ["| 分类 | 模型 / 能力 | 接口 | 路径 | 说明 |", "|---|---|---|---|---|"]
    for group, pages in site.groups:
        for page in pages:
            for e in page.endpoints:
                if not e.routes:
                    continue
                paths = "<br>".join(f"`{m} {p}`" for m, p in e.routes)
                rows.append(f"| {group} | [{page.nav}](/docs/{page.md_name}) | {e.title} | {paths} | {e.returns} |")
    return "\n".join(rows)


def llms_index(site: Site) -> str:
    cfg = site.config
    lines = [f"# {cfg['title']}", "", f"> {cfg['tagline']}", ""]
    lines += [f"鉴权：`{cfg['auth']}`。Base URL：" + "、".join(f"`{b['url']}`（{b['name']}）" for b in cfg["base_urls"]) + "。", ""]
    lines += ["每个链接是一页 Markdown，开头的 YAML 写明接口路径，`## Agent 调用规则` 写明提交、轮询和失败处理方式。", ""]
    index = site.pages["index"]
    lines += ["## 对接指南", "", f"- [{index.title}](/docs/{index.md_name}): {index.summary_text()}"]
    for group, pages in site.groups:
        if group != "对接指南":
            lines += ["", f"## {group}", ""]
        for page in pages:
            lines.append(f"- [{page.title}](/docs/{page.md_name}): {page.summary_text()}")
    lines += ["", "## 机器可读", "", "- [全部文档](/docs/llms-full.txt): 以上所有页面合并为一份 Markdown"]
    return "\n".join(lines) + "\n"


# --------------------------------------------------------------------------
# HTML templates
# --------------------------------------------------------------------------

SEARCH_ICON = ('<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" '
               'stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"></circle>'
               '<path d="m20 20-3.5-3.5"></path></svg>')
CHECK_ICON = ('<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" '
              'stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12 5 5L20 7"></path></svg>')


def method_badge(method: str) -> str:
    return f'<span class="method m-{method.lower()}">{method}</span>'


def head(site: Site, page: Page, title: str) -> str:
    cfg = site.config
    desc = esc(page.summary_text() or cfg["tagline"])
    return f"""<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{esc(title)}</title>
<meta name="description" content="{desc}">
<link rel="alternate" type="text/markdown" href="{page.md_name}" title="Markdown">
<link rel="alternate" type="text/plain" href="llms.txt" title="llms.txt">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&amp;family=JetBrains+Mono:wght@400;500&amp;family=Noto+Sans+SC:wght@400;500;700&amp;display=swap" media="print" onload="this.media='all'">
<link rel="icon" type="image/png" sizes="64x64" href="assets/favicon.png">
<link rel="apple-touch-icon" href="assets/apple-touch-icon.png">
<link rel="stylesheet" href="assets/docs.css">
<script>try{{var m=localStorage.getItem('docs.mode');if(m)document.documentElement.dataset.mode=m}}catch(e){{}}</script>
</head>
<body data-page="{esc(page.slug)}" data-md="{esc(page.md_name)}">
<a class="skip" href="#main">跳到正文</a>
"""


def header(site: Site, page: Page) -> str:
    cfg = site.config
    guide_on = page.kind == "overview"
    def tab(label, href, on):
        cur = ' aria-current="page"' if on else ""
        return f'<a href="{href}"{cur}>{label}</a>'
    return f"""<header class="topbar">
<a class="brand" href="./"><span class="brand-mark" aria-hidden="true">{esc(cfg['brand'][:1])}</span><span>{esc(cfg['brand'])}</span><span class="brand-sub">对接文档</span></a>
<nav class="topnav" aria-label="主导航">{tab('对接指南', './', guide_on)}{tab('API 参考', './#endpoints' if guide_on else page.href, not guide_on)}</nav>
<div class="search" data-search>
<label class="search-box"><span class="sr-only">搜索文档</span>{SEARCH_ICON}<input type="search" placeholder="搜索接口或参数" autocomplete="off" spellcheck="false" data-search-input aria-controls="search-results" aria-expanded="false" role="combobox"><kbd data-kbd>⌘K</kbd></label>
<div class="search-results" id="search-results" role="listbox" hidden data-search-results></div>
</div>
<button type="button" class="nav-toggle" aria-expanded="false" aria-controls="sidenav" data-nav-toggle>目录</button>
</header>
"""


def sidenav(site: Site, page: Page) -> str:
    parts = ['<nav class="sidenav" id="sidenav" aria-label="文档目录">']
    index = site.pages["index"]
    groups = [("对接指南", [index])]
    for label, pages in site.groups:
        if label == "对接指南":
            groups[0] = (label, [index, *pages])
        else:
            groups.append((label, pages))
    for label, pages in groups:
        parts.append(f'<div class="nav-group"><div class="nav-label">{esc(label)}</div>')
        for p in pages:
            current = p.slug == page.slug
            cur = ' aria-current="page"' if current else ""
            parts.append(f'<a class="nav-item" href="{p.href}"{cur}><span>{esc(p.nav)}</span></a>')
            if current and p.endpoints:
                parts.append('<div class="nav-sub">')
                for e in p.endpoints:
                    if not e.id:
                        continue
                    badge = method_badge(e.routes[0][0]) if e.routes else ""
                    parts.append(f'<a href="#{esc(e.id)}" data-target="{esc(e.id)}"><span>{esc(e.title)}</span>{badge}</a>')
                parts.append("</div>")
        parts.append("</div>")
    parts.append('<div class="nav-group"><div class="nav-label">机器可读</div>'
                 f'<a class="nav-item mono" href="{page.md_name}">{esc(page.md_name)}</a>'
                 '<a class="nav-item mono" href="llms.txt">llms.txt</a>'
                 '<a class="nav-item mono" href="llms-full.txt">llms-full.txt</a></div>')
    parts.append("</nav>")
    return "\n".join(parts)


def share_buttons() -> str:
    return ('<div class="share" role="group" aria-label="分享本页">'
            '<button type="button" class="btn" data-share="link">复制链接</button>'
            '<button type="button" class="btn" data-share="md">复制为 Markdown</button>'
            '<button type="button" class="btn btn-primary" data-share="prompt">交给 AI 助手</button></div>')


def footer() -> str:
    return """<div class="toast" role="status" aria-live="polite" data-toast hidden><span class="toast-icon">""" + CHECK_ICON + """</span><span data-toast-text></span></div>
<script src="assets/docs.js" defer></script>
</body>
</html>
"""


def flow_html(renderer: PageRenderer, page: Page) -> str:
    if not page.flow:
        return ""
    cards = []
    for i, (title, step) in enumerate(page.flow, 1):
        body = renderer.transform(step)
        cards.append(f'<div class="flow-step"><span class="step-n">{i}</span><span class="flow-text"><b>{esc(title)}</b><br>{body}</span></div>')
    return f'<section class="flow" aria-label="调用流程">{"".join(cards)}</section>'


def simulator_html(steps: list[dict]) -> str:
    if len(steps) < 2:
        return ""
    return """<section class="sim" aria-label="模拟调用" data-sim>
<div class="sim-head"><h2>模拟一次调用</h2><button type="button" class="btn btn-dark" data-sim-run>提交任务</button></div>
<p class="sim-note">按本页示例响应演示流程，不会真正发起请求。</p>
<div class="sim-steps" aria-live="polite" data-sim-steps></div>
<pre class="sim-resp"><code data-sim-resp></code></pre>
</section>"""


def reference_page(site: Site, page: Page) -> str:
    renderer = PageRenderer(site, page)
    summary = renderer.transform(page.summary)
    flow = flow_html(renderer, page)
    endpoints_html = []
    first_example = ""
    for endpoint in page.endpoints:
        main = renderer.transform(endpoint.main)
        examples = renderer.transform(endpoint.examples) if endpoint.examples is not None else ""
        routes = "".join(f'<div class="route">{method_badge(m)}<code>{esc(p)}</code></div>' for m, p in endpoint.routes)
        anchor = (f'<a class="anchor" href="#{esc(endpoint.id)}" aria-label="本节链接" data-anchor>#</a>' if endpoint.id else "")
        idattr = f' id="{esc(endpoint.id)}"' if endpoint.id else ""
        aside = f'<aside class="ep-aside" aria-label="示例"><div class="ep-aside-inner">{examples}<!--SIM--></div></aside>' if examples else ""
        cls = "endpoint has-aside" if examples else "endpoint"
        endpoints_html.append(
            f'<article class="{cls}"{idattr}><div class="ep-main"><header class="ep-head"><h2>{esc(endpoint.title)}{anchor}</h2>'
            f'{f"<div class=routes>{routes}</div>" if routes else ""}</header>{main}</div>{aside}</article>')
        if not first_example and endpoint.examples is not None:
            first_pre = next((p for p in endpoint.examples.find_all("pre") if text_of(p).lstrip().startswith("curl")), None)
            if first_pre is not None:
                first_example = to_html(renderer.code_block(first_pre, record_sim=False))
    sim = simulator_html(renderer.sim_steps)
    full = "\n".join(endpoints_html)
    if sim:
        full = full.replace("<!--SIM-->", sim, 1)
    full = full.replace("<!--SIM-->", "")
    simple = renderer.transform(page.simple)
    first_routes = list(dict.fromkeys((m, p) for e in page.endpoints for m, p in e.routes))
    badges = "".join(f'<span class="route">{method_badge(m)}<code>{esc(p)}</code></span>' for m, p in first_routes[:4])
    if len(first_routes) > 4:
        badges += f'<span class="route-more">等 {len(first_routes)} 个接口</span>'
    sim_data = json.dumps(renderer.sim_steps, ensure_ascii=False).replace("</", "<\\/")
    simple_aside = ""
    if first_example or sim:
        simple_aside = f'<aside class="ep-aside" aria-label="示例"><div class="ep-aside-inner">{first_example}{sim}</div></aside>'
    out = [head(site, page, f"{page.title} · {site.config['title']}"), header(site, page), '<div class="shell">', sidenav(site, page),
           '<main id="main" class="content">',
           f'<div class="page-top"><div class="crumb">API 参考 / {esc(page.group)}</div>',
           f'<div class="page-head"><div class="page-title"><h1>{esc(page.title)}</h1><div class="badges">{badges}<span class="tag-async">异步任务</span></div></div>{share_buttons()}</div>'
           if page.kind == "async" else
           f'<div class="page-head"><div class="page-title"><h1>{esc(page.title)}</h1><div class="badges">{badges}</div></div>{share_buttons()}</div>',
           f'<div class="lead">{summary}</div>' if summary else "",
           '<div class="mode-row"><div class="modes" role="radiogroup" aria-label="阅读模式" data-modes>'
           '<button type="button" role="radio" data-mode="simple" aria-checked="false">简明说明</button>'
           '<button type="button" role="radio" data-mode="full" aria-checked="true">完整参考</button></div>'
           '<span class="mode-hint" data-mode-hint>完整参数、示例、状态与错误格式</span></div>',
           flow, "</div>",
           f'<div class="mode-panel" data-panel="simple"><div class="endpoint has-aside simple"><div class="ep-main">{simple}'
           f'<p class="to-full"><button type="button" class="linkish" data-mode-switch="full">查看完整参数 →</button></p></div>{simple_aside}</div></div>',
           f'<div class="mode-panel" data-panel="full">{full}</div>',
           f'<script type="application/json" id="sim-data">{sim_data}</script>',
           "</main></div>", footer()]
    return "\n".join(x for x in out if x)


def overview_page(site: Site, page: Page) -> str:
    renderer = PageRenderer(site, page)
    body = renderer.transform(page.body)
    body = body.replace("<doc-endpoint-index></doc-endpoint-index>", endpoint_index_html(site))
    summary = renderer.transform(page.summary)
    legacy = {}
    for node_id, owner in sorted(site.id_owner.items()):
        if owner != "index":
            legacy[node_id] = site.pages[owner].href
    toc = "".join(f'<a href="#{esc(h.get("id"))}">{esc(text_of(h))}</a>' for h in parse(body).find_all("h2") if h.get("id"))
    legacy_json = json.dumps(legacy, ensure_ascii=False).replace("</", "<\\/")
    out = [head(site, page, site.config["title"]), header(site, page), '<div class="shell overview">', sidenav(site, page),
           '<main id="main" class="content"><div class="ov-grid"><div class="ov-main">',
           f'<div class="page-head"><div class="page-title"><h1>{esc(page.title)}</h1><div class="lead">{summary}</div></div>'
           '<div class="share" role="group" aria-label="分享本页"><button type="button" class="btn" data-share="link">复制链接</button>'
           '<button type="button" class="btn" data-share="md">复制为 Markdown</button></div></div>',
           body, "</div>",
           '<aside class="ov-aside" aria-label="辅助信息"><section class="ai-card"><h2>用 AI 助手对接</h2>'
           '<p>把这个链接发给 Agent，它能读到全部接口说明：</p><code data-llms-url>https://api.o1key.com/docs/llms.txt</code>'
           '<button type="button" class="btn btn-primary" data-share="llms">复制链接和提示词</button></section>'
           f'<nav class="toc" aria-label="本页目录"><div class="nav-label">本页目录</div>{toc}</nav></aside>',
           "</div></main></div>",
           f'<script type="application/json" id="legacy-anchors">{legacy_json}</script>',
           footer()]
    return "\n".join(out)


def endpoint_index_html(site: Site) -> str:
    rows = []
    for group, pages in site.groups:
        for page in pages:
            for e in page.endpoints:
                if not e.routes:
                    continue
                routes = "".join(f'<div class="route">{method_badge(m)}<code>{esc(p)}</code></div>' for m, p in e.routes)
                href = f"{page.href}#{e.id}" if e.id else page.href
                rows.append(f'<tr><td class="nowrap">{esc(group)}</td><td><a href="{esc(href)}">{esc(page.nav)}</a><div class="muted">{esc(e.title)}</div></td>'
                            f'<td>{routes}</td><td>{esc(e.returns)}</td></tr>')
    return ('<div class="table-wrap"><table class="index-table"><thead><tr><th>分类</th><th>模型 / 能力</th><th>接口</th><th>返回</th></tr></thead>'
            f'<tbody>{"".join(rows)}</tbody></table></div>')


def search_index(site: Site) -> list[dict]:
    entries = []
    for page in site.ordered():
        entries.append({"t": page.title, "u": page.href, "g": page.group or "对接指南", "k": page.nav})
        for e in page.endpoints:
            keys = " ".join(p for _, p in e.routes)
            fields = []
            for table in e.main.find_all("table"):
                for tr in table.find_all("tr"):
                    first = next((c for c in tr.children if c.tag == "td"), None)
                    code = first.find("code") if first is not None else None
                    if code is not None:
                        fields.append(text_of(code))
            href = f"{page.href}#{e.id}" if e.id else page.href
            entries.append({"t": e.title, "u": href, "g": page.nav, "k": f"{keys} {' '.join(dict.fromkeys(fields))}".strip()})
        for node in page.iter_content():
            for h in node.find_all("h3"):
                if h.get("id"):
                    entries.append({"t": text_of(h).strip(), "u": f"{page.href}#{h.get('id')}", "g": page.nav, "k": ""})
    return entries


# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------


def build(out: Path):
    site = Site()
    if out.exists():
        shutil.rmtree(out)
    (out / "assets").mkdir(parents=True)
    for asset in (ROOT / "assets").iterdir():
        shutil.copy2(asset, out / "assets" / asset.name)
    full_md = [llms_index(site).rstrip(), ""]
    for page in site.ordered():
        html_text = overview_page(site, page) if page.kind == "overview" else reference_page(site, page)
        (out / ("index.html" if page.slug == "index" else f"{page.slug}.html")).write_text(html_text, encoding="utf-8")
        md = page_markdown(site, page)
        (out / page.md_name).write_text(md, encoding="utf-8")
        full_md += ["", "---", "", md.split("\n---\n", 1)[1].strip() if md.startswith("---") else md]
    (out / "llms.txt").write_text(llms_index(site), encoding="utf-8")
    (out / "llms-full.txt").write_text("\n".join(full_md).strip() + "\n", encoding="utf-8")
    (out / "assets" / "search-index.json").write_text(json.dumps(search_index(site), ensure_ascii=False), encoding="utf-8")
    print(f"built {len(site.pages)} pages into {out}")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=ROOT / "dist")
    args = parser.parse_args(argv)
    build(args.out.resolve())


if __name__ == "__main__":
    sys.exit(main())
