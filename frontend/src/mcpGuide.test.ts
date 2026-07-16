import { describe, expect, test } from "bun:test";
import { mcpEndpointPath, mcpGuideToolGroups } from "./mcpGuide";

describe("MCP guide inventory", () => {
  test("documents every currently exposed MCP tool exactly once", () => {
    const tools = mcpGuideToolGroups.flatMap((group) => group.tools);

    expect(tools).toHaveLength(61);
    expect(new Set(tools).size).toBe(61);
    expect(tools).toContain("get_server_health");
    expect(tools).toContain("get_account_overview");
    expect(tools).toContain("dispatch_fleet");
    expect(tools).toContain("mutate_admin_panel");
  });

  test("uses the Streamable HTTP MCP endpoint", () => {
    expect(mcpEndpointPath).toBe("/mcp");
  });
});
