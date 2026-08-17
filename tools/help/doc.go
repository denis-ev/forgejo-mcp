// Package help provides the read-only `help` MCP tool.
//
// The tool exists so that an agent connecting to this server with no prior
// knowledge can orient itself in a single call: which tools exist, how the
// server names IDs, which multi-tool sequences solve the common tasks, and
// which tools depend on a recent Forgejo version.
//
// The tool inventory is generated from the live MCP tool registry (see
// tools.RegisteredTools) rather than being hand-maintained, so it cannot drift
// as tools are added or renamed. Only the narrative parts - conventions,
// workflows, gotchas - are curated in this package.
package help
