import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  type LegacyPublicLoginProps,
  type PublicUniverse
} from "./LegacyPublicHome";

const legacyOpenDates = new Map([
  [1, "27.03.11"],
  [2, "09.05.12"],
  [3, "09.05.12"]
]);

export function LegacyPublicUniverses({
  universes,
  loginDraft,
  loginResult,
  loginPending,
  loginError,
  onLoginChange,
  onLoginSubmit
}: LegacyPublicLoginProps) {
  const listedUniverses = universes.length > 0 ? universes : fallbackUniverses;

  return (
    <main className="legacy-public-page" style={legacyPublicStyle("part_big.jpg")}>
      <a className="legacy-public-skip" href="#pustekuchen">
        Link Login
      </a>
      <div className="legacy-public-main">
        <LanguageLinks />
        <MainMenu />
        <UniversesContent universes={listedUniverses} />
        <LoginStrip
          loginDraft={loginDraft}
          loginError={loginError}
          loginPending={loginPending}
          loginResult={loginResult}
          onLoginChange={onLoginChange}
          onLoginSubmit={onLoginSubmit}
          universes={universes}
        />
      </div>
    </main>
  );
}

function UniversesContent({ universes }: { universes: PublicUniverse[] }) {
  return (
    <section className="legacy-public-universes-panel">
      <div className="legacy-public-title">Особенности во вселенных</div>
      <div className="legacy-public-content">
        <div className="legacy-public-scroll">
          <div className="legacy-universes-text">
            <p>Вселенные, на которые нельзя нажать, в данный момент заполнены.</p>
            <p>3-я Вселенная: скорость игры x4, скорость флота x2, 70% флота в обломки, 5 галактик.</p>
          </div>
          <div className="legacy-universes-title2">Вселенная:</div>
          <div className="legacy-universes-content">
            {universes.map((universe) => (
              <UniverseLink key={universe.number} universe={universe} />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function UniverseLink({ universe }: { universe: PublicUniverse }) {
  const label = universeLabel(universe);
  if (universe.open === false) {
    return (
      <span className="legacy-closed-universe" title={universe.status ?? "closed"}>
        {label}
      </span>
    );
  }
  return <a href={`/register?linkuni=${universe.number}`}>{label}</a>;
}

function universeLabel(universe: PublicUniverse): string {
  const opened = legacyOpenDates.get(universe.number);
  if (opened) {
    return `Вселенная ${universe.number} (открыта ${opened})`;
  }
  return universe.name || `Вселенная ${universe.number}`;
}

const fallbackUniverses: PublicUniverse[] = [
  { number: 1, name: "Вселенная 1", baseUrl: "", open: true },
  { number: 2, name: "Вселенная 2", baseUrl: "", open: true },
  { number: 3, name: "Вселенная 3", baseUrl: "", open: true }
];
