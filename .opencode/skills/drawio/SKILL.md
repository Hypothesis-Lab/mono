---
name: drawio
description: Create, generate, draw, or edit draw.io diagrams, flowcharts, architecture diagrams, sequence diagrams, ER diagrams, network diagrams, mockups, wireframes, or diagram exports.
---

# Draw.io diagrams

Use this skill whenever the user asks for a draw.io/diagrams.net diagram, a
pipeline visualization, a flowchart, or an export to PNG, SVG, or PDF.

## Preferred workflow

1. Use the project draw.io MCP tools when they are available:
   - `open_drawio_xml` for native draw.io XML;
   - `open_drawio_mermaid` for ordinary flowcharts and standard diagrams;
   - `open_drawio_csv` for tabular diagrams;
   - `search_shapes` only when branded or domain-specific shapes are needed;
   - `list_pages`, `get_page`, and `set_page` for local multi-page `.drawio`
     files.
2. Keep the authoritative source data in the repository. Do not invent
   pipeline steps, labels, IDs, or relationships that are not supported by the
   source files or the user's request.
3. Prefer Mermaid for standard diagrams. Use native XML when precise layout,
   custom styling, containers, metadata, or multi-page structure is needed.
4. Keep node labels short. Put large prompts, evidence, and implementation
   details into linked detail pages or a companion report rather than trying
   to fit them into the overview node.
5. Validate generated XML before opening it: use a well-formed XML parser and
   ensure the diagram contains root cells `0` and `1`, unique cell IDs, and a
   geometry child for every edge.

## Without MCP

If the MCP server is unavailable, create a native `.drawio` XML file directly.
The minimal structure is:

```xml
<mxGraphModel adaptiveColors="auto">
  <root>
    <mxCell id="0"/>
    <mxCell id="1" parent="0"/>
  </root>
</mxGraphModel>
```

Use escaped XML attribute values (`&amp;`, `&lt;`, `&gt;`, and `&quot;`) and
never put XML comments in generated diagram files. If draw.io Desktop is
installed, it can convert Mermaid to `.drawio` and export `.drawio` files to
PNG, SVG, or PDF. Otherwise deliver the native `.drawio` file and state that
the desktop CLI is not available.

## Deliverables

- Default: an editable `.drawio` file.
- For a static preview: `.drawio.svg`, `.drawio.png`, or `.drawio.pdf` with
  embedded diagram XML when supported.
- For large prompts or detailed inspection: a self-contained HTML report with
  a clickable details panel, in addition to the editable `.drawio` overview.

The project MCP server is configured in the repository's `opencode.json` and
uses the pinned official `@drawio/mcp` version `1.9.0` over local stdio. It
opens diagrams in the draw.io web editor; it does not send the diagram to an
external AI service. The diagram payload is carried in the editor URL produced
by the server.

Upstream skill and server:
https://github.com/jgraph/drawio-mcp
