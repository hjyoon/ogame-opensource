import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  publicImageBase,
  type LegacyPublicLoginProps
} from "./LegacyPublicHome";

export function LegacyPublicStory({
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
      <div className="legacy-public-main">
        <LanguageLinks />
        <MainMenu active="about" />
        <StoryContent />
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

function StoryContent() {
  return (
    <section className="legacy-public-story-panel">
      <div className="legacy-public-title">Story</div>
      <div className="legacy-public-content">
        <div className="legacy-public-scroll">
          <p className="legacy-story-head">The Ogame Story</p>
          <p>
            <img alt="" className="legacy-image-right" src={`${publicImageBase}/ogame_technokrat.jpg`} />
            This is the story of a species, a race - its our race, the humans.
          </p>
          <p>
            Interestingly enough the story has not yet happened, but it should still be told. Once in a time you will
            find that time runs in parallel, that everything that was in the past, forms the present as well as the
            present is the basis for the future.
          </p>
          <p>
            <img alt="" className="legacy-image-right" src={`${publicImageBase}/fight.gif`} />
            It began in the year 2250 - the year alpha - when the first man took the risk of an interstellar flight of
            more than three minutes and thus farther than any probe had gone before.
          </p>
          <img alt="" className="legacy-image-left" src={`${publicImageBase}/ogame_ingenieur.jpg`} />
          <p>
            Certainly the competitive struggle of the involved aerospace companies has helped for new inventions and
            technological advancement. Rival companies pushed propulsion systems further until impulse engines were
            ready for real journeys.
          </p>
          <img alt="" className="legacy-image-right" src={`${publicImageBase}/ogame_geologe.jpg`} />
          <p>
            Modern engines used highly energetic deuterium fuel, and later debates over stronger fuel led to engines
            that carried cultures deeper into space.
          </p>
          <img alt="" className="legacy-image-right" src={`${publicImageBase}/light.gif`} />
          <p>
            There was enough space and places to colonize, so peace and prosperity spread through the universe for many
            decades. But it was the quiet before the storm.
          </p>
          <p>
            <img alt="" className="legacy-image-left" src={`${publicImageBase}/hyper.gif`} />
            Hyper-space engine technology let explorers travel further and colonize faster. Diplomacy was strong until
            the rare element Xentronium caused envy, conflict, and war.
          </p>
          <img alt="" className="legacy-image-right" src={`${publicImageBase}/omega.gif`} />
          <p>
            The war lasted more than 300 years. The omega bomb wiped out entire parts of a galaxy, and only a few
            survived by fleeing through wormholes into another universe.
          </p>
          <img alt="" className="legacy-big-image" src={`${publicImageBase}/legorians.jpg`} />
          <p>
            They found a single planet and encountered the legorians, who allowed the strangers to settle nearby under
            two conditions: each nation would colonize only nine planets, and each rising empire would build a senate.
          </p>
          <img alt="" className="legacy-image-left" src={`${publicImageBase}/ogame_admiral_left.jpg`} />
          <p>
            Follow us, and see what you can do with a nation that awaits a new emperor. The task will not always be
            peaceful, but your will and power could create a prosperous nation.
          </p>
          <p>I will leave you now... hoping you would join us... yet its your decision... Dare it!</p>
          <a className="legacy-join-link" href="/register">
            Join now!
          </a>
        </div>
      </div>
    </section>
  );
}
