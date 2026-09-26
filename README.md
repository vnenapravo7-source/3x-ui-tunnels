# 3x-ui Tunnels

Лёгкий форк [3x-ui](https://github.com/MHSanaei/3x-ui): штатные VLESS, MTProto и AmneziaWG плюс управляемые OpenFlux, WDTT-Plus и CSQTT.

## Установка

```bash
bash <(curl -Ls https://raw.githubusercontent.com/vnenapravo7-source/3x-ui-tunnels/main/install.sh)
x-ui tunnels install all
```

Вторая команда ставит или обновляет серверные компоненты OpenFlux, WDTT и CSQTT. После неё создайте нужный протокол в **Входящие**, добавьте клиента и возьмите ссылку или QR-код из подписки. Для OpenFlux ключ создаётся на 32 байта, доступны до 8 транспортов, а Cups.online сам создаёт комнату и добавляет её код в ссылку.

## Обновление форка

```bash
XUI_UPDATE_TAG=fork-v3.8.5-2 bash <(curl -Ls https://raw.githubusercontent.com/vnenapravo7-source/3x-ui-tunnels/main/update.sh)
x-ui tunnels update all
```

Обычная команда меню также остаётся доступна: `x-ui update`.

## Откат

```bash
x-ui rollback fork-v3.8.5-1
```

Перед откатом база сохраняется в `/etc/x-ui/rollback/`. Обновление основы 3x-ui запускается через **Actions → Upstream update or rollback** и приходит отдельным PR без импорта истории и тегов upstream.

Подробности и ограничения: [docs/extra-tunnels.md](docs/extra-tunnels.md). CSQTT разрешён только для некоммерческого использования без отдельной лицензии автора.
