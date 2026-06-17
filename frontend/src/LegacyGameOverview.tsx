import React from "react";

export type GameOverviewStatus = {
  authenticated: boolean;
  issues: { code: string; message: string }[];
  overview?: GameOverview;
};

type GameOverview = {
  commander: string;
  score: {
    points: number;
    rawScore: number;
    rank: number;
    universePlayers: number;
  };
  currentPlanet: GamePlanetOverview;
  planetSwitcher: GamePlanetSummary[];
};

type GamePlanetOverview = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  diameter: number;
  temperature: number;
  fields: number;
  maxFields: number;
  resources: Resources;
};

type GamePlanetSummary = {
  id: number;
  name: string;
  type: number;
  coordinates: Coordinates;
  current: boolean;
};

type Coordinates = {
  galaxy: number;
  system: number;
  position: number;
};

type Resources = {
  metal: number;
  crystal: number;
  deuterium: number;
};

type LegacyGameOverviewProps = {
  status: GameOverviewStatus | null;
  error: string | null;
};

const skinBase = "/legacy-assets/use/uV";

const menuItems = [
  "Overview",
  "Empire",
  "Buildings",
  "Resources",
  "Research",
  "Shipyard",
  "Fleet",
  "Technology",
  "Galaxy",
  "Defense",
  "Alliance",
  "Statistics",
  "Messages"
];

export function LegacyGameOverview({ status, error }: LegacyGameOverviewProps) {
  const overview = status?.authenticated ? status.overview : undefined;
  const issue = status && !status.authenticated ? status.issues[0]?.message ?? "Session is invalid." : null;

  return (
    <main
      className="legacy-game-shell"
      style={
        {
          "--legacy-body-bg": `url("${skinBase}/img/background.jpg")`,
          "--legacy-title-bg": `url("${skinBase}/img/bg1.gif")`
        } as React.CSSProperties
      }
    >
      <header className="legacy-header-top">
        {overview ? <LegacyResourceHeader overview={overview} /> : <div className="legacy-header-placeholder">OGame</div>}
      </header>
      <LegacyLeftMenu />
      <section className="legacy-content">
        {error ? <LegacyMessage tone="error" text={error} /> : null}
        {!error && issue ? <LegacyMessage tone="error" text={issue} /> : null}
        {!error && !issue && !overview ? <LegacyMessage tone="neutral" text="Loading overview..." /> : null}
        {overview ? <OverviewTable overview={overview} /> : null}
      </section>
    </main>
  );
}

function LegacyResourceHeader({ overview }: { overview: GameOverview }) {
  const planet = overview.currentPlanet;
  const resources = [
    { name: "Metal", value: planet.resources.metal, img: `${skinBase}/images/metall.gif` },
    { name: "Crystal", value: planet.resources.crystal, img: `${skinBase}/images/kristall.gif` },
    { name: "Deuterium", value: planet.resources.deuterium, img: `${skinBase}/images/deuterium.gif` },
    { name: "Energy", value: 0, secondary: 0, img: `${skinBase}/images/energie.gif` }
  ];

  return (
    <table className="legacy-header-table">
      <tbody>
        <tr>
          <td className="legacy-header-cell">
            <table className="legacy-header-table">
              <tbody>
                <tr>
                  <td className="legacy-header-cell">
                    <img alt="" height={50} src={planetImagePath(planet, true)} width={50} />
                  </td>
                  <td className="legacy-header-cell">
                    <select
                      aria-label="Planet selector"
                      onChange={(event) => {
                        window.history.pushState({}, "", planetHref(Number(event.currentTarget.value)));
                        window.dispatchEvent(new PopStateEvent("popstate"));
                      }}
                      value={planet.id}
                    >
                      {overview.planetSwitcher.map((item) => (
                        <option key={item.id} value={item.id}>
                          {item.name} [{formatCoordinates(item.coordinates)}]
                        </option>
                      ))}
                    </select>
                  </td>
                </tr>
              </tbody>
            </table>
          </td>
          <td className="legacy-header-cell">
            <table className="legacy-resource-table" id="resources">
              <tbody>
                <tr>
                  {resources.map((resource) => (
                    <td className="legacy-header-cell" key={resource.name}>
                      <img alt="" height={22} src={resource.img} width={42} />
                    </td>
                  ))}
                </tr>
                <tr>
                  {resources.map((resource) => (
                    <td className="legacy-header-cell legacy-resource-name" key={resource.name}>
                      {resource.name}
                    </td>
                  ))}
                </tr>
                <tr>
                  {resources.map((resource) => (
                    <td className="legacy-header-cell" key={resource.name}>
                      {formatNumber(resource.value)}
                      {resource.secondary !== undefined ? `/${formatNumber(resource.secondary)}` : null}
                    </td>
                  ))}
                </tr>
              </tbody>
            </table>
          </td>
        </tr>
      </tbody>
    </table>
  );
}

function LegacyLeftMenu() {
  return (
    <aside className="legacy-leftmenu">
      <div className="legacy-center">
        <div className="legacy-menu">
          <p>
            <span className="legacy-nowrap">Universe 1 (v 0.84)</span>
          </p>
          <table>
            <tbody>
              <tr>
                <td>
                  <img alt="" height={40} src={`${skinBase}/gfx/ogame-produktion.jpg`} width={110} />
                </td>
              </tr>
              {menuItems.map((item) => (
                <tr key={item}>
                <td>
                    <div className="legacy-center">
                      <a href="/game/overview">{item}</a>
                    </div>
                  </td>
                </tr>
              ))}
              <tr>
                <td>
                  <img alt="" height={19} src={`${skinBase}/gfx/info-help.jpg`} width={110} />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </aside>
  );
}

