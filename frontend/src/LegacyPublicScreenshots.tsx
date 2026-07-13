import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
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
  { label: "Overview", thumb: "img/overview_t.jpg", href: "/screenshot?pic=overview&type=JPG&path=img/" },
  { label: "Buildings", thumb: "img/buildings_t.jpg", href: "/screenshot?pic=buildings&type=JPG&path=img/" },
  { label: "Shipyard", thumb: "img/shipyard_t.jpg", href: "/screenshot?pic=shipyard&type=JPG&path=img/" },
  { label: "Empire", thumb: "img/empire_t.jpg", href: "/screenshot?pic=empire&type=JPG&path=img/" }
];

const wallpapers = [
  {
    label: "Battleship",
    thumb: "img/battleship_t.jpg",
    href: "/screenshot?pic=battleship_1280x1024&type=jpg&path=img/wallpapers/"
  },
  {
    label: "Destroyer",
    thumb: "img/destroyer_t.jpg",
    href: "/screenshot?pic=destroyer_1280x1024&type=jpg&path=img/wallpapers/"
  }
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

function ScreenshotLink({ item }: { item: { label: string; thumb: string; href: string } }) {
  return (
    <div className="image legacy-screenshot-image">
      <a href={item.href}>
        <img alt="" src={item.thumb} />
      </a>
    </div>
  );
}
