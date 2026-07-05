import React from "react";

type LegacyRegistrationFormProps = {
  universeNumber?: number;
};

export function LegacyRegistrationForm({ universeNumber = 1 }: LegacyRegistrationFormProps) {
  const headingRef = React.useRef<HTMLHeadingElement>(null);
  const [infoText, setInfoText] = React.useState("");
  const [statusText, setStatusText] = React.useState<{ className: "fine" | "warning"; text: string } | null>(null);

  React.useLayoutEffect(() => {
    headingRef.current?.setAttribute("style", "font-size: 22;");
  }, []);

  const showInfo = (code: "201" | "202" | "204") => {
    setStatusText(null);
    if (code === "201") {
      setInfoText("Name in Game: This is the name of your character in the game. No two names can be the same in the same universe.");
      return;
    }
    if (code === "202") {
      setInfoText("Email: Your password will be sent to this address. If you enter a wrong or invalid address, you will not be able to play.");
      return;
    }
    setInfoText("In order to start the game you must agree to the Basic Regulations.");
  };

  const pollUsername = (event: React.FocusEvent<HTMLInputElement>) => {
    showInfo("201");
    const username = event.currentTarget.value;
    if (username.length > 2 && username.length < 20) {
      setStatusText({ className: "fine", text: "OK" });
    } else if (username.length > 0) {
      setStatusText({ className: "warning", text: "Name must be between 3 and 20 characters long!" });
    }
  };

  return (
    <>
      <div id="overDiv" style={{ position: "absolute", visibility: "hidden", zIndex: 1000 }}></div>
      <center>
        <h1 ref={headingRef}>{`OGame Universe ${universeNumber} Registration`}</h1>

        <form id="registration" method="POST">
          <table width="700">
            <tbody>
              <tr>
                <td>
                  <table width="380">
                    <tbody>
                      <tr>
                        <td className="c" colSpan={2}>
                          Player information
                        </td>
                      </tr>
                      <tr>
                        <th className="">In-game name</th>
                        <th>
                          <input
                            name="character"
                            onBlur={() => undefined}
                            onChange={(event) => {
                              const username = event.currentTarget.value;
                              if (username.length > 2 && username.length < 20) {
                                setStatusText({ className: "fine", text: "OK" });
                              } else if (username.length > 0) {
                                setStatusText({ className: "warning", text: "Name must be between 3 and 20 characters long!" });
                              }
                            }}
                            onFocus={pollUsername}
                            size={20}
                            defaultValue=""
                          />
                        </th>
                      </tr>
                      <tr>
                        <th className="">Email</th>
                        <th>
                          <input name="email" onBlur={() => undefined} onFocus={() => showInfo("202")} size={20} defaultValue="" />
                        </th>
                      </tr>
                      <tr>
                        <th className="">
                          I agree with <a href="#" target="_blank">Basic Regulations</a>
                        </th>
                        <th>
                          <input name="agb" onFocus={() => showInfo("204")} type="checkbox" />
                        </th>
                      </tr>
                      <tr>
                        <th colSpan={2} style={{ textAlign: "center" }}>
                          <input type="submit" value="Sign up" />
                        </th>
                      </tr>
                    </tbody>
                  </table>
                  <input name="v" type="hidden" value="3" />
                  <input name="step" type="hidden" value="validate" />
                  <input name="try" type="hidden" value="2" />
                  <input name="kid" type="hidden" value="" />
                </td>
                <td>
                  <table width="320">
                    <tbody>
                      <tr>
                        <td className="c">Info</td>
                      </tr>
                      <tr style={{ height: 93 }}>
                        <th>
                          <p />
                          <div id="infotext">{infoText}</div>
                          <p />
                          <div id="statustext">{statusText ? <span className={statusText.className}>{statusText.text}</span> : null}</div>
                          <div id="debug"></div>
                        </th>
                      </tr>
                    </tbody>
                  </table>
                </td>
              </tr>
            </tbody>
          </table>
        </form>
      </center>
    </>
  );
}