function OverviewTable({ overview }: { overview: GameOverview }) {
  const planet = overview.currentPlanet;
  const moon = overview.planetSwitcher.find(
    (item) =>
      item.type === 0 &&
      item.coordinates.galaxy === planet.coordinates.galaxy &&
      item.coordinates.system === planet.coordinates.system &&
      item.coordinates.position === planet.coordinates.position
  );
  const otherPlanets = overview.planetSwitcher.filter((item) => item.type !== 0 && item.id !== planet.id);

  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c" colSpan={4}>
            <a href="/game/overview" title="Planet menu">
              Planet "{planet.name}"
            </a>{" "}
            ({overview.commander})
          </td>
        </tr>
        <tr>
          <th>Server time</th>
          <th colSpan={3}>{new Date().toUTCString()}</th>
        </tr>
        <tr>
          <td className="legacy-c" colSpan={4}>
            Events
          </td>
        </tr>
        <tr>
          <th colSpan={4}>&nbsp;</th>
        </tr>
        <tr>
          <th>
            {moon ? (
              <>
                {moon.name}
                <br />
                <a href={planetHref(moon.id)}>
                  <img alt="Moon" height={50} src={planetImagePath(moon, true)} width={50} />
                </a>
              </>
            ) : null}
          </th>
          <th colSpan={2}>
            <img alt="" height={200} src={planetImagePath(planet, false)} width={200} />
            <br />
            <div className="legacy-center">free</div>
            <br />
          </th>
          <th className="legacy-s">
            <table className="legacy-planet-list">
              <tbody>
                {otherPlanets.length === 0 ? (
                  <tr>
                    <th>&nbsp;</th>
                  </tr>
                ) : (
                  rowsOfTwo(otherPlanets).map((row, index) => (
                    <tr key={index}>
                      {row.map((item) => (
                        <th key={item.id}>
                          {item.name}
                          <br />
                          <a href={planetHref(item.id)} title={`${item.name} [${formatCoordinates(item.coordinates)}]`}>
                            <img
                              alt=""
                              height={50}
                              src={planetImagePath(item, false)}
                              title={`${item.name} [${formatCoordinates(item.coordinates)}]`}
                              width={50}
                            />
                          </a>
                          <br />
                          <div className="legacy-center">free</div>
                        </th>
                      ))}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </th>
        </tr>
        <tr>
          <th>Diameter</th>
          <th colSpan={3}>{formatNumber(planet.diameter)} km</th>
        </tr>
        <tr>
          <th>Fields</th>
          <th colSpan={3}>
            {planet.fields} / {planet.maxFields}
          </th>
        </tr>
        <tr>
          <th>Temperature</th>
          <th colSpan={3}>
            approx. {planet.temperature}C to {planet.temperature + 40}C
          </th>
        </tr>
        <tr>
          <th>Position</th>
          <th colSpan={3}>
            <a href="/game/overview">[{formatCoordinates(planet.coordinates)}]</a>
          </th>
        </tr>
        <tr>
          <th>Points</th>
          <th colSpan={3}>
            {formatNumber(overview.score.points)} (Rank{" "}
            <a href="/game/overview">{formatNumber(overview.score.rank)}</a> of {formatNumber(overview.score.universePlayers)})
          </th>
        </tr>
      </tbody>
    </table>
  );
}

function LegacyMessage({ tone, text }: { tone: "error" | "neutral"; text: string }) {
  return (
    <table className="legacy-overview-table" width={519}>
      <tbody>
        <tr>
          <td className="legacy-c">{tone === "error" ? "Error" : "Overview"}</td>
        </tr>
        <tr>
          <th>{text}</th>
        </tr>
      </tbody>
    </table>
  );
}

function rowsOfTwo(items: GamePlanetSummary[]): GamePlanetSummary[][] {
  const rows: GamePlanetSummary[][] = [];
  for (let index = 0; index < items.length; index += 2) {
    rows.push(items.slice(index, index + 2));
  }
  return rows;
}

function planetHref(planetID: number): string {
  const search = new URLSearchParams(window.location.search);
  search.set("cp", String(planetID));
  return `/game/overview?${search.toString()}`;
}

function planetImagePath(planet: GamePlanetOverview | GamePlanetSummary, small: boolean): string {
  if (planet.type === 0) {
    return `${skinBase}/planeten/${small ? "small/s_" : ""}mond.jpg`;
  }
  const imageID = (planet.id % 10) + 1;
  const category = planetCategory(planet.coordinates.position);
  const filename = `${category}${String(imageID).padStart(2, "0")}.jpg`;
  return `${skinBase}/planeten/${small ? "small/s_" : ""}${filename}`;
}

function planetCategory(position: number): string {
  if (position <= 3) {
    return "trockenplanet";
  }
  if (position <= 6) {
    return "dschjungelplanet";
  }
  if (position <= 9) {
    return "normaltempplanet";
  }
  if (position <= 12) {
    return "wasserplanet";
  }
  return "eisplanet";
}

function formatCoordinates(coordinates: Coordinates): string {
  return `${coordinates.galaxy}:${coordinates.system}:${coordinates.position}`;
}

function formatNumber(value: number): string {
  return Math.floor(Math.max(0, value)).toLocaleString("en-US");
}
