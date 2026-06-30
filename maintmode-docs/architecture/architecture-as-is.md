# Текущая архитектура MaintMode (as-is)

> Снимок состояния «как сейчас» — для сравнения с целевым модульным монолитом
> ([architecture.md](architecture.md)). Те же разрезы: §1 — слои × модули,
> §2 — граф пакетов и контрактов. Различия с целевым выделены в §3.

Сегодня MaintMode — это **один Go-модуль**, из которого собираются **два
бинаря**: `cmd/maintmode` (core + notify) и `cmd/auth`. Оба ходят в **одну
общую Postgres-БД**; core вызывает auth **по S2S через `internal/gateways/auth`**
(HTTP в пределах одного хоста), а не в памяти. Notify-домен (targets + доставка)
живёт **внутри core-бинаря**.

Сразу важная оговорка: **архитектура аккуратная и здоровая** — слои чистые,
модули разведены, контракты узкие. На этих диаграммах нет «грязи», которую надо
чинить. Единственное, что целевой дизайн ([architecture.md](architecture.md))
меняет, — снимает **сетевую границу core↔auth** (красный `gateways/auth` +
пунктир S2S ниже). Всё остальное переезжает один-в-один; поэтому as-is и целевой
почти неотличимы.

---

## 1. Слои × модули (as-is)

Та же сетка слоёв и модулей, что в целевом доке, но с двумя отличиями, видимыми
прямо на схеме: **граница процесса** проходит между core и auth, и core→auth
идёт через `gateways/auth` (S2S), а не прямым вызовом сервиса.

```mermaid
flowchart TB
    classDef layer fill:#f5f5f5,stroke:#bbb,color:#333;
    classDef mod fill:#fff,stroke:#888,color:#111;
    classDef gw fill:#ffecec,stroke:#c0392b,color:#5a1a14;

    subgraph PROC_M["процесс cmd/maintmode (core + notify)"]
        subgraph L_api_m["слой API"]
            direction LR
            a_core["maint, resources,<br/>calendar, userpicker"]:::mod
            a_notif["notifytargets"]:::mod
        end
        subgraph L_proc_m["слой async"]
            direction LR
            p_core["maint.auto.cancel,<br/>maint.reminder"]:::mod
            p_notif["messaging.send<br/>(asyncsender)"]:::mod
        end
        subgraph L_svc_m["слой service"]
            direction LR
            s_core["maint, resources,<br/>conflicts, calendar,<br/>userpicker, usersummary"]:::mod
            s_notif["notifytargets, messaging,<br/>maintnotify,<br/>deferrednotifications"]:::mod
        end
        subgraph L_store_m["слой storage / gateway"]
            direction LR
            st_core["maintenances, resources,<br/>conflicts, conflict_snapshots"]:::mod
            st_notif["notifychannel, notifytargets,<br/>deferrednotifications,<br/>gateways/notifytransport"]:::mod
            gw_auth["gateways/auth<br/>(S2S-клиент к auth)"]:::gw
        end
    end

    subgraph PROC_A["процесс cmd/auth"]
        subgraph L_api_a["слой API"]
            direction LR
            a_auth["auth, users, roles,<br/>invitations, audit"]:::mod
        end
        subgraph L_proc_a["слой async"]
            direction LR
            p_auth["invitation.email,<br/>audit.write, audit.prune"]:::mod
        end
        subgraph L_svc_a["слой service"]
            direction LR
            s_auth["user, token, authz, jwtverifier,<br/>invitation, oauthprovider,<br/>auditor, auditpublisher"]:::mod
        end
        subgraph L_store_a["слой storage"]
            direction LR
            st_auth["users, useridentities, refreshtoken,<br/>blacklisttoken, userinvitations,<br/>distributedlock, audit"]:::mod
        end
    end

    db[("одна общая Postgres-БД")]
    redis[("Redis")]

    a_core --> s_core
    a_notif --> s_notif
    p_core --> s_core
    p_notif --> s_notif
    a_auth --> s_auth
    p_auth --> s_auth

    s_core --> st_core
    s_notif --> st_notif
    s_auth --> st_auth

    %% core -> auth НЕ напрямую, а через S2S-шлюз и сеть
    s_core --> gw_auth
    gw_auth -.S2S HTTP.-> a_auth

    st_core --> db
    st_notif --> db
    st_auth --> db
    st_auth --> redis

    class L_api_m,L_proc_m,L_svc_m,L_store_m,L_api_a,L_proc_a,L_svc_a,L_store_a layer;
```

Что важно на этой схеме:

- **Две рамки процессов** (`cmd/maintmode`, `cmd/auth`) — физическая граница
  деплоя. В целевом монолите она исчезает (один процесс).
- **`gateways/auth` (красный) + пунктир `S2S HTTP`** — core не зовёт auth-сервис
  напрямую, а гоняет HTTP-запрос по сети через S2S-шлюз. Это и есть та обёртка,
  которая в монолите снимается.
