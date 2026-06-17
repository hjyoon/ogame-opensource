import React from "react";
import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
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
      <div className="legacy-public-main">
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
  return (
    <section className="legacy-public-register-panel">
      <div className="legacy-public-title">Registration</div>
      <div className="legacy-public-content">
        <div className="legacy-register-head">
          In order to play you only have to enter a <strong>username</strong>, a <strong>password</strong> and an{" "}
          <strong>E-Mail address</strong> and <strong>proceed to read the terms and conditions</strong> before
          activating the check box about your agreement to them.
        </div>
        <div className="legacy-register-container">
          <form id="legacy-register-form" name="registerForm" onSubmit={onRegistrationSubmit}>
            <table>
              <tbody>
                <tr>
                  <td className="legacy-register-label">Username:</td>
                  <td className="legacy-register-input-cell">
                    <input
                      autoFocus
                      className="legacy-public-input legacy-register-field"
                      name="character"
                      onChange={(event) => onRegistrationChange("character", event.currentTarget.value)}
                      size={20}
                      type="text"
                      value={registrationDraft.character}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="legacy-register-label">E-Mail-Address:</td>
                  <td className="legacy-register-input-cell">
                    <input
                      className="legacy-public-input legacy-register-field"
                      name="email"
                      onChange={(event) => onRegistrationChange("email", event.currentTarget.value)}
                      size={20}
                      type="text"
                      value={registrationDraft.email}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="legacy-register-label">Password:</td>
                  <td className="legacy-register-input-cell">
                    <input
                      className="legacy-public-input legacy-register-field"
                      name="password"
                      onChange={(event) => onRegistrationChange("password", event.currentTarget.value)}
                      size={20}
                      type="password"
                      value={registrationDraft.password}
                    />
                  </td>
                </tr>
                <tr>
                  <td className="legacy-register-uni-label">Universe:</td>
                  <td className="legacy-register-input-cell">
                    <select
                      className="legacy-public-input legacy-register-universe"
                      name="universe"
                      onChange={(event) => onRegistrationChange("universe", event.currentTarget.value)}
                      size={1}
                      value={registrationDraft.universe}
                    >
                      <option value="">Choose a universe...</option>
                      {universes.map((universe) => (
                        <UniverseOption key={universe.number} universe={universe} />
                      ))}
                    </select>
                    <div className="legacy-register-uni-info">
                      <a href="/universes">Specials of the universes</a>
                    </div>
                  </td>
                </tr>
                <tr className="legacy-register-agb-row">
                  <td />
                  <td className="legacy-register-agb-cell">
                    <input
                      checked={registrationDraft.agb}
                      name="agb"
                      onChange={(event) => onRegistrationChange("agb", event.currentTarget.checked)}
                      type="checkbox"
                    />{" "}
                    I accept the{" "}
                    <a className="legacy-register-agb" href="/legal">
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
        <RegistrationFeedback error={registrationError} result={registrationResult} />
        <button
          className="legacy-register-submit"
          disabled={registrationPending}
          form="legacy-register-form"
          type="submit"
        >
          {registrationPending ? "Creating..." : "Join now!"}
        </button>
      </div>
    </section>
  );
}

function UniverseOption({ universe }: { universe: PublicUniverse }) {
  const label = universe.number === 1 ? `${universe.number} (recommended)` : String(universe.number);
  return <option value={universe.baseUrl}>{label}</option>;
}

function RegistrationFeedback({
  error,
  result
}: {
  error: string | null;
  result: PublicRegistrationResult | null;
}) {
  if (error) {
    return <div className="legacy-register-status legacy-register-warning">{error}</div>;
  }
  if (!result) {
    return <div className="legacy-register-info" />;
  }
  if (result.valid) {
    return (
      <div className="legacy-register-status legacy-register-fine">
        Registration was successful.
        {result.session?.redirectTo ? (
          <>
            {" "}
            <a href={result.session.redirectTo}>Open overview</a>
          </>
        ) : null}
      </div>
    );
  }
  return (
    <div className="legacy-register-status legacy-register-warning">
      {result.issues.map((issue) => issue.message).join(" ")}
    </div>
  );
}
