import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  publicImageBase,
  type LegacyPublicLoginProps
} from "./LegacyPublicHome";

export function LegacyPublicScreenshots({
  universes,
  loginDraft,
  loginResult,
  loginPending,
  loginError,
  onLoginChange,
  onLoginSubmit
}: LegacyPublicLoginProps) {
  return (
    <main className="legacy-public-page" style={legacyPublicStyle("part_big.jpg")}>
      <a className="legacy-public-skip" href="#pustekuchen">
        Link Login
      </a>
      <div className="legacy-public-main" id="main">
        <LanguageLinks />
        <MainMenu active="preview" />
        <ScreenshotsContent />
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

const screenshots = [
  { label: "Overview", thumb: "overview_t.jpg", full: "overview.JPG" },
  { label: "Buildings", thumb: "buildings_t.jpg", full: "buildings.JPG" },
  { label: "Shipyard", thumb: "shipyard_t.jpg", full: "shipyard.JPG" },
  { label: "Empire", thumb: "empire_t.jpg", full: "empire.JPG" }
];

const wallpapers = [
  { label: "Battleship", thumb: "battleship_t.jpg", full: "wallpapers/battleship_1280x1024.jpg" },
  { label: "Destroyer", thumb: "destroyer_t.jpg", full: "wallpapers/destroyer_1280x1024.jpg" }
];

function ScreenshotsContent() {
  return (
    <section className="rightmenu_big legacy-public-screenshots-panel" id="rightmenu">
      <div className="legacy-public-title" id="title">Pictures</div>
      <div className="legacy-public-content" id="content">
        <div className="legacy-public-scroll legacy-screenshots-scroll" id="contentscroll" style={{ textAlign: "center" }}>
          <p className="headline legacy-screenshots-headline">Screenshots</p>
          {screenshots.map((item) => (
            <ScreenshotLink item={item} key={item.thumb} />
          ))}
          <p className="headline legacy-screenshots-headline">Wallpapers</p>
          {wallpapers.map((item) => (
            <ScreenshotLink item={item} key={item.thumb} />
          ))}
        </div>
      </div>
    </section>
  );
}

function ScreenshotLink({ item }: { item: { label: string; thumb: string; full: string } }) {
  return (
    <div className="image legacy-screenshot-image">
      <a href={`${publicImageBase}/${item.full}`}>
        <img alt={item.label} src={`${publicImageBase}/${item.thumb}`} />
      </a>
    </div>
  );
}
