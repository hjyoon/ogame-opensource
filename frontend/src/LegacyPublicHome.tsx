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
  return (
    <main className="legacy-public-page" style={legacyPublicStyle()}>
      <a className="legacy-public-skip" href="#pustekuchen">
        Link Login
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
      <a href="#">Choose your language</a>
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
  const items = [
    { key: "home", label: "Start", href: "home.php" },
    { key: "about", label: "About OGame", href: "about.php" },
    { key: "preview", label: "Pictures", href: "screenshots.php" },
    { key: "reg", label: "Join Now!", href: "register.php" }
  ] as const;

  return (
    <div className="legacy-public-mainmenu" id="mainmenu">
      {items.map((item) =>
        item.key === active ? (
          <div className="menupoint legacy-public-menupoint" key={item.key}>
            {item.label}
          </div>
        ) : (
          <a href={item.href} key={item.key}>
            {item.label}
          </a>
        )
      )}
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
  return (
    <section className="rightmenu legacy-public-rightmenu" id="rightmenu">
      <div className="legacy-public-title" id="title">Welcome to OGame</div>
      <div className="legacy-public-content" id="content">
        <div
          id="text1"
          dangerouslySetInnerHTML={{
            __html:
              "<strong>OGame</strong> is a <strong>strategic space simulation game</strong>with \n<strong>thousands of players</strong> across the world competing with each other <strong>simultaneously</strong>. All you need to play is a standard web browser."
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
          Play for free now!
        </div>
        <div className="legacy-public-text2" id="text2">Register now and enter the fantastic world of OGame!</div>
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

  useLegacyPublicAutoFocus(universeRef, autoFocusUniverse);

  return (
    <section className="legacy-public-login" id="login">
      <a id="pustekuchen"></a>
      <div className="legacy-public-login-labels" id="login_text_1">
        <div className="legacy-public-login-name">Username</div>
        <div className="legacy-public-login-pass">Password</div>
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
                      <option value="">Choose a universe...</option>
                      {universes.map((universe) => (
                        <option key={universe.number} value={universe.baseUrl}>
                          {universe.number}. Universe
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
          <a href="#">Forgot your password?</a>
        </div>
        <div className="legacy-public-login-confirm">
          By logging in, I accept the <a href="#" target="_blank">T&amp;C&apos;s</a>.
        </div>
      </div>
      <LoginFeedback loginError={loginError} loginResult={loginResult} />
      <div className="legacy-public-copyright" id="copyright">
        (C) 2007 by <a href="http://www.gameforge.de" target="_blank">Gameforge Productions GmbH</a>. All rights reserved.&nbsp;&nbsp;
      </div>
      <div
        className="legacy-public-downmenu"
        id="downmenu"
        dangerouslySetInnerHTML={{
          __html:
            '\n        <a href="regeln.php">Rules</a>&nbsp;\n        <a target="_blank" href="impressum.php">Imprint</a>&nbsp;\n        <a target="_blank" href="#">T&amp;C\'s</a>\n\n     '
        }}
      />
    </section>
  );
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
