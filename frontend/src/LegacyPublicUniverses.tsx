import { LoginStrip, MainMenu, legacyPublicStyle, type LegacyPublicLoginProps } from "./LegacyPublicHome";

export function LegacyPublicUniverses({
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
        <MainMenu />
        <UniversesContent />
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

function UniversesContent() {
  return (
    <section className="rightmenu_big legacy-public-universes-panel" id="rightmenu">
      <div className="legacy-public-title" id="title">Особенности во вселенных</div>
      <div className="legacy-public-content" id="content">
        <div className="legacy-public-scroll" id="contentscroll">
          <div className="legacy-universes-text" id="text1">
            <p>Вселенные, на которые нельзя нажать, в данный момент заполнены.</p>
            <p>3-я Вселенная: скорость игры x4, скорость флота x2, 70% флота в обломки, 5 галактик.</p>
          </div>
          <div className="legacy-universes-title2" id="unis_title2">Вселенная:</div>
          <div className="legacy-universes-content" id="unis_content">
            <a href="/register?linkuni=1">Вселенная 1 (открыта 27.03.11)</a>
            <a href="/register?linkuni=2">Вселенная 2 (открыта 09.05.12)</a>
            <a href="/register?linkuni=3">Вселенная 3 (открыта 09.05.12)</a>
          </div>
        </div>
      </div>
    </section>
  );
}
