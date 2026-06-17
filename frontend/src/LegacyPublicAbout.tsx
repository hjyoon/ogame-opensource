import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  publicImageBase,
  type LegacyPublicLoginProps
} from "./LegacyPublicHome";

export function LegacyPublicAbout({
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
        <MainMenu active="about" />
        <AboutContent />
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

function AboutContent() {
  return (
    <section className="rightmenu_big legacy-public-about-panel" id="rightmenu">
      <div className="legacy-public-title" id="title">What is OGame?</div>
      <div className="legacy-public-content" id="content">
        <div className="legacy-public-scroll" id="contentscroll">
          <p className="aboutParagraphHead legacy-about-head">
            <img alt="" className="imageRight legacy-image-right" src={`${publicImageBase}/ogame_admiral.jpg`} />
            OGame is a game of intergalactic conquest.
          </p>
          <p>
            You start out with just one undeveloped world and turn that into a <strong>mighty empire</strong> able to
            defend your hard earned colonies.
          </p>
          <p>
            Create an <strong>economic and military infrastructure</strong> to support your quest for the next greatest
            technological achievements.
          </p>
          <img alt="" className="imageLeft legacy-image-left" src={`${publicImageBase}/technik.gif`} />
          <p>
            <strong>Wage war</strong> against other empires as you struggle with other players to gain the materials.
          </p>
          <p>
            <strong>Negotiate</strong> with other emperors and create an alliance or trade for much needed resources.
          </p>
          <p>
            <strong>Build an armada</strong> to enforce your will throughout the universe.
          </p>
          <img alt="" className="imageRight legacy-image-right" src={`${publicImageBase}/laser.gif`} />
          <p>
            <strong>Hoard your resources</strong> behind an impregnable wall of planetary defences.
          </p>
          <p>
            Whatever you wish to do, <strong>OGame can let you do it.</strong>
          </p>
          <p>Will you terrorize the area around you? Or will you strike fear into the hearts of those who attack the helpless?</p>
          <a className="storyLink legacy-story-link" href="/story">
            Read the Ogame Story
          </a>
        </div>
      </div>
    </section>
  );
}
