import type { ReactNode } from "react";
import {
  LanguageLinks,
  LoginStrip,
  MainMenu,
  legacyPublicStyle,
  type LegacyPublicLoginProps
} from "./LegacyPublicHome";

export function LegacyPublicRules({
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
        <MainMenu />
        <RulesContent />
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

function RulesContent() {
  return (
    <section className="legacy-public-rules-panel">
      <div className="legacy-public-title">Правила</div>
      <div className="legacy-public-content">
        <div className="legacy-public-scroll legacy-rules-scroll">
          <p>Правила предназначены для того, чтобы обеспечить всем игрокам честную игру и доставить им удовольствие от игры.</p>
          <p>За соблюдением правил следят игровые Операторы.</p>

          <RuleSection title="1. Аккаунты">
            <p>
              Владельцем аккаунта является владелец почтового адреса, указанного в настройках игры. Пользоваться
              аккаунтом может только один игрок, но временно разрешается присматривать за чужим аккаунтом.
            </p>
            <p>
              Менять владельца аккаунта можно не чаще одного раза в месяц. После принятия чужого аккаунта новый владелец
              обязан сменить почтовый адрес в течение 12 часов.
            </p>
          </RuleSection>

          <RuleSection title="2. Мультиаккаунты">
            <p>В одной вселенной разрешено иметь только один аккаунт.</p>
            <p>
              Настоятельно рекомендуется уведомить Оператора, если два или более аккаунта используются в одной сети
              (например из школы, института или интернет-кафе).
            </p>
            <p>При нарушении: бессрочная блокировка всех аккаунтов без РО.</p>
          </RuleSection>

          <RuleSection title="3. Присмотр за Аккаунтом (Ситтинг)">
            <p>Ситтинг означает присмотр игрока за чужим аккаунтом.</p>
            <ul>
              <li>Присмотр разрешен на период не более 12 часов.</li>
              <li>Период досрочно прерывается, если на аккаунт заходит его владелец или другой игрок.</li>
              <li>Игрок может тратить ресурсы только на постройки и исследования.</li>
              <li>Во время присмотра запрещены боевые действия, ракетные атаки, фаланга и ворота.</li>
              <li>Во время ситтинга разрешено включать Режим Отпуска, но запрещено отключать его.</li>
            </ul>
            <p className="legacy-rules-warning">Программная реализация Ситтинга пока находится в стадии разработки.</p>
          </RuleSection>

          <RuleSection title="4. Башинг">
            <p>
              Запрещено атаковать одну планету или луну более 6 раз за период 24 часа. Атаки шпионскими зондами,
              Межпланетными ракетами и задание Уничтожить не учитываются.
            </p>
            <p>Наказание за башинг - блокировка атак на три дня.</p>
            <p className="legacy-rules-note">В третьей вселенной ограничение на количество атак отсутствует.</p>
          </RuleSection>

          <RuleSection title="5. Прокачка (Пушинг)">
            <p>Нижестоящему в рейтинге игроку запрещено передавать ресурсы игроку, стоящему выше в рейтинге.</p>
            <p>К исключениям относятся помощь переработчиками, дележ САБ, создание луны, торговля и займы с уведомлением Оператора.</p>
            <p>
              Наказание за прокачку: 1-й случай - 3 дня с РО, 2-й случай - 7 дней с РО, 3-й случай - бессрочный блок
              без РО.
            </p>
          </RuleSection>

          <RuleSection title="6. Использование Багов и Скриптов">
            <p>
              Использование ошибок игры для получения прибыли, сокрытие ошибок от администрации, сторонние клиенты и
              автоматические скрипты запрещены.
            </p>
            <p>Обо всех найденных ошибках игроки должны сообщать на форум, в раздел Ошибки.</p>
          </RuleSection>

          <RuleSection title="7. Угрозы в реальной жизни, оскорбления и спам">
            <p>
              Не запрещается употребление матерных слов и угроз в реале, но администрация не несет ответственности за
              сохранность игроков. "За маму" бессрочный бан аккаунта без РО.
            </p>
          </RuleSection>

          <RuleSection title="8. Амнистия">
            <p>
              Администрация сохраняет за собой право амнистирования бессрочно заблокированных аккаунтов, приуроченную к
              определенным событиям.
            </p>
          </RuleSection>
        </div>
      </div>
    </section>
  );
}

function RuleSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="legacy-rules-section">
      <h2>{title}</h2>
      {children}
    </section>
  );
}
