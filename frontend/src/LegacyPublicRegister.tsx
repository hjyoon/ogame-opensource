import React from "react";
import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  useLegacyPublicAutoFocus,
  type LegacyPublicLoginProps,
  type PublicUniverse
} from "./LegacyPublicHome";

export type PublicRegistrationIssue = {
  field: string;
  code: string;
  message: string;
};

export type PublicRegistrationResult = {
  valid: boolean;
  created?: boolean;
  issues: PublicRegistrationIssue[];
  session?: {
    redirectTo: string;
    universeNumber: number;
  };
};

export type PublicRegistrationDraft = {
  character: string;
  password: string;
  email: string;
  universe: string;
  agb: boolean;
};

type LegacyPublicRegisterProps = LegacyPublicLoginProps & {
  registrationDraft: PublicRegistrationDraft;
  registrationError: string | null;
  registrationPending: boolean;
  registrationResult: PublicRegistrationResult | null;
  onRegistrationChange: (field: keyof PublicRegistrationDraft, value: string | boolean) => void;
  onRegistrationSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
};

type RegisterHelpCode = "201" | "202" | "204" | "205";

type RegisterStatus = {
  className: "fine" | "warning";
  text: string;
};

export function LegacyPublicRegister({
  universes,
  loginDraft,
  loginResult,
  loginPending,
  loginError,
  onLoginChange,
  onLoginSubmit,
  registrationDraft,
  registrationError,
  registrationPending,
  registrationResult,
  onRegistrationChange,
  onRegistrationSubmit
}: LegacyPublicRegisterProps) {
  return (
    <main className="legacy-public-page" style={legacyPublicStyle("part_register2.jpg")}>
      <a className="legacy-public-skip" href="#pustekuchen">
        Link Login
      </a>
      <div className="legacy-public-main" id="main">
        <LanguageLinks />
        <MainMenu active="reg" />
        <RegisterContent
          onRegistrationChange={onRegistrationChange}
          onRegistrationSubmit={onRegistrationSubmit}
          registrationDraft={registrationDraft}
          registrationError={registrationError}
          registrationPending={registrationPending}
          registrationResult={registrationResult}
          universes={universes}
        />
        <LoginStrip
          autoFocusUniverse={false}
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

function RegisterContent({
  universes,
  registrationDraft,
  registrationError,
  registrationPending,
  registrationResult,
  onRegistrationChange,
  onRegistrationSubmit
}: Pick<
  LegacyPublicRegisterProps,
  | "universes"
  | "registrationDraft"
  | "registrationError"
  | "registrationPending"
  | "registrationResult"
  | "onRegistrationChange"
  | "onRegistrationSubmit"
>) {
  const characterRef = React.useRef<HTMLInputElement>(null);
  const formRef = React.useRef<HTMLFormElement>(null);
  const [helpCode, setHelpCode] = React.useState<RegisterHelpCode>("201");
  const [pollingUsername, setPollingUsername] = React.useState(false);
  const [registerStatus, setRegisterStatus] = React.useState<RegisterStatus | null>(null);

  useLegacyPublicAutoFocus(characterRef);
  React.useEffect(() => {
    if (!pollingUsername) {
      return;
    }
    const timer = window.setInterval(() => {
      const username = registrationDraft.character;
      if (username.length > 2 && username.length < 20) {
        setRegisterStatus({ className: "fine", text: "OK" });
      } else {
        setRegisterStatus({ className: "warning", text: "The name must be between 3 and 20 characters long!" });
      }
    }, 1000);
    return () => window.clearInterval(timer);
  }, [pollingUsername, registrationDraft.character]);

  const submitRegistration = () => {
    if (!registrationPending) {
      formRef.current?.requestSubmit();
    }
  };

  const submitRegistrationFromKeyboard = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      submitRegistration();
    }
  };

  return (
    <section className="rightmenu_register legacy-public-register-panel" id="rightmenu">
      <div className="legacy-public-title" id="title">Registration</div>
      <div className="legacy-public-content" id="content">
        <div
          className="legacy-register-head"
          id="text1"
          dangerouslySetInnerHTML={{
            __html:
              "In order to play you only have to enter a <strong>username</strong>, a <strong>password</strong> and an <strong>E-Mail address</strong> and <strong>proceed to read the terms and conditions</strong> before activating the check box about your agreement to them."
          }}
        />
        <div className="legacy-register-container" id="register_container">
          <form id="legacy-register-form" name="registerForm" onSubmit={onRegistrationSubmit} ref={formRef}>
            <table>
              <tbody>
                <tr>
                  <td className="table_lable legacy-register-label">Username:</td>
                  <td className="table_input legacy-register-input-cell">
                    <input
                      className="eingabe legacy-public-input legacy-register-field"
                      name="character"
                      onChange={(event) => onRegistrationChange("character", event.currentTarget.value)}
                      onBlur={() => setPollingUsername(false)}
                      onFocus={() => {
                        setHelpCode("201");
                        setRegisterStatus(null);
                        setPollingUsername(true);
                      }}
                      ref={characterRef}
                      size={20}
                      type="text"
                      value={registrationDraft.character}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="table_lable legacy-register-label">E-Mail-Address:</td>
                  <td className="table_input legacy-register-input-cell">
                    <input
                      className="eingabe legacy-public-input legacy-register-field"
                      name="email"
                      onChange={(event) => onRegistrationChange("email", event.currentTarget.value)}
                      onFocus={() => {
                        setHelpCode("202");
                        setRegisterStatus(null);
                      }}
                      size={20}
                      type="text"
                      value={registrationDraft.email}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="table_lable legacy-register-label">Password:</td>
                  <td className="table_input legacy-register-input-cell">
                    <input
                      className="eingabe legacy-public-input legacy-register-field"
                      name="password"
                      onChange={(event) => onRegistrationChange("password", event.currentTarget.value)}
                      onFocus={() => {
                        setHelpCode("205");
                        setRegisterStatus(null);
                      }}
                      size={20}
                      type="password"
                      value={registrationDraft.password}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="legacy-register-uni-label" id="uni_label">Universe:</td>
                  <td className="table_input legacy-register-input-cell">
                    <select
                      className="eingabe legacy-public-input legacy-register-universe"
                      name="universe"
                      onChange={(event) => onRegistrationChange("universe", event.currentTarget.value)}
                      size={1}
                      style={{ width: 122 }}
                      value={registrationDraft.universe}
                    >
                      <option value=""></option>
                      {universes.map((universe) => (
                        <UniverseOption key={universe.number} universe={universe} />
                      ))}
                    </select>
                    <div className="legacy-register-uni-info" id="uni_infos_link">
                      <a href="/universes">Specials of the universes</a>
                    </div>
                  </td>
                </tr>
                <tr className="legacy-register-agb-row" id="agb_zeile">
                  <td />
                  <td className="legacy-register-agb-cell" id="table_agb">
                    <input
                      checked={registrationDraft.agb}
                      name="agb"
                      onChange={(event) => onRegistrationChange("agb", event.currentTarget.checked)}
                      onFocus={() => {
                        setHelpCode("204");
                        setRegisterStatus(null);
                      }}
                      type="checkbox"
                    />{" "}
                    I accept the{" "}
                    <a className="register_agb legacy-register-agb" href="/legal">
                      T&amp;C&apos;s
                    </a>
                  </td>
                </tr>
              </tbody>
            </table>
            <input name="v" type="hidden" value="3" />
            <input name="step" type="hidden" value="validate" />
            <input name="try" type="hidden" value="2" />
            <input name="kid" type="hidden" value="" />
            <input name="lang" type="hidden" value="en" />
            <input name="errorCodeOn" type="hidden" value="1" />
          </form>
        </div>
        <RegistrationFeedback error={registrationError} helpCode={helpCode} result={registrationResult} status={registerStatus} />
        <div
          aria-disabled={registrationPending}
          id="register_submit"
          onClick={submitRegistration}
          onKeyDown={submitRegistrationFromKeyboard}
          role="button"
          tabIndex={0}
        >
          Join now!
        </div>
      </div>
    </section>
  );
}

