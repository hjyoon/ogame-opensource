# Обзор

> [!WARNING]
> Старый PHP-код и необязательные Mod сохранены для совместимости и эталонного тестирования. Актуальный статус миграции указан ниже.

Это возрождённый OGame канонической версии 0.84 со старым дизайном.

## Миграция Go/React

Ветка `hjyoon/golang` использует React 19/Bun 1.3 и Go 1.25 с `net/http`. Go обслуживает production-сборку React. Код следует Clean Architecture, а PHP служит эталоном для сравнительных E2E.

Все обнаруженные и зарегистрированные функции базового продукта без Mod проходят итоговый QA. Это не абсолютное доказательство всех возможных состояний старого runtime. Четыре необязательных PHP Mod и 37 hooks исключены из области нового продукта.

```sh
OGAME_RUN_LEGACY_E2E=1 OGAME_GO_PORT=8890 OGAME_KEEP_GO_DOCKER=1 testing/e2e/run-golang-migration-qa.sh
```

Документы: [статус](./MIGRATION_STATUS.md), [API](./backend/API_ENDPOINTS.md), [MCP](./MCP.md), [QA](./testing/e2e/README.md).

## Установка

- [Обычная установка](/wiki/ru/install.md)
- [Установка Docker](/wiki/ru/install_docker.md)
- [Альтернативный Docker](https://gitlab.com/nolialsea/ogame-opensource-docker)
- [Discord](https://discord.gg/xpCV3McAj2)

PHP находится в `game`, новый frontend в `frontend`, backend в `backend`.

## Возможности

- оригинальная механика, стоимость, производство и время строительства
- быстрый боевой движок с правильным скорострелом
- Admin, Bot, статистика и много Universe
- очередь событий без обязательного CRON
- ACS, экспедиции, луны, ракеты и флот
- сообщения, альянсы, друзья, офицеры и Merchant
- русский, английский и немецкий языки

![screen1](/wiki/imgstore/screen1.jpg)
![screen2](/wiki/imgstore/screen2.jpg)

Проект некоммерческий. Торговые марки и защищённые материалы принадлежат Gameforge 4D GmbH. Премиум-функции бесплатны.

## Кредиты

Благодарим Александра Рёснера (Legor). Его аккаунт остаётся на планете Аракис в позиции \[1:1:2\].
