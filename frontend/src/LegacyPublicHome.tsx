import React from "react";

export type PublicUniverse = {
  number: number;
  name: string;
  baseUrl: string;
  speed?: number;
  fleetSpeed?: number;
  status?: string;
  open?: boolean;
};

export type PublicLoginDraft = {
  login: string;
  pass: string;
  universe: string;
};

export type PublicLoginIssue = {
  field: string;
  code: string;
  message: string;
};

export type PublicLoginResult = {
  valid: boolean;
  issues: PublicLoginIssue[];
  session?: {
    redirectTo: string;
  };
};

type LegacyPublicHomeProps = {
  universes: PublicUniverse[];
  loginDraft: PublicLoginDraft;
  loginResult: PublicLoginResult | null;
  loginPending: boolean;
  loginError: string | null;
  onLoginChange: (field: keyof PublicLoginDraft, value: string) => void;
  onLoginSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
};

export type LegacyPublicLoginProps = Pick<
  LegacyPublicHomeProps,
  "universes" | "loginDraft" | "loginResult" | "loginPending" | "loginError" | "onLoginChange" | "onLoginSubmit"
>;

export const publicImageBase = "/public-assets/img";
const legacyAlignRightProps = { align: "right" } as React.HTMLAttributes<HTMLDivElement> & { align: string };
type LegacyPublicLanguage = "de" | "en" | "fr" | "it" | "ru";

type LegacyPublicTexts = {
  menuStart: string;
  menuAbout: string;
  menuPictures: string;
  menuRegister: string;
  loginLink: string;
  loginName: string;
  loginPass: string;
  loginChooseUniverse: string;
  loginUniverse: string;
  loginConfirm: string;
  loginImpressum: string;
  loginRemind: string;
  loginNotChosen: string;
  chooseLanguage: string;
  copyright: string;
  downRules: string;
  downImprint: string;
  downTerms: string;
  homeTitle: string;
  homeText1: string;
  homeText2: string;
  homeButton: string;
};