function UniverseOption({ universe }: { universe: PublicUniverse }) {
  const label = universe.number === 3 ? `${universe.number} (recommended)` : String(universe.number);
  return <option value={universe.baseUrl}>{label}</option>;
}

function RegistrationFeedback({
  error,
  helpCode,
  result,
  status
}: {
  error: string | null;
  helpCode: RegisterHelpCode;
  result: PublicRegistrationResult | null;
  status: RegisterStatus | null;
}) {
  if (error) {
    return <div id="statustext"><span className="warning">{error}</span></div>;
  }
  if (!result) {
    return (
      <>
        <div id="infotext" dangerouslySetInnerHTML={{ __html: registerHelpHTML(helpCode) }} />
        <div id="statustext">{status ? <span className={status.className}>{status.text}</span> : null}</div>
      </>
    );
  }
  if (result.valid) {
    return (
      <div id="statustext">
        <span className="fine">
        Registration was successful.
        {result.session?.redirectTo ? (
          <>
            {" "}
            <a href={result.session.redirectTo}>Open overview</a>
          </>
        ) : null}
        </span>
      </div>
    );
  }
  return (
    <div id="statustext">
      <span className="warning">{result.issues.map((issue) => issue.message).join(" ")}</span>
    </div>
  );
}

function registerHelpHTML(code: RegisterHelpCode): string {
  switch (code) {
    case "202":
      return "E-Mail-Address: <br />Enter a valid E-Mail address to activate your account. You have 3 days to activate your account during those 3 days you are already able to play.";
    case "204":
      return "T&amp;C:<br /> Accept the T&amp;C (Terms and Conditions) to be able to play OGame.";
    case "205":
      return "Password:<br/>Your password works as a safety meassure when login in to your account. Do not give your password to anyone!";
    case "201":
    default:
      return "Name in the game: <br />This is the name you use in the game. It is unique throughout the universe.";
  }
}
