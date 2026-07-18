export type MCPGuideToolGroup = {
  title: string;
  scopes: string;
  tools: string[];
};

export const mcpEndpointPath = "/mcp";

export const mcpGuideToolGroups: MCPGuideToolGroup[] = [
  {
    title: "Public and access",
    scopes: "Public / mcp:read",
    tools: ["get_server_health", "get_mcp_access"]
  },
  {
    title: "Game state",
    scopes: "mcp:read",
    tools: [
      "list_planets",
      "get_account_overview",
      "get_planet_resources",
      "get_resource_production_options",
      "get_building_queue",
      "get_fleet_movements",
      "get_fleet_options",
      "get_officer_status",
      "search_game",
      "get_statistics",
      "get_alliance_status",
      "get_buddy_status",
      "get_pranger",
      "get_notes",
      "get_options",
      "get_maintenance",
      "get_merchant_status",
      "get_jump_gate_status",
      "get_empire_overview",
      "get_technology_tree",
      "get_building_options",
      "get_research_options",
      "get_shipyard_options",
      "get_defense_options"
    ]
  },
  {
    title: "Galaxy exploration",
    scopes: "mcp:read + mcp:resources_write",
    tools: ["get_galaxy_system"]
  },
  {
    title: "Messages and reports",
    scopes: "mcp:messages / mcp:message_write",
    tools: ["list_messages", "get_message", "get_report", "send_message", "delete_messages", "report_message"]
  },
  {
    title: "Notes and buddies",
    scopes: "mcp:notes_write / mcp:buddy_write",
    tools: ["create_note", "update_note", "delete_notes", "mutate_buddy"]
  },
  {
    title: "Fleet and galaxy",
    scopes: "mcp:fleet_write",
    tools: [
      "validate_fleet_dispatch",
      "dispatch_fleet",
      "recall_fleet",
      "scan_phalanx",
      "jump_gate",
      "mutate_fleet_template",
      "launch_interplanetary_missiles",
      "dispatch_galaxy_action"
    ]
  },
  {
    title: "Economy and queues",
    scopes: "Queue, resource, premium, and merchant write scopes",
    tools: [
      "cancel_building_queue",
      "cancel_research_queue",
      "enqueue_shipyard_order",
      "update_resource_production",
      "recruit_officer",
      "mutate_merchant",
      "mutate_building",
      "start_research",
      "mutate_commander_queue"
    ]
  },
  {
    title: "Account and alliance",
    scopes: "Planet, alliance, account, and payment write scopes",
    tools: ["mutate_planet", "mutate_alliance", "update_account_options", "redeem_coupon"]
  },
  {
    title: "Operator and Admin",
    scopes: "mcp:operator / mcp:admin",
    tools: ["get_admin_access", "get_admin_panel", "mutate_admin_panel"]
  }
];
