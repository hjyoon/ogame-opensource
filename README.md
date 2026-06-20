# Introduction

> [!WARNING]
> We're currently undergoing a massive refactoring based on PHPStan analysis results, and a [modification engine](/wiki/en/mods.md) is being added. Therefore, some bugs and syntax errors may be present, but they will be fixed as we go.

This is revived OGame v 0.84 with old design.

## Go/React migration

The `hjyoon/golang` branch starts a staged port to React 19 on Bun 1.3 and Go 1.25 with the native `net/http` module. PHP stays the oracle; keep only the latest Go `goapp` container.

New migration code must follow Clean Architecture. Do not copy PHP file structure or `.php` routes one-for-one; implement natural internal React and Go modules. Visible page composition, skin, density, and assets must match legacy PHP screens unless a documented compatibility exception says otherwise. Game mechanics must remain strictly equivalent to the legacy engine. Go serves the React production build; Bun is only the frontend build tool.

New code lives in:

- `backend/`: Go server, health API, static React serving, legacy asset serving.
- `frontend/`: React migration shell built by Bun.
- `testing/e2e/run-golang-migration-qa.sh`: migration QA entrypoint using the existing Docker E2E suite plus Go/React smoke checks.

Local migration smoke:

```sh
cd frontend && bun install && bun run build
cd ../backend && go test ./...
docker compose -f compose.golang.yaml up -d --build goapp
```

Full compatibility QA still starts with:

```sh
testing/e2e/run-docker-e2e.sh
```

Markdown files are capped at 4KB. Split larger docs by topic and link them from a short index.

Current migration progress and remaining work are tracked in [MIGRATION_STATUS.md](./MIGRATION_STATUS.md).

Need help with installation? You have the following options:
- Use the millennial guide: [install](/wiki/en/install.md)
- Use the zoomer guide: [install_docker](/wiki/en/install_docker.md)
- There is also another deployment option with Docker from Noli: https://gitlab.com/nolialsea/ogame-opensource-docker
- Ask the community for help. Discord: https://discord.gg/xpCV3McAj2

:warning: Fellow developers! Don't be confused if you see a lot of crap in the root of the repository. You probably don't need all of this; it's just spare parts for Docker, PHPStan, and PHPUnit. The main source files are in the `game` folder.

**!!! All trademarks and copyrighted materials are belongs to OGame respective owners - Gameforge 4D GmbH !!!**

_Thank you for the great game, but redesign is not we like_

![whc50b7bd1f6b2a2](/wiki/imgstore/whc50b7bd1f6b2a2.jpg)

Currently only Russian, English and Deutch languages are supported. Other language packs can be submitted by volunteers. The game engine is multilingual.

Features:
- Original game mechanics!
- Well tested fast battle engine with fair rapidfire, written on C language (there is also a backup in PHP)
- Improved admin tool
- Integrated Galaxy-tool
- CRON-less event queue (but there is the option of using CRON in addition if you want to)
- Multi-language support
- ACS
- Planet temperature, planet images and sizes are same as in original game
- 100% match on resource costs and production timings
- Original home planet distribution algorithm and spy protection
- Fixed some original game bugs ("buggy" 10th planets, recalled ACS delay, buggy fleet return activity etc)
- Original expedition with triple Dark Matter chance
- Open-source!
- And many, many more!

![screen1](/wiki/imgstore/screen1.jpg)
![screen2](/wiki/imgstore/screen2.jpg)
![screen5](/wiki/imgstore/screen5.jpg)

**This is non commercial project, all Premium functions of original OGame (Dark Matter, Officeers and Trader) are free.**

All copyrighted material is proprietary Gameforge stuff. We do not make money on it! We just have fun =)

## Credits

Credits go to Alexander Rösner (Legor) for such revolutionary breakthrough in browser games.
He was not first, but he was the one, who was successful.
To pay respect, we still have Legor's account, sitting on its own planet Arakis at \[1:1:2\] =)