const legacyPublicTextsByLanguage: Record<LegacyPublicLanguage, LegacyPublicTexts> = {
  de: {
    menuStart: "Startseite",
    menuAbout: "Über OGame",
    menuPictures: "Bilder",
    menuRegister: "Mitspielen",
    loginLink: "Link Login",
    loginName: "Spielername",
    loginPass: "Passwort",
    loginChooseUniverse: "Universum auswählen...",
    loginUniverse: "Universum",
    loginConfirm: "Mit dem Login akzeptiere ich die",
    loginImpressum: "AGB",
    loginRemind: "Passwort vergessen?",
    loginNotChosen: "Du hast kein Universum ausgewählt.",
    chooseLanguage: "Wählen Sie Ihre Sprache",
    copyright: "Alle Rechte vorbehalten.",
    downRules: "Regeln",
    downImprint: "Impressum",
    downTerms: "AGB",
    homeTitle: "Willkommen bei OGame",
    homeText1:
      "<strong>OGame</strong> ist ein <strong>Strategiespiel, das im Weltraum</strong> spielt. <strong>Tausende Spieler</strong> treten zur <strong>gleichen Zeit</strong> gegeneinander an. Zum Spielen brauchst du nur einen normalen Webbrowser.",
    homeText2: "Melde dich an und lerne die fantastische Welt von OGame kennen!",
    homeButton: "Jetzt kostenlos mitspielen!"
  },
  en: {
    menuStart: "Start",
    menuAbout: "About OGame",
    menuPictures: "Pictures",
    menuRegister: "Join Now!",
    loginLink: "Link Login",
    loginName: "Username",
    loginPass: "Password",
    loginChooseUniverse: "Choose a universe...",
    loginUniverse: "Universe",
    loginConfirm: "By logging in, I accept the",
    loginImpressum: "T&C's",
    loginRemind: "Forgot your password?",
    loginNotChosen: "You haven't chosen a universe.",
    chooseLanguage: "Choose your language",
    copyright: "All rights reserved.",
    downRules: "Rules",
    downImprint: "Imprint",
    downTerms: "T&C's",
    homeTitle: "Welcome to OGame",
    homeText1:
      "<strong>OGame</strong> is a <strong>strategic space simulation game</strong>with \n<strong>thousands of players</strong> across the world competing with each other <strong>simultaneously</strong>. All you need to play is a standard web browser.",
    homeText2: "Register now and enter the fantastic world of OGame!",
    homeButton: "Play for free now!"
  },
  fr: {
    menuStart: "Page de démarrage",
    menuAbout: "A propos d'OGame",
    menuPictures: "Captures d'écran",
    menuRegister: "S'inscrire",
    loginLink: "Link Login",
    loginName: "Nom de joueur",
    loginPass: "Mot de passe",
    loginChooseUniverse: "Choisissez l'univers...",
    loginUniverse: "Univers",
    loginConfirm: "En me loggant, j'accepte les ",
    loginImpressum: "conditions générales",
    loginRemind: "Vous avez oublié votre mot de passe ?",
    loginNotChosen: "Vous n'avez pas choisi d'univers.",
    chooseLanguage: "Choisissez votre langue",
    copyright: "Tous droits réservés.",
    downRules: "Règles du jeu",
    downImprint: "Informations légales",
    downTerms: "conditions générales",
    homeTitle: "Bienvenue sur OGame",
    homeText1:
      "<strong>OGame</strong> est un <strong>jeu de stratégie dans l'espace</strong>. <strong>Des milliers de joueurs</strong> s'y affrontent en <strong>même temps</strong>. Pour jouer, il suffit d'un navigateur internet.",
    homeText2: "Inscrivez-vous et découvrez le monde fantastique d'OGame",
    homeButton: "Jouez dès maintenant gratuitement!"
  },
  it: {
    menuStart: "Pagina Iniziale",
    menuAbout: "A proposito di OGame",
    menuPictures: "Immagini",
    menuRegister: "Registrati ora!",
    loginLink: "Link Login",
    loginName: "Nick di gioco",
    loginPass: "Password",
    loginChooseUniverse: "Scegli un universo...",
    loginUniverse: "Universo",
    loginConfirm: "Entrando, accetti i",
    loginImpressum: "T&C;",
    loginRemind: "Hai dimenticato la tua password?",
    loginNotChosen: "Non hai scelto nessun universo.",
    chooseLanguage: "Scegliere la lingua",
    copyright: "Tutti i diritti riservati.",
    downRules: "Regole",
    downImprint: "Contatti",
    downTerms: "T&C;",
    homeTitle: "Benvenuti a OGame",
    homeText1:
      "<strong>OGame</strong> è un <strong>gioco strategico di simulazione spaziale</strong> con <strong>migliaia di giocatori</strong> impegnati <strong>contemporaneamente</strong>, all'interno del medesimo universo, a competere fra di loro.. Tutto ciò che vi serve è uno standard browser web.",
    homeText2: "Registrati ora ed entra nel fantastico mondo di OGame!",
    homeButton: "Gioca ora, gratis!"
  },
  ru: {
    menuStart: "Главная",
    menuAbout: "Про ОГейм",
    menuPictures: "Картинки",
    menuRegister: "Присоединиться",
    loginLink: "Link Логин",
    loginName: "Имя",
    loginPass: "Пароль",
    loginChooseUniverse: "Вселенная...",
    loginUniverse: "Вселенная",
    loginConfirm: "Заходя в игру, я принимаю",
    loginImpressum: "Основные положения",
    loginRemind: "Забыли пароль?",
    loginNotChosen: "Вы не выбрали вселенную.",
    chooseLanguage: "Выберите свой язык",
    copyright: "Все права защищены.",
    downRules: "Правила",
    downImprint: "Impressum",
    downTerms: "Основные положения",
    homeTitle: "Добро пожаловать в ОГейм",
    homeText1:
      "<strong>ОГейм</strong> - это <strong>космическая стратегия</strong>. \n<strong>Тысячи игроков</strong> выступают <strong>одновременно</strong> против друг друга. Для игры Вам нужен всего лишь нормальный браузер.",
    homeText2: "Зарегистрируйтесь и откройте для себя фантастический мир ОГейм!",
    homeButton: "РЕГИСТРИРУЙТЕСЬ И ИГРАЙТЕ!"
  }
};

