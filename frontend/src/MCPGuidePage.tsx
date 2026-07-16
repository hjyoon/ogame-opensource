import { gameRouteURL } from "./gameRoutes";
import { mcpEndpointPath, mcpGuideToolGroups } from "./mcpGuide";

export function MCPGuidePage() {
  const optionsURL = gameRouteURL("/game/options", window.location.search);
  const endpointURL = `${window.location.origin}${mcpEndpointPath}`;

  return (
    <table className="legacy-overview-table legacy-mcp-guide-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c c" colSpan={2}>
            MCP Quick Guide
          </td>
        </tr>
        <tr>
          <td className="legacy-l l">Quick start</td>
          <th>
            <ol>
              <li>
                Open <a href={optionsURL}>Options</a> and create an MCP token.
              </li>
              <li>Select only the scopes your client needs and choose an expiry.</li>
              <li>Add the endpoint and token to an MCP client that supports Streamable HTTP.</li>
              <li>Let the client call <code>tools/list</code> to discover the tools currently available to you.</li>
            </ol>
          </th>
        </tr>
        <tr>
          <td className="legacy-l l">Connection</td>
          <th>
            <div>
              Server URL: <code>{endpointURL}</code>
            </div>
            <div>
              Transport: <code>Streamable HTTP</code>
            </div>
            <div>
              Authorization: <code>Bearer YOUR_TOKEN</code>
            </div>
          </th>
        </tr>
        <tr>
          <td className="legacy-l l">Scopes</td>
          <th>
            Start with <code>mcp:read</code>. Add message, fleet, queue, resource, premium, merchant, planet, alliance, account, or
            payment write scopes only when required. Operator and Admin scopes are available only to matching account roles.
          </th>
        </tr>
        <tr>
          <td className="legacy-l l">Safe mutations</td>
          <th>
            Mutation tools normally start as a dry run. Review the result, then call the tool again with <code>dryRun: false</code> and
            the returned <code>confirm</code> value.
          </th>
        </tr>
        <tr>
          <td className="legacy-c c" colSpan={2}>
            Available tools
          </td>
        </tr>
        {mcpGuideToolGroups.map((group) => (
          <tr key={group.title}>
            <td className="legacy-l l">
              {group.title}
              <br />
              <small>{group.scopes}</small>
            </td>
            <th className="legacy-mcp-guide-tools">
              {group.tools.map((tool, index) => (
                <span key={tool}>
                  {index > 0 ? ", " : null}
                  <code>{tool}</code>
                </span>
              ))}
            </th>
          </tr>
        ))}
        <tr>
          <td className="legacy-l l">Availability</td>
          <th>
            The exact list depends on your scopes, account role, Commander status, and enabled server features. The result of
            <code> tools/list</code> is always authoritative.
          </th>
        </tr>
        <tr>
          <td className="legacy-l l">Token safety</td>
          <th>
            A token secret is shown only once. Do not share it or place it in source control. Revoke unused or exposed tokens from
            <a href={optionsURL}> Options</a>.
          </th>
        </tr>
      </tbody>
    </table>
  );
}