- **notify в core-процессе** — `notifytargets`/`messaging`/`maintnotify`/
  `deferrednotifications` собраны в бинаре `cmd/maintmode`, не отдельно.
- **`audit` в auth-процессе** — стор `audit` и дренаж `audit.write`/`audit.prune`
  живут в auth-бинаре (исторический артефакт RUK-179).
- **Одна общая БД** — все сторы обоих процессов пишут в неё; границы данных нет.

---

## 2. Граф пакетов и контрактов (as-is)

Те же межмодульные контракты, что в целевом доке (§2.2 [architecture.md](architecture.md)),
но core→auth-рёбра реализованы **через `gateways/auth` по S2S**, а не локальным
вызовом. Сам набор интерфейсов идентичен — меняется только то, чем они
удовлетворяются.

```mermaid
flowchart LR
    classDef core fill:#eef6ff,stroke:#4a78b5,color:#11304f;
    classDef auth fill:#fff3e6,stroke:#c07a2b,color:#5a3410;
    classDef notif fill:#eaf7ee,stroke:#3f9d5a,color:#14502a;
    classDef audit fill:#f3eef9,stroke:#7a52a8,color:#34205a;
    classDef gw fill:#ffecec,stroke:#c0392b,color:#5a1a14;

    subgraph PM["процесс cmd/maintmode"]
        subgraph CORE["core"]
            maint["maint.Service"]:::core
            usersummary["usersummary.Service"]:::core
            userpicker["userpicker.Service"]:::core
        end
        subgraph NOTIF["notificator (внутри core-процесса)"]
            maintnotify["maintnotify.Service"]:::notif
            messaging["messaging/sender.Service"]:::notif
        end
        gwauth["gateways/auth<br/>S2S-клиент"]:::gw
    end

    subgraph PA["процесс cmd/auth"]
        subgraph AUTH["auth"]
            user["user.Service"]:::auth
            token["token.Service"]:::auth
            authz["authz.CasbinAuthorizer"]:::auth
            jwtverifier["jwtverifier.Service"]:::auth
        end
        subgraph AUDIT["audit (внутри auth-процесса)"]
            auditpublisher["auditpublisher.Publisher"]:::audit
        end
    end

    middleware["server/middlewares"]:::core

    %% внутри процесса core: notify и audit-publish — прямые вызовы
    maint -->|"notifier (maintnotify.Service)"| maintnotify
    maintnotify -->|"#lt;#lt;I#gt;#gt; MessageSender"| messaging
    maint -->|"#lt;#lt;I#gt;#gt; AuditPublisher"| auditpublisher

    %% core -> auth: ВСЁ через S2S-шлюз и сеть
    maint -->|"#lt;#lt;I#gt;#gt; ApproverValidator"| gwauth
    usersummary -->|"#lt;#lt;I#gt;#gt; AuthUsersGateway"| gwauth
    userpicker -->|"#lt;#lt;I#gt;#gt; ActiveUsersLister"| gwauth
    middleware -->|"#lt;#lt;I#gt;#gt; ActiveTokenIntrospector"| gwauth
    gwauth -.S2S HTTP.-> user

    %% middleware: верификация токена и RBAC — уже локальны в core-процессе
    middleware -->|"#lt;#lt;I#gt;#gt; TokenVerifier"| jwtverifier
    middleware -->|"#lt;#lt;I#gt;#gt; Authorizer"| authz
```

На графе видно ключевое: **четыре core→auth-контракта** (`ApproverValidator`,
`AuthUsersGateway`, `ActiveUsersLister`, `ActiveTokenIntrospector`) сейчас идут
через `gateways/auth` и сеть, а `TokenVerifier`/`Authorizer` уже локальны
(`jwtverifier`/`authz` собраны в core-процесс). `audit.write` пересекает границу
процессов через **общую БД и общий `goque_task`** (атомарность outbox держится
только потому, что БД одна).

---

## 3. Чем as-is отличается от целевого (architecture.md)

| Аспект | as-is (этот док) | целевой монолит (architecture.md) |
| --- | --- | --- |
| Процессов | два (`cmd/maintmode`, `cmd/auth`) | один |
| core → auth | через `gateways/auth` по S2S HTTP | прямой вызов сервиса в памяти |
| `gateways/auth` | есть (S2S-клиент) | удалён |
| notify | в core-процессе | модуль в общем процессе (без изменений места) |
| audit | в auth-процессе | модуль в общем процессе |
| БД | одна общая | одна общая (без изменений) |
| `goque_task` | один общий, дренят два бинаря | один общий, дренит один процесс |

Главное: **переход меняет немного** — снимается `gateways/auth` и вторая рамка
процесса, четыре контракта переключаются с S2S на вызов в памяти. Сами
интерфейсы, сторы, БД и набор пакетов остаются те же. Это и есть аргумент
«откат дешёвый» из [architecture.md](architecture.md) §1.
