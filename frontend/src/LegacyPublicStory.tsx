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
      <div className="legacy-public-main" id="main">
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
    <section className="rightmenu_big legacy-public-story-panel" id="rightmenu">
      <div className="legacy-public-title" id="title">Story</div>
      <div className="legacy-public-content" id="content">
        <div className="legacy-public-scroll" id="contentscroll">
          <p className="storyParagraphHead legacy-story-head">The Ogame Story</p>
          <div dangerouslySetInnerHTML={{ __html: storyHtml }} />
          <a className="joinLink legacy-join-link" href="/register">
            Join now!
          </a>
        </div>
      </div>
    </section>
  );
}

const rawStoryHtml = `
<p><img src="img/ogame_technokrat.jpg" class="imageRight legacy-image-right"> This is the story of a species, a race - its our race, the humans.</p> <p >Interestingly enough the story has not yet happened, but it should still be told. Once in a time you will find that time runs in parallel, that everything that was in the past, forms the present as well as the present is the basis for the future. There has been found a way to visit the past without altering the future. Because one can only alter the future when living it for oneself. This might be hard to understand, but its true. Only when you know that something has happened, you can change it, because it then is part of one's own past. You will simply disappear from that point on, no changes will take place, because this is already the past.. your past.</p>
      <p >
       <img src="img/fight.gif" class="imageRight legacy-image-right">
      It began in the year 2250 - the year 'alpha' - when the first man took the risk of an interstellar flight of more than three minutes and thus farther than any probe had gone before. It was a new kind of engine called impulse engine, which allowed mankind to travel at the speed of light. Yes there were earlier space flights, but those were limited, a mere glimpse into space compared to the later achievements. But mankind's natural impulse for new adventures continued to push the development further and thus prolonging the space flights little by little over the years. </p>
      <img src="img/ogame_ingenieur.jpg" class="imageLeft legacy-image-left">
      <p >Certainly the competitive struggle of the involved aerospace companies has helped for new inventions and technological advancement. While Boing Industries still relied on their old combustion engine that has been banned from the earths atmosphere, rival ErBus tried to put aside the various troubles they encountered with their impulse engine. Drawbacks and explosions at first dampened the vision of a new kind of technology, but in the process of development the Y-particles were replaced by a chemical compound being more safe, thanks to the theoretical contributions of the well-known particle physicist Al Ainstone. Only slightly aside the main stage, smaller companies and private groups also felt the need for space exploration and the demand for better and faster yet more efficient propulsion systems was immanent. </p>
      <img src="img/ogame_geologe.jpg" class="imageRight legacy-image-right">
       <p >Modern engines were using highly energetic deuterium fuel allowing them to travel from galaxy to galaxy in relatively short amounts of time. Unfortunately, this deuterium was very rare, on earth only 0.015% of all water was in the form of this heavy isotope. Later, even the best deuterium wasn't good enough anymore, and a new scientific debate was born, discussing over new ways to generate an even better fuel than deuterium itself already was. Based on a well known theory, it was believed to be possible to combine deuterium with a mysterious gas emitted by a quasar's explosion. The quantum-chemical reaction with this mixture would set free huge amounts of energy, so much more that ships could travel faster and faster. </p><p >Scientist Fred Terrane put his hands together and started to design an engine based on the above theory. Despite the criticism from almost every scientific side, the new propulsion engine finally showed the theory to work - although based on the current level of scientific knowledge, it shouldn't have to. In real-world applications the new machinery proved to live up to its promises and cultures all around were able to head for outer space even more easily and even faster.</p>
       <img src="img/light.gif" class="imageRight legacy-image-right">
       <p > There was well enough space and places to colonize and so peace and prosperity quickly spread in the universe and remained so for many decades. Ever more new cultures were discovered and also created and life spread into every corner of space. The Universe was light and peaceful. But it was the quiet before the storm... </p><p >Even after this prosperous period, new and faster technologies for propulsion systems were investigated. Based on old theories and eventually combined with the eager of young student, a completely revolutionary propulsion technology was created that put everything away known so far... </p><p ><img src="img/hyper.gif" class="imageLeft legacy-image-left">The Hyper-space engine technology. Basically a hyper-dimensional bubble being in the fourth and the fifth dimensions yet isolating its innards from these influences and obeying the laws of the four-dimensional space. There is a narrow field of vision on the galaxy in direction of movement. Using this outstanding Engine technology, explorers could go even further into the deep space and colonization took place much faster due to decreased travel times. Again, a new era was entered, the development speed increased again and many new worlds were discovered. Diplomacy was big in these times and there were not many conflicts between races. Until an previously unknown chemical element named Xentronium was discovered on a some people's colony. The war should begin soon... </p><p >This chemical element was extremely rare to find, and during the first years after its discovery, only 1.5 micrograms could be harvested. Because it had been found on terran ground, only laboratories of the terrans would be able to investigate on this new element. Xentronium was believed to have the power of 10'000 terran suns per microgram! Once this knowledge spread across the universe, envy and ill will arose from the dark and caused conflicts among the nations which sadly enough ended in a war. Quickly two parties were established, those to the terran fraction and those against the terrans. </p>
      <img src="img/omega.gif" class="imageRight legacy-image-right">
       <p >It was Nostradamus who once saw that the war would last for 150 years, but reality proved him wrong as the war wasted more than 300 years. Thousands and thousands of nations were drawn apart, fell or completely disappeared of annihilation.The side of the enemies to the terran federation started to use some ultimate weapon by the name "omega bomb". As the name already suggests, usage of this bomb wipes out entire parts of a galaxy, freeing it from any civilization. Hundreds of years after the new time system and the year alpha was established, the universe was faced with the most dangerous and most deadly weapons ever created, solarsystem after solarsystem was destroyed and literally removed. Calculations were conducted to predict the outcome: Only a mere 24 years after the first omega bomb was dropped, known space within the universe would be wiped out leaving lifeless space behind...</p><p > Only few were able to survive, and even lesser were able to flee through wormholes and make their way out into new areas of the universe - but it was a way of no return to them. A new life...</p><p > With great relief the people found themselves in an entirely different universe where they had never been before.. interestingly enough, they only found a single planet in the entire area and as far as they were able to see...</p>
       <img src="img/legorians.jpg" class="bigImage legacy-big-image">
       <p >They dared to settle on this peace of universal driftwood and encountered quite a strange life-form, a nation to the name "legorians". The visitors were treated well, and the legorians had a similar technical knowledge. Eventually, the legorians agreed to let the strangers settle in the near environment of the legorian's inhabited world. However there were two conditions: Every nation, every race, would only colonize 9 planets, so that others would also get the change to settle for a new civilization after those long conflicts and their escape. The second condition was to create a senate on the homeworld of every rising empire allowing the people to debate and talk. Of course the nations and their representatives agreed thankfully and even established trading associations and guarding troops to ensure peace and fairness. A new civilization arose... </p>
      <img src="img/ogame_admiral_left.jpg" class="imageLeft legacy-image-left">
       <p >You might be asking by now, why we wanted to tell you this story. The answer is very simple, it is for you come with us to build the future! Follow us, and see what YOU can do with a nation thats awaits a new emperor desperately ready to grow and succeed! You will have to put much work into this task, and the times wont always be peaceful, but it is up to you, to take part in this part of the universe and save your nation a respected seat among all the empires. Follow us through this time portal, and enjoy this world full of new things and an big future. It might be hard from time to time, it might be easy now and then, but your will and your power could create a powerful and prosperous nation. </p><p >I will leave you now... hoping you would join us... yet its your decision... Dare it! </p>
`;

const storyHtml = rawStoryHtml.replaceAll('src="img/', `src="${publicImageBase}/`);
