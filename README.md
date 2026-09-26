# 3x-ui Tunnels

Лёгкий форк [3x-ui](https://github.com/MHSanaei/3x-ui) для дополнительных туннельных конфигураций без переноса чужих панелей внутрь 3x-ui.

## Возможности

- создание ссылок `openflux://`, `wdtt://` и `csqtt://`;
- добавление ссылок клиенту: **Clients → Links → Create tunnel link**;
- выдача в подписке, копирование и QR-коды;
- штатные MTProto и AmneziaWG (AWG);
- обновление к выбранному релизу 3x-ui через проверяемый Pull Request;
- откат к релизу форка с резервной копией базы.

OpenFlux, WDTT-Plus и CSQTT работают как внешние sidecar-сервисы. Панель создаёт и раздаёт клиентскую конфигурацию; автоматическое создание серверного аккаунта в sidecar пока не реализовано.

## Установка

```bash
bash <(curl -Ls https://raw.githubusercontent.com/vnenapravo7-source/3x-ui-tunnels/main/install.sh)
```

Конкретный релиз:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/vnenapravo7-source/3x-ui-tunnels/main/install.sh) fork-v3.8.5-1
```

## Обновление и откат

```bash
x-ui update
x-ui rollback fork-v3.8.5-1
```

Перед откатом база сохраняется в `/etc/x-ui/rollback/`.

Обновление основы проекта: **Actions → Upstream update or rollback → Run workflow → update**, затем укажите тег 3x-ui, например `v3.8.5`. Изменения приходят отдельным PR без импорта тысяч upstream-коммитов и тегов.

Форматы ссылок и ограничения: [docs/extra-tunnels.md](docs/extra-tunnels.md).

> Используйте проект законно. Ссылки содержат ключи и пароли — относитесь к ним как к секретам.
