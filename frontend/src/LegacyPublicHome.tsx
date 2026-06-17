import React from "react";

export type PublicUniverse = {
  number: number;
  name: string;
  baseUrl: string;
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
      <div className="legacy-public-main">
        <LanguageLinks />
        <MainMenu active="home" />
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
    <div className="legacy-public-products">
      {flags.map(([lang, label, file]) => (
        <a href="/home" key={lang}>
          <img alt={label} src={`${publicImageBase}/flags/${file}`} title={label} />
        </a>
      ))}
      <a href="/home">Choose your language</a>
    </div>
  );
}

export function MainMenu({ active }: { active?: "home" | "about" | "preview" | "reg" }) {
  const items = [
    { key: "home", label: "Start", href: "/home" },
    { key: "about", label: "About OGame", href: "/about" },
    { key: "preview", label: "Pictures", href: "/screenshots" },
    { key: "reg", label: "Join Now!", href: "/register" }
  ] as const;

  return (
    <nav className="legacy-public-mainmenu" aria-label="Main menu">
      {items.map((item) =>
        item.key === active ? (
          <div className="legacy-public-menupoint" key={item.key}>
            {item.label}
          </div>
        ) : (
          <a href={item.href} key={item.key}>
            {item.label}
          </a>
        )
      )}
    </nav>
  );
}

function HomeContent() {
  return (
    <section className="legacy-public-rightmenu">
      <div className="legacy-public-title">Welcome to OGame</div>
      <div className="legacy-public-content">
        <div>
          <strong>OGame</strong> is a <strong>strategic space simulation game</strong> with{" "}
          <strong>thousands of players</strong> across the world competing with each other{" "}
          <strong>simultaneously</strong>. All you need to play is a standard web browser.
        </div>
        <a className="legacy-public-register-button" href="/register">
          Play for free now!
        </a>
        <div className="legacy-public-text2">Register now and enter the fantastic world of OGame!</div>
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
  onLoginSubmit
}: LegacyPublicLoginProps) {
  return (
    <section className="legacy-public-login" id="pustekuchen">
      <div className="legacy-public-login-labels">
        <div className="legacy-public-login-name">Username</div>
        <div className="legacy-public-login-pass">Password</div>
      </div>
      <form className="legacy-public-login-input" name="loginForm" onSubmit={onLoginSubmit}>
        <table>
          <tbody>
            <tr>
              <td>
                <select
                  className="legacy-public-input"
                  name="universe"
                  onChange={(event) => onLoginChange("universe", event.currentTarget.value)}
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
              </td>
              <td>
                <input
                  className="legacy-public-input legacy-public-login-field"
                  maxLength={20}
                  name="login"
                  onChange={(event) => onLoginChange("login", event.currentTarget.value)}
                  tabIndex={2}
                  value={loginDraft.login}
                />
              </td>
              <td>
                <input
                  className="legacy-public-input legacy-public-password-field"
                  maxLength={20}
                  name="pass"
                  onChange={(event) => onLoginChange("pass", event.currentTarget.value)}
                  tabIndex={3}
                  type="password"
                  value={loginDraft.pass}
                />
              </td>
              <td className="legacy-public-login-button-cell">
                <input
                  alt="Login"
                  className="legacy-public-login-button"
                  disabled={loginPending}
                  name="button"
                  src={`${publicImageBase}/login_button.jpg`}
                  type="image"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </form>
      <div className="legacy-public-login-links">
        <div className="legacy-public-login-confirm">
          By logging in, I accept the <a href="/legal">T&amp;C&apos;s</a>.
        </div>
        <div className="legacy-public-remind">
          <a href="/home">Forgot your password?</a>
        </div>
      </div>
      <LoginFeedback loginError={loginError} loginResult={loginResult} />
      <div className="legacy-public-copyright">
        (C) 2007 by <a href="http://www.gameforge.de">Gameforge Productions GmbH</a>. All rights reserved.
      </div>
      <div className="legacy-public-downmenu">
        <a href="/rules">Rules</a>&nbsp;
        <a href="/legal">Imprint</a>&nbsp;
        <a href="/legal">T&amp;C&apos;s</a>
      </div>
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
    return (
      <div className="legacy-public-login-feedback">
        <a href={loginResult.session?.redirectTo ?? "/game/overview"}>Open overview</a>
      </div>
    );
  }
  return <div className="legacy-public-login-feedback">{loginResult.issues[0]?.message ?? "Login failed."}</div>;
}
