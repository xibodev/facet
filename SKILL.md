# Facet Producer

Use this skill to turn a video request into a reviewed delivery. Claude Code is
the sole producer and orchestrator; personas are reasoning postures, not agents.

## Produce

1. Understand the requested outcome, audience, source of truth, supplied
   materials, delivery profile, budget, and constraints. Inspect supplied files
   before committing to an approach, and clarify only decisions that materially
   affect the result. Confirm the user may use identifiable people, music, and
   other supplied material when rights or consent are unclear.
2. Select one primary production pipeline from the dominant transformation, not
   from the genre, format, or platform:
   - For supplied footage, use the core source-edit workflow.
   - For 2D motion graphics and explainers, use `@xibodev/facet-pack-explainer`.
   - For archival or documentary montage, use `@xibodev/facet-pack-cinematic`.
3. Adopt one editor-producer posture: protect the user's intent and source
   truth, choose moments deliberately, prefer clarity and continuity over
   effects, and make editorial tradeoffs explicit. Do not create a subagent or
   hand creative control to the toolbox.
4. Query the live toolbox with `facet tools list`, then use
   `facet tools describe <tool>` for relevant candidates. Treat
   implementation, dependency, network, cost, and configuration data as live
   facts. Never infer availability from this documentation or let a ranking
   silently choose a provider.
5. Propose a feasible route before consequential work. Prefer a supplied,
   local, non-paid route when it can meet the request; explain material quality,
   time, source, and output compromises. Ask before paid work, publication,
   external mutation, or a material creative downgrade.
6. Load only the selected method and the knowledge it references. Create only
   artifacts that help execute, inspect, revise, or explain this production;
   ordinary project files are records, not mandatory stages or workflow state.
7. Estimate when a tool supports estimation, then invoke described tools with
   explicit inputs and outputs. Read structured results and warnings, diagnose
   failures, and do not claim an artifact exists until it is inspected.
8. Review technical evidence and the rendered video's editorial result. Revise
   defects that matter to the request, rerender as needed, and repeat focused
   review rather than accepting tool execution as creative approval.
9. Finish with a compact delivery record containing output location, material
   provenance, consequential editorial and provider choices, cost and network
   facts, review outcome, and known limitations. Return the completed video and
   point to the useful project records created for it.

Do not introduce a swarm, host-owned state, mandatory artifact sequence,
provider auto-choice, or hidden pipeline progression.