export function legacyPublicStyle(panelImage = "part_register.jpg"): React.CSSProperties {
  return {
    "--legacy-public-body-bg": `url("${publicImageBase}/sterne_bg2.jpg")`,
    "--legacy-public-main-bg": `url("${publicImageBase}/startseite_bg.jpg")`,
    "--legacy-public-login-bg": `url("${publicImageBase}/part_login2.jpg")`,
    "--legacy-public-panel-bg": `url("${publicImageBase}/${panelImage}")`,
    "--legacy-public-input-bg": `url("${publicImageBase}/eingabe_back.png")`,
    "--legacy-public-point-bg": `url("${publicImageBase}/point.png")`
  } as React.CSSProperties;
}

export function legacyPublicLanguage(): LegacyPublicLanguage {
  if (typeof document === "undefined") {
    return "en";
  }
  const cookieLanguage = document.cookie
    .split("; ")
    .find((cookie) => cookie.startsWith("ogamelang="))
    ?.split("=")[1];
  return isLegacyPublicLanguage(cookieLanguage) ? cookieLanguage : "en";
}

export function legacyPublicTexts(): LegacyPublicTexts {
  return legacyPublicTextsByLanguage[legacyPublicLanguage()];
}

function isLegacyPublicLanguage(value: string | undefined): value is LegacyPublicLanguage {
  return value === "de" || value === "en" || value === "fr" || value === "it" || value === "ru";
}

function escapeLegacyPublicHTML(value: string): string {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}

function legacyDownmenuHTML(texts: LegacyPublicTexts): string {
  return `\n        <a href="regeln.php">${escapeLegacyPublicHTML(texts.downRules)}</a>&nbsp;\n        <a target="_blank" href="impressum.php">${escapeLegacyPublicHTML(
    texts.downImprint
  )}</a>&nbsp;\n        <a target="_blank" href="#">${escapeLegacyPublicHTML(texts.downTerms)}</a>\n\n     `;
}

export function useLegacyPublicAutoFocus<T extends HTMLElement>(ref: React.RefObject<T | null>, enabled = true) {
  React.useEffect(() => {
    if (!enabled) {
      return undefined;
    }
    let cancelled = false;
    let focusFrame: number | undefined;
    let pollTimer: number | undefined;
    const startedAt = window.performance.now();

    const focus = () => {
      if (cancelled) {
        return;
      }
      focusFrame = window.requestAnimationFrame(() => {
        ref.current?.focus();
      });
    };

    const waitForLegacyCss = () => {
      if (cancelled) {
        return;
      }
      const links = Array.from(document.querySelectorAll<HTMLLinkElement>("link[data-legacy-public-css]"));
      const legacyCssReady =
        document.body.classList.contains("legacy-public-body") &&
        links.length >= 2 &&
        links.every((link) => link.sheet !== null);
      if (legacyCssReady || window.performance.now() - startedAt > 1000) {
        focus();
        return;
      }
      pollTimer = window.setTimeout(waitForLegacyCss, 16);
    };

    waitForLegacyCss();
    return () => {
      cancelled = true;
      if (focusFrame !== undefined) {
        window.cancelAnimationFrame(focusFrame);
      }
      if (pollTimer !== undefined) {
        window.clearTimeout(pollTimer);
      }
    };
  }, [enabled, ref]);
}

export function LegacyPublicHome({
  universes,
  loginDraft,
  loginResult,
  loginPending,
  loginError,
  onLoginChange,
  onLoginSubmit
}: LegacyPublicHomeProps) {
  const texts = legacyPublicTexts();
  return (
    <main className="legacy-public-page" style={legacyPublicStyle()}>
      <a className="legacy-public-skip" href="#pustekuchen">
        {texts.loginLink}
      </a>
      <div className="legacy-public-main" id="main">
        <LanguageLinks />
        <MainMenu active="home" withHomeCounterSpace />
        <HomeContent />
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

export function LanguageLinks() {
  const texts = legacyPublicTexts();
  const flags = [
    ["de", "Deutschland", "de.gif"],
    ["en", "English", "gb.gif"],
    ["fr", "France", "fr.gif"],
    ["it", "Italy", "it.gif"],
    ["ru", "Russia", "ru.gif"]
  ];

  return (
    <div {...legacyAlignRightProps} className="products legacy-public-products">
      {flags.map(([lang, label, file]) => (
        <React.Fragment key={lang}>
          <a
            href="#"
            onClick={(event) => {
              event.preventDefault();
              setLegacyLanguage(lang);
            }}
          >
            <img alt={label} src={`${publicImageBase}/flags/${file}`} title={label} />
          </a>{" "}
        </React.Fragment>
      ))}
      <a href="#">{texts.chooseLanguage}</a>
    </div>
  );
}

function setLegacyLanguage(lang: string) {
  const expires = new Date();
  expires.setTime(expires.getTime() + 9999 * 24 * 60 * 60 * 1000);
  document.cookie = `ogamelang=${lang}; expires=${expires.toUTCString()}; path=/`;
  window.location.reload();
}

export function MainMenu({ active, withHomeCounterSpace = false }: { active?: "home" | "about" | "preview" | "reg"; withHomeCounterSpace?: boolean }) {
  const texts = legacyPublicTexts();
  const items = [
    { key: "home", label: texts.menuStart, href: "home.php" },
    { key: "about", label: texts.menuAbout, href: "about.php" },
    { key: "preview", label: texts.menuPictures, href: "screenshots.php" },
    { key: "reg", label: texts.menuRegister, href: "register.php" }
  ] as const;

  return (
    <div className="legacy-public-mainmenu" id="mainmenu">
      {items.map((item) => (
        <React.Fragment key={item.key}>
          {item.key === active ? (
            <div className="menupoint legacy-public-menupoint">
              {item.label}
            </div>
          ) : (
            <a href={item.href}>
              {item.label}
            </a>
          )}{" "}
        </React.Fragment>
      ))}
      {withHomeCounterSpace ? (
        <>
          <br />
          <br />
        </>
      ) : null}
    </div>
  );
}

function HomeContent() {
  const texts = legacyPublicTexts();
  return (
    <section className="rightmenu legacy-public-rightmenu" id="rightmenu">
      <div className="legacy-public-title" id="title">{texts.homeTitle}</div>
      <div className="legacy-public-content" id="content">
        <div
          id="text1"
          dangerouslySetInnerHTML={{
            __html: texts.homeText1
          }}
        />
        <div
          className="bigbutton legacy-public-register-button"
          id="register"
          onClick={() => {
            window.history.pushState({}, "", "register.php");
            window.dispatchEvent(new PopStateEvent("popstate"));
          }}
        >
          {texts.homeButton}
        </div>
        <div className="legacy-public-text2" id="text2">{texts.homeText2}</div>
      </div>
    </section>
  );
}

export function LoginStrip({
  universes,
  loginDraft,
  loginResult,
  loginPending,
  loginError,
  onLoginChange,
  onLoginSubmit,
  autoFocusUniverse = true
}: LegacyPublicLoginProps & { autoFocusUniverse?: boolean }) {
  const universeRef = React.useRef<HTMLSelectElement>(null);
  const language = legacyPublicLanguage();
  const texts = legacyPublicTexts();

  useLegacyPublicAutoFocus(universeRef, autoFocusUniverse);

  const handlePasswordReminder = (event: React.MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault();
    if (!loginDraft.universe) {
      window.alert(texts.loginNotChosen);
      return;
    }
    const form = document.forms.namedItem("loginForm");
    if (form instanceof HTMLFormElement) {
      form.action = legacyPublicUniverseActionURL(loginDraft.universe, "/game/reg/mail.php");
      form.submit();
    }
  };

  return (
    <section className="legacy-public-login" id="login">
      <a id="pustekuchen"></a>
      <div className="legacy-public-login-labels" id="login_text_1">
        <div className="legacy-public-login-name">{texts.loginName}</div>
        {" "}
        <div className="legacy-public-login-pass">{texts.loginPass}</div>
      </div>
      <div className="legacy-public-login-input" id="login_input">
        <table cellPadding={0} cellSpacing={0}>
          <tbody>
            <tr style={{ verticalAlign: "top" }}>
              <td style={{ paddingRight: 4 }}>
                <form id="legacy-public-login-form" name="loginForm" onSubmit={onLoginSubmit}>
                  <input name="v" type="hidden" value="2" />
                  <span>
                    <select
                      className="eingabe legacy-public-input"
                      name="universe"
                      onChange={(event) => onLoginChange("universe", event.currentTarget.value)}
                      ref={universeRef}
                      style={{ width: 144 }}
                      tabIndex={1}
                      value={loginDraft.universe}
                    >
                      <option value="">{texts.loginChooseUniverse}</option>
                      {universes.map((universe) => (
                        <option key={universe.number} value={universe.baseUrl}>
                          {universe.number}. {texts.loginUniverse}
                        </option>
                      ))}
                    </select>
                  </span>
                </form>
              </td>
              <td style={{ paddingRight: 3 }}>
                <span>
                  <input
                    className="eingabe legacy-public-input legacy-public-login-field"
                    form="legacy-public-login-form"
                    maxLength={20}
                    name="login"
                    onChange={(event) => onLoginChange("login", event.currentTarget.value)}
                    style={{ top: 0, width: 111 }}
                    tabIndex={2}
                    value={loginDraft.login}
                  />
                </span>
              </td>
              <td>
                <span>
                  <input
                    className="eingabe legacy-public-input legacy-public-password-field"
                    form="legacy-public-login-form"
                    maxLength={20}
                    name="pass"
                    onChange={(event) => onLoginChange("pass", event.currentTarget.value)}
                    style={{ top: 0, width: 113 }}
                    tabIndex={3}
                    type="password"
                    value={loginDraft.pass}
                  />
                </span>
              </td>
              <td className="legacy-public-login-button-cell" style={{ paddingTop: 2 }}>
                <input
                  alt="Login"
                  className="loginButton legacy-public-login-button"
                  disabled={loginPending}
                  form="legacy-public-login-form"
                  name="button"
                  onMouseOut={(event) => {
                    event.currentTarget.src = `${publicImageBase}/login_button.jpg`;
                  }}
                  onMouseOver={(event) => {
                    event.currentTarget.src = `${publicImageBase}/login_button2.jpg`;
                  }}
                  src={`${publicImageBase}/login_button.jpg`}
                  type="image"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div className="legacy-public-login-links" id="login_text_2">
        <div className="legacy-public-remind">
          <a data-navigation-href="/game/reg/mail.php" href="#" onClick={handlePasswordReminder}>{texts.loginRemind}</a>
        </div>
        {" "}
        <div className="legacy-public-login-confirm">
          {language === "en" ? (
            <>By logging in, I accept the <a href="#" target="_blank">T&amp;C&apos;s</a>.</>
          ) : (
            <>{texts.loginConfirm} <a href="#" target="_blank">{texts.loginImpressum}</a>.</>
          )}
        </div>
      </div>
      <LoginFeedback loginError={loginError} loginResult={loginResult} />
      <div className="legacy-public-copyright" id="copyright">
        (C) 2007 by <a href="http://www.gameforge.de" target="_blank">Gameforge Productions GmbH</a>. {texts.copyright}&nbsp;&nbsp;
      </div>
      <div
        className="legacy-public-downmenu"
        id="downmenu"
        dangerouslySetInnerHTML={{
          __html: legacyDownmenuHTML(texts)
        }}
      />
    </section>
  );
}

export function legacyPublicUniverseActionURL(universe: string, actionPath: string, currentHref = window.location.href): string {
  try {
    const selectedUniverse = new URL(universe, currentHref);
    if (isLocalLegacyUniverseURL(selectedUniverse)) {
      return actionPath;
    }
    const url = new URL(selectedUniverse.toString());
    url.pathname = actionPath;
    url.search = "";
    url.hash = "";
    return url.toString();
  } catch {
    return actionPath;
  }
}

function isLocalLegacyUniverseURL(url: URL): boolean {
  return ["localhost", "127.0.0.1", "[::1]", "::1"].includes(url.hostname);
}

function LoginFeedback({ loginError, loginResult }: Pick<LegacyPublicHomeProps, "loginError" | "loginResult">) {
  if (loginError) {
    return <div className="legacy-public-login-feedback">{loginError}</div>;
  }
  if (!loginResult) {
    return null;
  }
  if (loginResult.valid) {
    return null;
  }
  return <div className="legacy-public-login-feedback">{loginResult.issues[0]?.message ?? "Login failed."}</div>;
}
