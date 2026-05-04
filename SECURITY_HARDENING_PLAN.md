# Security & Reliability Hardening Plan

| Поле | Значение |
|---|---|
| Версия документа | 1.0 |
| Дата создания | 2026-05-01 |
| Статус | Proposed (требует утверждения перед стартом реализации) |
| Автор плана | Security/Reliability Review |
| Ответственный (backend) | _<TBD: backend lead — `midas_exchange_back`>_ |
| Ответственный (frontend) | _<TBD: frontend lead — `midas-exchange-frontend`>_ |
| Получатели плана | gtrondin@gmail.com, владелец продукта, тех. лиды двух репозиториев |
| Целевой релиз | Production launch (после закрытия CRITICAL и HIGH) |

---

## 1. Executive Summary

Перед продакшн-запуском Midas Exchange (Go-бэкенд + React-фронтенд) выявлены 5 CRITICAL и 16 HIGH-находок: денежные расчёты на `float64`, токены сессии в `localStorage` и в URL WebSocket, fail-open webhook-секрет, отсутствие `FOR UPDATE` на кошельках с TOCTOU в OTC, открытый `CheckOrigin` для WebSocket, OTP без привязки к `userID`, отсутствующий rate-limit, сломанный фронтовый ESLint и пр. Все находки блокируют прод-запуск либо создают финансовый/учётный риск. План разбит на 10 тем (A–J) с явными зависимостями, подробной разбивкой по двум репозиториям, оценкой трудозатрат и стратегией выкатки. Совокупный объём — ориентировочно 18–24 человеко-дня (бэкенд) и 10–14 человеко-дней (фронт), при условии параллельной работы двух разработчиков. После закрытия всех CRITICAL и HIGH из этого плана продукт допускается до прода.

---

## 2. Оглавление

- [1. Executive Summary](#1-executive-summary)
- [2. Оглавление](#2-оглавление)
- [3. Сводная таблица находок](#3-сводная-таблица-находок)
- [3a. Детальное описание каждой находки](#3a-детальное-описание-каждой-находки)
- [4. Этапы плана (A–J)](#4-этапы-плана-aj)
  - [Этап A — Money precision](#этап-a--money-precision)
  - [Этап B — Auth tokens & session hardening](#этап-b--auth-tokens--session-hardening)
  - [Этап C — OTC race conditions](#этап-c--otc-race-conditions)
  - [Этап D — Rate limiting & security alerts](#этап-d--rate-limiting--security-alerts)
  - [Этап E — Webhook hardening](#этап-e--webhook-hardening)
  - [Этап F — WebSocket origin & ticket](#этап-f--websocket-origin--ticket)
  - [Этап G — Tooling & CI hygiene](#этап-g--tooling--ci-hygiene)
  - [Этап H — Type safety hygiene](#этап-h--type-safety-hygiene)
  - [Этап I — Frontend resilience](#этап-i--frontend-resilience)
  - [Этап J — WebAuthn / 2FA polishing](#этап-j--webauthn--2fa-polishing)
- [5. Порядок выполнения и зависимости](#5-порядок-выполнения-и-зависимости)
- [6. План тестирования](#6-план-тестирования)
- [7. Definition of Done](#7-definition-of-done)
- [8. Что не входит в этот план](#8-что-не-входит-в-этот-план)
- [9. Approval](#9-approval)

---

## 3. Сводная таблица находок

> Колонка **Сторона** показывает, в каком репозитории/слое находится первичная причина бага: **BE** = `midas_exchange_back` (Go), **FE** = `midas-exchange-frontend` (React/TS), **BE+FE** = парный баг, требующий синхронных правок.

| # | Severity | ID | Сторона | Краткое описание | Файл : строка | Подсистема | Этап |
|---|---|---|---|---|---|---|---|
| 1 | CRITICAL | CRIT-1 | **BE** | Webhook secret через `==` + fail-open | `internal/api/webhook/cryptogate_handler.go:26-31` | Cryptogate webhook (внешние интеграции) | E |
| 2 | CRITICAL | CRIT-2 | **BE** | `WalletGetForUpdateQuery` без `FOR UPDATE` + TOCTOU в OTC | `const/queries/wallet_repo.go:29` + `internal/service/otc_service.go:114-125` | Wallets / OTC (доменная транзакционность) | C |
| 3 | CRITICAL | CRIT-3 | **BE** | WS `CheckOrigin: return true` (CSWH) | `cmd/server/ws.go:36`, `cmd/server/otc_ws.go:32` | WebSocket / OTC chat | F |
| 4 | CRITICAL | CRIT-4 | **BE** | `float64` для денег во всех domain-моделях | `internal/domain/{wallet.go, transaction.go, order.go, otc.go}` | Money / Domain models | A |
| 5 | CRITICAL | CRIT-5 | **BE** | OTP не привязан к `userID` в `VerifyOTP` | `internal/service/twofa_service.go:134-171` + `internal/service/auth_service.go:332-348` | 2FA / Forgot-password flow | J |
| 6 | CRITICAL | C-1 | **FE** | JWT access+refresh в localStorage (zustand persist) | `src/store/authStore.ts:225-234` | Auth state / persistence | B |
| 7 | CRITICAL | C-2 | **FE** | `access_token` в URL WebSocket OTC | `src/pages/OTCOrderDetail.tsx:229`, `src/pages/AdminOTCOrderDetail.tsx:233` | OTC WebSocket client | B+F |
| 8 | CRITICAL | C-3 | **FE** | `number` (float64) для всех денежных полей | `src/types/index.ts:19,46,74-77,165-170` + `src/pages/Exchange.tsx:26-34`, `src/store/exchangeStore.ts:82-90` | Money types / расчёты обмена | A |
| 9 | CRITICAL | C-4 | **FE** | Stale closure + утечка WS в OTC ордере | `src/pages/OTCOrderDetail.tsx:199,223-237`, `src/pages/AdminOTCOrderDetail.tsx:223-241` | OTC realtime UI | C |
| 10 | HIGH | HIGH-1 | **BE** | TOCTOU pre-check `Balance < fromAmount` поверх кэша | `internal/service/otc_service.go:114-125` | OTC создание ордера | C |
| 11 | HIGH | HIGH-2 | **BE** | `OTCOrderAgreeQuery` UPDATE без `operator_id`/`status` в WHERE | `const/queries/otc_repo.go:55-61` | OTC state machine (admin) | C |
| 12 | HIGH | HIGH-3 | **BE** | `SendSecurityAlert` пишет только в лог, не доставляет | `internal/service/twofa_service.go:260-264` | 2FA / уведомления безопасности | D |
| 13 | HIGH | HIGH-4 | **BE** | Rate-limit middleware не подключено (конфиг есть) | `cmd/server/routes.go:89-108` + `pkg/config/config.go:176` | Auth endpoints (HTTP middleware) | D |
| 14 | HIGH | HIGH-5 | **BE** | Ошибка `BlockRawToken` при logout игнорируется | `internal/api/client/auth_handler.go:121` | Logout / JWT blocklist | B |
| 15 | HIGH | HIGH-6 | **BE** | Pre-check кошелька в exchange-flow по устаревшему кэшу | `internal/service/order_service.go:61-73` | Exchange (обмен валют) | A (косвенно) |
| 16 | HIGH | H-1 | **FE** | ESLint полностью сломан (Node 16 vs `structuredClone`) | `eslint.config.js` + системный Node 16.15.1 | Tooling / CI | G |
| 17 | HIGH | H-2 | **FE** | `PublicRoute` редиректит non-admin staff на `/exchange` | `src/routes/PublicRoute.tsx:14-19` | Routing / role access | I |
| 18 | HIGH | H-3 | **FE** | `ProtectedRoute` не вызывает `check_auth` при mount → flash UI | `src/routes/ProtectedRoute.tsx:8-18` + `src/App.tsx` | Routing / auth bootstrap | B |
| 19 | HIGH | H-4 | **FE** | Риск регрессии `pending_2fa_token` в persist | `src/store/authStore.ts:225-234` | Auth state / persistence | B |
| 20 | HIGH | H-5 | **FE** | `as any` для полей `BackendUser` | `src/pages/Profile.tsx:32-34,228-234`, `src/pages/AdminUserProfile.tsx:49,90` | Type safety / DTO | H |
| 21 | HIGH | H-6 | **FE** | Мёртвый `exchangeStore` с устаревшим endpoint `/exchange/execute` | `src/store/exchangeStore.ts` (полностью) | Dead code / Exchange | I |
| 22 | HIGH | H-7 | **FE** | `currency: wallet.currency.code as any` в WithdrawModal | `src/components/modals/WithdrawModal.tsx:87` | Type safety / Wallet DTO | H |
| 23 | HIGH | H-8 | **FE** | `parseInt(v, 0)` — radix 0 даёт авто-детект | `src/components/modals/CreateRateModal.tsx:86-87` | Type safety / Admin rates | H |
| 24 | HIGH | H-9 | **FE** | Нет ErrorBoundary на корне — белый экран при ошибке рендера | `src/main.tsx`, `src/App.tsx` | Resilience / общая отказоустойчивость | I |
| 25 | HIGH | H-10 | **FE** | `session_id`/`temp_token` в query string POST WebAuthn | `src/api/services/twoFactorService.ts:108` | WebAuthn / Passkey flow | J |

---

## 3a. Детальное описание каждой находки

> Раздел содержит полный разбор по каждой строке таблицы выше. Для каждой находки: метаданные, что происходит сейчас, сценарий эксплуатации/риск, направление фикса (детали реализации — в соответствующем этапе раздела 4).

---

### CRIT-1 — Timing-attack и fail-open в верификации webhook-секрета

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/api/webhook/cryptogate_handler.go:26-31` |
| Подсистема | Cryptogate webhook (приём депозитов от внешнего сервиса) |
| Этап плана | E |

**Что происходит сейчас.** Метод `verifySecret` сравнивает заголовок `X-TOKEN` с `h.webhookSecret` оператором `==`. Дополнительно: при `webhookSecret == ""` возвращает `true` («dev mode»), то есть пускает любой запрос.

**Сценарий эксплуатации.** (1) Timing attack: атакующий по разнице времени ответа сервера побайтово восстанавливает секрет, после чего отправляет поддельные depo-вебхуки и зачисляет деньги на свои кошельки. (2) Fail-open: при любой мисконфигурации/ошибке секретного хранилища (Vault не отдал значение, env-переменная не пробросилась в контейнер) `webhookSecret` становится пустой строкой — webhook принимает любые запросы со всего интернета без аутентификации. Для финтех-вебхука, через который зачисляются крипто-депозиты, это прямой путь к потере средств.

**Направление фикса.** Заменить сравнение на `subtle.ConstantTimeCompare`. При пустом `webhookSecret` — `return false` (fail-closed). Дополнительно — в `cmd/server/main.go` фейлить старт сервиса в `production`-окружении, если секрет пуст.

---

### CRIT-2 — `WalletGetForUpdateQuery` без `FOR UPDATE` + TOCTOU в OTC

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Backend** (Go + SQL) |
| Файл / строка | `const/queries/wallet_repo.go:29` (запрос) + `internal/service/otc_service.go:114-125` (использование) |
| Подсистема | Wallets / OTC (доменная транзакционность) |
| Этап плана | C |

**Что происходит сейчас.** Запрос называется `WalletGetForUpdateQuery`, но текст SQL — обычный `SELECT * FROM wallets WHERE id = $1`, без `FOR UPDATE`. Используется в `otc_service.go:114-125` для проверки `if fromWallet.Balance < fromAmount { return ErrInsufficient }` непосредственно перед вызовом `LockAmount`. Сам `LockAmount` атомарен (`UPDATE ... WHERE balance >= $1`), но pre-check читает кэш/БД без блокировки.

**Сценарий эксплуатации.** Два параллельных запроса `CreateOrder` от одного пользователя проходят pre-check одновременно (баланс достаточен для каждого по отдельности), затем оба вызывают `LockAmount`. SQL-уровень корректно отклонит второй (вернёт `0 rows affected`), но pre-check создаёт у разработчика ложное ощущение безопасности и маскирует более глубокий риск: имя `WalletGetForUpdate` обещает SELECT … FOR UPDATE, поэтому будущий рефактор может убрать атомарный `WHERE balance >= $1`, доверившись имени запроса. Кроме того, в OTC через цепочку «прочитал → отдал в кэш → списал» возможны несогласованные снимки, если две корутины пишут в разные части одного и того же кошелька.

**Направление фикса.** (1) В SQL добавить `FOR UPDATE`. (2) Вызовы выполнять строго внутри транзакции (`db.BeginTxx`). (3) Удалить pre-check `Balance < fromAmount` и доверять `WHERE balance >= $1` в SQL UPDATE. (4) Audit всех мест, где используется `WalletGetForUpdate`, — убедиться, что они в транзакции.

---

### CRIT-3 — WebSocket-апгрейдеры пускают любой Origin (CSWH)

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Backend** (Go) |
| Файл / строка | `cmd/server/ws.go:36`, `cmd/server/otc_ws.go:32` |
| Подсистема | WebSocket / OTC chat |
| Этап плана | F |

**Что происходит сейчас.** Оба WebSocket-апгрейдера задают `CheckOrigin: func(r *http.Request) bool { return true }`. В `ws.go` остался даже комментарий «adjust for production». Конфиг `AllowedOrigins` существует, но не применяется.

**Сценарий эксплуатации.** Cross-Site WebSocket Hijacking (CSWH): пока пользователь залогинен в Midas Exchange, атакующий заманивает его на сторонний сайт (`https://attacker.example`). Браузер автоматически прикрепляет cookie/auth-headers (если cookie SameSite не Strict, либо токен передаётся в URL — см. CRIT-2/C-2). Сторонняя страница открывает `new WebSocket("wss://midas/ws/otc/{uid}")`, бэк её принимает, и далее атакующая страница читает приватный поток OTC-чата (переписка пользователя с оператором, реквизиты, суммы) и отправляет в него команды от лица пользователя.

**Направление фикса.** `CheckOrigin` сверяет `r.Header.Get("Origin")` с allowlist из конфига. Параллельно ввести аутентификацию через одноразовый ws-ticket (см. этап F), чтобы заодно убрать `access_token` из URL.

---

### CRIT-4 — `float64` для всех денежных полей domain-моделей

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/domain/wallet.go:22-23`, `internal/domain/transaction.go:27-28`, `internal/domain/order.go:22-26`, `internal/domain/otc.go:52-56` (все денежные поля) |
| Подсистема | Money / Domain models / арифметика обмена и комиссий |
| Этап плана | A |

**Что происходит сейчас.** `Balance`, `Amount`, `Fee`, `Rate`, `ToAmountWithFee`, `AgreedRate`, `AgreedFromAmount` объявлены как `float64`. PG-колонки имеют точный тип `NUMERIC`. При SELECT `sqlx` приводит `NUMERIC → float64` — точность теряется ещё до арифметики. Расчёты вида `toAmount := req.FromAmount * rate.Rate` и `toAmountWithFee := toAmount * (100 - rate.Fee) / 100` (`order_service.go:57`) выполняются в IEEE-754.

**Сценарий риска.** Финансовая корректность: `0.1 + 0.2 = 0.30000000000000004`; `0.1 * 3 = 0.30000000000000004`. На больших объёмах и низких номиналах (BTC = 8 знаков, ETH = 18) расхождения накапливаются. Реальные последствия: (1) сумма, заблокированная при создании OTC-оффера, не равна сумме, списанной при `Complete` → расхождение в учёте; (2) пользователь видит в UI комиссию 0.5%, фактически списано 0.500000004% → жалобы и спор; (3) при auto-rebalancing нескольких кошельков — потеря/появление «фантомных» долей сатоши; (4) в OTC-аналитике суммарные обороты не сходятся с аудитом по транзакциям.

**Направление фикса.** Миграция на `github.com/shopspring/decimal`: тип `decimal.Decimal` для всех денежных полей; сравнения через `LessThan`/`GreaterThan`; арифметика — `Mul`, `Div`, `Add`, `Sub`; явные `Round(n)` под точность валюты. Контракт API — деньги как `string` (см. C-3 на фронте). Бэк и фронт деплоятся синхронно.

---

### CRIT-5 — OTP-верификация не проверяет `userID` владельца сессии

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/service/twofa_service.go:134-171` (verify) + `internal/service/auth_service.go:332-348` (forgot-password vs OTP) |
| Подсистема | 2FA / Forgot-password flow / Telegram-OTP |
| Этап плана | J |

**Что происходит сейчас.** При `SendOTP` в Redis-payload сохраняется `UserID`, но `VerifyOTP(phone, code, purpose)` принимает только телефон, код и purpose, и **не сравнивает** сохранённый `payload.UserID` с тем, кто верифицирует код. То есть достаточно знать телефон жертвы и (любым путём) её OTP-код.

**Сценарий эксплуатации.** Атакующий заходит в `/forgot-password`, передаёт `phone = phoneOfVictim` + код, который атакующий каким-то образом получил (социнженерия, утечка SMS-логов, перехват, уличный сценарий «жертва прочла OTP вслух»). Бэк ищет OTP по ключу `otp:reset_password:{phone}`, проверяет `code == payload.Code` — true. UserID не валидируется. В ответ возвращается `action_token`, по которому атакующий сбрасывает пароль чужого аккаунта. То же — для других purpose'ов (смена телефона, добавление 2FA), где flow допускает чужой `userID` в action-token.

**Направление фикса.** `VerifyOTP` принимает явный `expectedUserID` и сравнивает с `payload.UserID`. Несовпадение — 403 + инкремент `attempts`. В forgot-password: `userID` определяется по телефону **до** SendOTP и связывается с OTP-сессией; при verify сверяется именно эта пара.

---

### C-1 — JWT access + refresh хранятся в `localStorage` (zustand persist)

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/store/authStore.ts:225-234` (`partialize`) |
| Подсистема | Auth state / persistence |
| Этап плана | B |

**Что происходит сейчас.** `zustand/middleware persist` с `partialize` сохраняет `access_token` и `refresh_token` в `localStorage` под ключом `auth-storage`. Любой JS-код в origin читает их через `localStorage.getItem("auth-storage")`.

**Сценарий эксплуатации.** Любой XSS (через зависимость, скомпрометированный CDN, расширение браузера, рекламный SDK) получает оба токена — особенно опасен долгоживущий `refresh_token`. Атакующий может выкачивать новые access-токены месяцами после однократной XSS, даже если жертва закрыла браузер. Также токены доступны в DevTools и в backup-снимках localStorage, синхронизируемых браузером.

**Направление фикса.** Access-токен — только в памяти zustand-стора, без `persist`. Refresh-токен — в `httpOnly + Secure + SameSite=Strict` cookie, вообще недоступной JS. Обновление — POST `/auth/refresh` (cookie уйдёт автоматически). После logout — бэк удаляет cookie через `Set-Cookie: ...; Max-Age=0`.

---

### C-2 — `access_token` передаётся в query string при подключении WebSocket OTC

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Frontend** (TS/React) — пара с **Backend** (см. CRIT-3, F) |
| Файл / строка | `src/pages/OTCOrderDetail.tsx:229`, `src/pages/AdminOTCOrderDetail.tsx:233` |
| Подсистема | OTC WebSocket client |
| Этап плана | B+F |

**Что происходит сейчас.** Клиент подключается к WS как `new WebSocket("wss://.../ws/otc/{uid}?token=${access_token}")`. CLAUDE.md фронта явно отмечает этот риск, но фикса нет.

**Сценарий риска.** `access_token` попадает в (1) `access_log` nginx/балансера/CDN, (2) URL trace в DevTools и Network history, (3) Referrer headers, (4) логи SIEM/monitoring. Любой, у кого есть доступ к этим логам (DevOps, SRE, third-party log-shipper), фактически держит активные JWT-токены пользователей в открытом виде. Дополнительно — токен в URL живёт пока живёт его JWT-exp, не one-shot.

**Направление фикса.** На бэке — ввести endpoint `/auth/ws-ticket`, выдающий короткоживущий (TTL ≈ 30 сек) одноразовый ticket в Redis (`ws:ticket:{uuid} → userID`). На фронте — перед открытием WS делать GET ws-ticket, подключаться через `?ticket=...`. На бэке апгрейдер использует `redis.GETDEL` (одноразово) и проверяет, что `userID` ticket-а совпадает с `:uid` ордера.

---

### C-3 — `number` (float64) для всех денежных полей фронта

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Frontend** (TS/React) — пара с CRIT-4 (бэк) |
| Файл / строка | `src/types/index.ts:19,46,74-77,165-170` (типы); расчёты — `src/pages/Exchange.tsx:26-34`, `src/store/exchangeStore.ts:82-90`, `src/store/walletStore.ts`, `src/pages/OTCOrderDetail.tsx`, `src/pages/AdminOTCOrderDetail.tsx` |
| Подсистема | Money types / расчёты обмена / отображение балансов |
| Этап плана | A |

**Что происходит сейчас.** В TypeScript типы `Wallet.balance`, `CurrencyPair.rate/fee`, `Order.amount`, `OTCOrder.from_amount/to_amount` — все `number`. JS `number` — это float64 IEEE-754, та же проблема, что на бэке. Расчёт `(from_val * (1 - pair_fee / 100) * pair_rate).toFixed(8)` теряет точность в умножении до `toFixed`, который только маскирует проблему в выводе.

**Сценарий риска.** UI показывает пользователю расчётные суммы, бэк после reform на `decimal` будет считать иначе → расхождение между «вы получите 0.12345678 BTC» и фактически зачисленной суммой. Жалобы, support-нагрузка, доверие к продукту. Также `walletStore.get_total_balance` (`walletStore.ts:55-67`) умножает баланс кошелька на захардкоженный курс — двойная погрешность.

**Направление фикса.** Установить `decimal.js`/`big.js`. В типах денежные поля — `string` (сериализация бэка). Утилиты `src/lib/money.ts`: `toDecimal`, `formatMoney`. zod-схемы инпутов — `z.string().refine(/^\d+(\.\d+)?$/)`. Все умножения/деления — методы `Decimal`. Деплой синхронно с бэком.

---

### C-4 — Stale closure и утечка WS в OTC-ордере

| Поле | Значение |
|---|---|
| Severity | **CRITICAL** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/pages/OTCOrderDetail.tsx:199, 223-237`; аналог — `src/pages/AdminOTCOrderDetail.tsx:223-241` |
| Подсистема | OTC realtime UI |
| Этап плана | C |

**Что происходит сейчас.** `useEffect`, открывающий WS, имеет зависимости `[uid, order?.status, access_token]`. При каждой смене `order.status` создаётся новый WS, но старый `ws.onmessage` хранит замороженную (stale) версию `fetch_order`, замкнутую на прежний рендер. Также `fetch_order` (HTTP) не имеет `AbortController` — если компонент размонтируется в полёте, `set_order` отрабатывает на размонтированном компоненте.

**Сценарий риска.** При быстрой смене статусов ордера несколько WS-инстансов остаются открыты одновременно, каждый дёргает `fetch_order` из своего snapshot'а; ответы прилетают в разном порядке, `set_order` пишет последнее по времени, что не обязательно последнее по статусу — оператор видит «откатившийся» статус ордера. Это особенно опасно в шаге `awaiting_payment` → `paid` → `completed`: можно увидеть «ордер отменён», когда он уже завершён, и принять неверное решение.

**Направление фикса.** Вынести `fetch_order` в `useCallback`, держать «свежее» значение `order` в `useRef`. WS-эффект — зависимости `[uid]`. Закрывать прежний WS перед открытием нового. На стороне UI: при ошибке 409 от бэка — toast «State changed, refreshing» и принудительный refetch.

---

### HIGH-1 — TOCTOU pre-check в OTC (дублирует часть CRIT-2)

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/service/otc_service.go:114-125` |
| Подсистема | OTC создание ордера |
| Этап плана | C |

**Что происходит сейчас.** Перед `LockAmount` сервис делает `if fromWallet.Balance.LessThan(fromAmount) { return ErrInsufficient }` по данным, прочитанным из кэша/БД без блокировки.

**Сценарий риска.** Описан в CRIT-2 — дубль для отслеживания, фикс выполняется одним и тем же кодом-чейнджем (удалить pre-check, доверять SQL `WHERE balance >= $1`).

**Направление фикса.** Удалить pre-check; ошибку «недостаточно средств» возвращать на основании `RowsAffected() == 0` после `LockAmount`.

---

### HIGH-2 — `OTCOrderAgreeQuery` UPDATE без проверки `operator_id`/`status` в WHERE

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (SQL + Go) |
| Файл / строка | `const/queries/otc_repo.go:55-61` (запрос); вызов — `internal/service/otc_service.go:322-323` |
| Подсистема | OTC state machine (admin) |
| Этап плана | C |

**Что происходит сейчас.** `UPDATE otc_orders SET ... WHERE id = $5` — без `operator_id` и `status` в WHERE. Проверка `operator.OperatorID == operatorID` сделана в Go перед вызовом, но не на уровне SQL. То же — в `AcceptOffer`, `ConfirmPayment`, `Complete`, `Cancel`.

**Сценарий риска.** Гонка двух операторов или баг в логике может привести к UPDATE'у в неожиданном статусе. Атомарность БД нарушается на уровне business state machine: ордер «прыгает» в `awaiting_payment` из любого состояния, минуя обязательный `negotiating`.

**Направление фикса.** Расширить WHERE: `WHERE id = $5 AND operator_id = $6 AND status = 'negotiating'`. После UPDATE проверять `RowsAffected() == 1`, иначе — `409 Conflict`. Аналогично — для всех state-transition-запросов OTC.

---

### HIGH-3 — `SendSecurityAlert` пишет только в лог

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/service/twofa_service.go:260-264` |
| Подсистема | 2FA / уведомления безопасности |
| Этап плана | D |

**Что происходит сейчас.** Метод заявлен как «алерт о подозрительной активности» (новое устройство, смена 2FA), но реализация — `s.logger.Info("security alert (not delivered — no plain-message API)", ...)`. Пользователь ничего не получает.

**Сценарий риска.** Если злоумышленник скомпрометировал аккаунт и зашёл с нового устройства, единственный механизм оповещения — Telegram-сообщение — не работает. Пользователь не узнаёт о компрометации до момента обнаружения пропавших денег.

**Направление фикса.** Реализовать через Telegram Bot API (`sendMessage` к chat_id пользователя). Если у бэка ещё нет `pkg/telegram`-клиента — добавить, использовать `tgbotapi`. Текст: «Новый вход с устройства X, IP Y, время Z. Если это не вы — нажмите …». Логирование оставить как audit-trail.

---

### HIGH-4 — Rate-limit middleware не подключено

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (Go) |
| Файл / строка | `cmd/server/routes.go:89-108` (auth-маршруты), `pkg/config/config.go:176` (мёртвый конфиг) |
| Подсистема | Auth endpoints (HTTP middleware) |
| Этап плана | D |

**Что происходит сейчас.** Конфиг `RateLimitRequests`/`RateLimitWindow` объявлен и читается, но middleware отсутствует. Маршруты `/auth/login`, `/auth/register`, `/auth/2fa/telegram/verify`, `/auth/forgot-password/send|verify` принимают неограниченное число запросов в секунду.

**Сценарий эксплуатации.** Брутфорс паролей на `/auth/login`, enumeration пользователей по timing'у на `/auth/register`, OTP-брутфорс на `/auth/2fa/telegram/verify` (если коды короткие — за минуты переберутся), DoS-нагрузка на `/auth/forgot-password/send` (отправляет реальные SMS, тратит бюджет).

**Направление фикса.** Реализовать `internal/api/middleware/ratelimit.go` на `golang.org/x/time/rate`, per-IP `*rate.Limiter` в LRU-кэше (`hashicorp/golang-lru/v2`). 429 + `Retry-After`. Применить к auth-группе. Учитывать `X-Forwarded-For` через trusted-proxy-list.

---

### HIGH-5 — Ошибка `BlockRawToken` при logout молча игнорируется

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/api/client/auth_handler.go:121` |
| Подсистема | Logout / JWT blocklist |
| Этап плана | B |

**Что происходит сейчас.** `_ = h.authMiddleware.BlockRawToken(r.Context(), rawToken)` — паттерн «отбросить ошибку». Если Redis недоступен в момент logout, токен не попадает в blocklist. `isBlocklisted` при недоступном Redis возвращает true (fail-closed) — то есть пока Redis лежит, все JWT'ы блокированы; но как только Redis восстановится, **старый** токен снова станет валидным до своего exp.

**Сценарий риска.** Злоумышленник украл токен, пользователь сделал logout «на всякий случай», но из-за временной недоступности Redis blocklist-запись потерялась. После восстановления Redis атакующий продолжает использовать токен.

**Направление фикса.** Логировать ошибку и возвращать клиенту 500 «logout failed»: фронт повторит. Alternative — записывать в персистентный outbox (БД) и фоновым воркером дозаписывать в Redis после восстановления.

---

### HIGH-6 — Pre-check кошелька в exchange-flow по устаревшему кэшу

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Backend** (Go) |
| Файл / строка | `internal/service/order_service.go:61-73` |
| Подсистема | Exchange (обмен валют) |
| Этап плана | A (косвенно) / самостоятельный микро-фикс |

**Что происходит сейчас.** `GetByUserAndCurrency` идёт через кэш (Redis/in-memory), затем `AtomicDeduct` атомарно списывает по SQL `WHERE balance >= $1`. Сама финансовая операция корректна; используется только `wallet.ID` — не баланс. Но есть архитектурная мина: `wallet` после `Get*` локально рассматривается как «свежий», что приводит к рассогласованию при последующих чтениях.

**Сценарий риска.** В текущей реализации прямой эксплуатации нет. Риск — будущий рефакторинг доверится «полному» wallet-снимку и введёт условие на `wallet.Balance` без транзакции — гонка появится автоматически.

**Направление фикса.** Возвращать из `Get*` только `walletID` либо явно помечать структуру как «not for arithmetic». В рамках этапа A (Money) уйдёт за компанию — кэш будет обновляться через `RETURNING` после `AtomicDeduct`.

---

### H-1 — ESLint полностью сломан (Node 16 vs `structuredClone`)

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) — Tooling |
| Файл / строка | `eslint.config.js`; runtime: системный Node 16.15.1; лог ошибки см. отчёт по аудиту |
| Подсистема | Tooling / CI |
| Этап плана | G |

**Что происходит сейчас.** `eslint@9` + `@typescript-eslint@8` используют `structuredClone`, появившийся в Node 17. Локальный/CI Node — 16. `npm run lint` падает с `ConfigError: ... structuredClone is not defined` ещё до проверки кода. То есть весь репозиторий мёрджится без ESLint-валидации.

**Сценарий риска.** Все остальные находки type-safety (H-5/H-7/H-8) — следствие того, что линтер не запущен; новые регрессии не отлавливаются.

**Направление фикса.** Поднять Node до 20 LTS (`.nvmrc`, `package.json.engines`, Docker-image, CI workflow). Прогнать `npm ci && npm run lint`, починить вылезшие ошибки.

---

### H-2 — `PublicRoute` редиректит non-admin staff на `/exchange`

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/routes/PublicRoute.tsx:14-19` |
| Подсистема | Routing / role access |
| Этап плана | I |

**Что происходит сейчас.** В `PublicRoute` после успешного логина проверяется только `user?.role === "admin"`. Все остальные роли (`super_admin`, `operator`, `support`) попадают на `/exchange` (клиентский маршрут), где их сразу пинает `ClientRoute` обратно — UX сломан, выглядит как «двойной редирект» или мерцание.

**Сценарий риска.** Не безопасностный, но нарушает работоспособность для staff-ролей и подрывает доверие к продукту. Ошибочно может стать security-проблемой, если в будущем `ClientRoute` начнёт раскрывать клиентские данные до редиректа.

**Направление фикса.** Использовать константу `STAFF_ROLES` (есть в `src/routes/index.tsx`), редиректить весь staff на `/admin/exchanges`.

---

### H-3 — `ProtectedRoute` не вызывает `check_auth` при mount

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/routes/ProtectedRoute.tsx:8-18` + `src/App.tsx` (нет вызова `check_auth` при старте) |
| Подсистема | Routing / auth bootstrap |
| Этап плана | B |

**Что происходит сейчас.** `ProtectedRoute` проверяет только локальный флаг `is_authenticated`, который переживает перезагрузку страницы через persist. При истёкшем access-токене флаг ещё `true` — рендерится защищённый UI. Только следующий API-запрос получает 401, и axios-интерцептор вызывает logout.

**Сценарий риска.** «Вспышка» приватного UI с потенциально чувствительными данными до редиректа: имя пользователя, баланс, скелетоны транзакций успевают мелькнуть. Также в DevTools остаётся история запросов, которые сделал «полу-залогиненный» пользователь.

**Направление фикса.** В `App.tsx` (или внутри `ProtectedRoute`) при mount вызывать `authStore.check_auth()`. До завершения проверки — рендерить spinner/skeleton, не приватный UI. Если check_auth вернул 401 — редирект на `/login`.

---

### H-4 — Регрессия `pending_2fa_token` в persist

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/store/authStore.ts:225-234` |
| Подсистема | Auth state / persistence |
| Этап плана | B |

**Что происходит сейчас.** Сейчас `pending_2fa_token` **не** включён в `partialize`, что правильно. Но это негативное условие — нет теста, который бы это закрепил. Любой будущий разработчик, добавляющий поле в `partialize` через автокомплит/copy-paste, может случайно протащить `pending_2fa_token` в `localStorage`, что превратит временный 2FA-токен в долгоживущий и бьющий C-1 эксплойт ещё сильнее.

**Сценарий риска.** Регрессия. Низкая текущая вероятность, но катастрофические последствия (`pending_2fa_token` — это «полу-аутентификация» до прохождения 2FA, его кража = обход 2FA).

**Направление фикса.** Unit-тест на `partialize`, проверяющий, что выходной объект не содержит ключей `pending_2fa_token`, `access_token`, `refresh_token`. После этапа B (когда токены окончательно покинут persist) — оставить тест как guard.

---

### H-5 — `as any` для полей `BackendUser`

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/pages/Profile.tsx:32-34, 228-234`; `src/pages/AdminUserProfile.tsx:49, 90` |
| Подсистема | Type safety / DTO |
| Этап плана | H |

**Что происходит сейчас.** Тип `BackendUser` (в `authService.ts`) описан неполно. В UI поля `first_name`, `last_name`, `phone`, `kyc_level`, `passkey_enabled`, `middle_name` читаются через `(user as any).first_name`. ~8 явных `as any`.

**Сценарий риска.** Любая рассинхронизация контракта (бэк переименовал поле, изменил тип) проходит мимо TS-проверок. Ошибка вылезает в runtime в виде `undefined` в UI или `TypeError` при `.toLowerCase()`. Также маскирует факт, что `BackendUser` не сериализован в zod-схеме.

**Направление фикса.** Расширить тип `BackendUser` в `src/api/services/authService.ts`. Убрать все `as any`. Дополнительно — включить ESLint-правило `@typescript-eslint/no-explicit-any` уровня `error` (после фикса).

---

### H-6 — Мёртвый `exchangeStore` со старым endpoint `/exchange/execute`

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/store/exchangeStore.ts` (полностью); используется в `src/pages/Home.tsx` для mock-калькулятора |
| Подсистема | Dead code / Exchange |
| Этап плана | I |

**Что происходит сейчас.** Стор реализует `execute_exchange` через endpoint `/exchange/execute`, которого больше нет на бэке (миграция на `/exchanges`). Реальный обмен идёт через `exchangesService` в `Exchange.tsx`. `exchangeStore` остаётся в бандле и используется в `Home.tsx` для mock-калькулятора — пользователь видит фейковые курсы.

**Сценарий риска.** (1) Любой разработчик случайно импортирует стор — возвращается 404. (2) Mock-калькулятор показывает захардкоженные неверные курсы (`USD/BTC = 0.000023` — устаревшее) — вводит пользователя в заблуждение. (3) Дополнительный bundle-size без пользы.

**Направление фикса.** Удалить `exchangeStore.ts`. В `Home.tsx` либо подтягивать реальные курсы из API, либо явно пометить «индикативно» и убрать «обмен». `grep -r "exchangeStore" src/` → 0.

---

### H-7 — `currency: wallet.currency.code as any` в WithdrawModal

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/components/modals/WithdrawModal.tsx:87` |
| Подсистема | Type safety / Wallet DTO |
| Этап плана | H |

**Что происходит сейчас.** `walletService.withdraw` ожидает `currency: "KZT" | "USD" | "BTC" | "ETH" | "USDT"` (union). `wallet.currency.code` — обычный `string`, потому что `CurrencyInfo.code: string`. Результат — `as any` для затыкания TS.

**Сценарий риска.** Нет валидации: если бэк когда-нибудь вернёт «новую» валюту, фронт молча отправит её в `withdraw` — 400 от бэка в лучшем случае, нештатное поведение в худшем.

**Направление фикса.** Сузить тип `CurrencyInfo.code` до union либо ввести runtime-валидацию `if (!ALLOWED_CURRENCIES.includes(code)) throw ...`.

---

### H-8 — `parseInt(v, 0)` — radix 0 даёт авто-детект

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/components/modals/CreateRateModal.tsx:86-87` |
| Подсистема | Type safety / Admin rates |
| Этап плана | H |

**Что происходит сейчас.** `parseInt(values.from_currency_id, 0)` и `parseInt(values.to_currency_id, 0)`. С radix=0 JS возвращается к авто-детекту: префикс `"0x"` → hex, лидирующий `"0"` в строгом режиме = 8 (хотя ECMAScript 5+ это убрал, движки могут варьироваться).

**Сценарий риска.** При значении `"0x10"` вместо `16` пользователь получит идентификатор `16`; при значении `"0123"` поведение между Node-версиями исторически отличалось. Скрытая бомба, если когда-нибудь currency_id придёт как строка с префиксом.

**Направление фикса.** `parseInt(v, 10)`. Проверка через ESLint-правило `radix: "always"`.

---

### H-9 — Нет ErrorBoundary на корне

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) |
| Файл / строка | `src/main.tsx`, `src/App.tsx` |
| Подсистема | Resilience / общая отказоустойчивость |
| Этап плана | I |

**Что происходит сейчас.** Дерево обёрнуто только в `<StrictMode><App /></StrictMode>`. ErrorBoundary нет ни на одном уровне.

**Сценарий риска.** Любой `throw` из `format(new Date(null), ...)` или `wallet.balance.toFixed(8)` (если бэк вернул `null` вместо ожидаемого) обрушивает весь SPA в белый экран. Пользователь финтех-приложения видит белую страницу — потеря доверия. Также теряется возможность отправить traceback в Sentry/posthog для диагностики.

**Направление фикса.** Классовый `<ErrorBoundary>` на уровне `<App />` с fallback-UI и кнопкой «Reload». Внутри `componentDidCatch` — отправка в Sentry/posthog (если подключены) либо `console.error`. Дополнительно — мелкие boundary вокруг крупных страниц (`Exchange`, `OTCOrderDetail`, `AdminOTCOrders`).

---

### H-10 — `session_id`/`temp_token` в query string POST WebAuthn

| Поле | Значение |
|---|---|
| Severity | **HIGH** |
| Сторона | **Frontend** (TS/React) — пара с **Backend** |
| Файл / строка | `src/api/services/twoFactorService.ts:108` |
| Подсистема | WebAuthn / Passkey flow |
| Этап плана | J |

**Что происходит сейчас.** POST-запрос `/auth/2fa/passkey/finish?session_id=...&temp_token=...` — оба чувствительных параметра идут в query, а не в body. POST URL логируется во всех access-логах.

**Сценарий риска.** Те же риски, что у C-2 (токен в URL), но в меньшем масштабе: `temp_token` короткоживущий, ограниченной мощности, но всё ещё даёт окно компрометации flow. `session_id` без `temp_token` бесполезен, но при утечке обоих из логов — обход passkey-верификации.

**Направление фикса.** Передавать `session_id` и `temp_token` в JSON body POST-запроса. На бэке временно поддерживать оба варианта (query + body) на один спринт, затем — только body. Документировать в OpenAPI.

---

## 4. Этапы плана (A–J)

> **Обозначения**: «бэк» — `/Users/gtrondin/Development/midas-exchange/midas_exchange_back`; «фронт» — `/Users/gtrondin/Development/midas-exchange/midas-exchange-frontend`. Пути даны относительно соответствующих репозиториев.

---

### Этап A — Money precision

**Закрывает:** CRIT-4, C-3, частично HIGH-6.

**Проблема и риск.** Все денежные поля (`Balance`, `Amount`, `Fee`, `Rate`, `ToAmountWithFee`) на бэке хранятся в Go как `float64`, на фронте — как `number` (тоже IEEE-754). PG-колонки имеют тип `NUMERIC`, значит точность теряется при чтении в Go. На больших суммах и низких номиналах (BTC/ETH с 8–18 знаками) это даёт расхождение баланса, неверные комиссии и потенциальные финансовые потери. Расчёты на фронте (`Exchange.tsx`, `exchangeStore.ts`) усиливают расхождение: пользователь видит одно, на бэке считается другое.

**Шаги — backend.**
1. Добавить зависимость `github.com/shopspring/decimal` (`go get`).
2. Заменить тип всех денежных полей в:
   - `internal/domain/wallet.go` — `Balance float64` → `Balance decimal.Decimal`.
   - `internal/domain/transaction.go` — `Amount`, `Fee`.
   - `internal/domain/order.go` — `Amount`, `FromAmount`, `ToAmount`, `ToAmountWithFee`, `Rate`, `Fee`.
   - `internal/domain/otc.go` — все денежные поля заявок и офферов.
3. Адаптировать DTO в `internal/dto/*` (сериализация в `string` для JSON через `decimal.MarshalJSON`).
4. Привести `internal/repository/*` к `decimal.Decimal` через `sqlx`-сканирование (`Scan` уже умеет, проверить теги).
5. Переписать арифметику в:
   - `internal/service/order_service.go` (включая HIGH-6 — pre-check на кэше: убрать или заменить на «индикативную проверку», окончательное решение — на уровне SQL).
   - `internal/service/otc_service.go`.
   - `internal/service/cryptogate_service.go`.
   - `internal/service/wallet_service.go`.
   - `internal/service/platform_fee_service.go`.
6. Сравнения вида `a < b` заменить на `a.LessThan(b)`, операции `+`/`-`/`*`/`/` — на методы `decimal`. Округления — `decimal.Decimal.Round(n)` с явной точностью на валюту.
7. Тесты (`internal/service/*_test.go`): золотые тесты на 8-знаковые BTC, на USDT-стейблкоин (2 знака после запятой), на сложение/вычитание/деление с фиксированным rate.
8. Обновить OpenAPI/контракт: денежные поля передаются как `string` десятичной записи (например, `"0.00000123"`).

**Шаги — frontend.**
1. Добавить `decimal.js` (или `big.js`) в `package.json`.
2. В `src/types/index.ts` все денежные поля (строки 19, 46, 74–77, 165–170 — `balance`, `amount`, `fee`, `rate`, `to_amount_with_fee`, `from_amount`) сделать `string` (как пришло с бэка).
3. Создать утилиты `src/lib/money.ts`:
   ```ts
   import Decimal from "decimal.js";
   export const toDecimal = (v: string | number) => new Decimal(v);
   export const formatMoney = (v: string, scale = 2) => new Decimal(v).toFixed(scale);
   ```
4. Переписать расчёты:
   - `src/pages/Exchange.tsx:26-34` — заменить `Number()` арифметику на `Decimal`.
   - `src/store/exchangeStore.ts:82-90` — то же (если стор не удалён в этапе I; если удаляется — пропустить).
   - `src/store/walletStore.ts` — отображение/сравнения балансов.
   - `src/pages/OTCOrderDetail.tsx`, `AdminOTCOrderDetail.tsx` — суммы офферов.
5. Компоненты ввода (react-hook-form + zod) — zod-схемы переписать на `z.string().refine(v => /^\d+(\.\d+)?$/.test(v))`.
6. Форматирование вывода — через `Intl.NumberFormat` поверх `Decimal.toFixed()`.

**Acceptance criteria.**
- [ ] Поиск `float64` рядом с `Balance|Amount|Fee|Rate` в `internal/domain/**` возвращает 0 совпадений.
- [ ] Поиск `: number` рядом с `balance|amount|fee|rate` в `src/types/index.ts` возвращает 0 совпадений.
- [ ] Backend unit-тесты проходят с 8-знаковой точностью на BTC.
- [ ] E2E: депозит 0.12345678 BTC, частичное использование 0.00000001 BTC в обмене, остаток на кошельке = 0.12345677 (точно).
- [ ] При продаже 0.1 BTC по курсу 5 432 100,33 KZT/BTC и комиссии 0.5% результат совпадает между UI и БД с точностью до 8 знаков.

**Риски миграции.**
- Контракт API меняется (числа → строки) — frontend и backend деплоятся **синхронно**. Нет промежуточного состояния, где один из них уже на decimal, а другой ещё на float.
- Внешние интеграции (cryptogate webhook): уточнить формат поля `amount` в payload и привести к `decimal.NewFromString` (без обращения через float).
- Откат: тег API-версии `v2` или флаг `MIDAS_MONEY_DECIMAL=true` для пилотного периода (см. ниже).

**Стратегия выкатки.**
1. Фича-флаг **не используется** (риск рассинхронизации форматов слишком высок). Вместо этого — короткий «freeze window»: одновременный деплой бэка и фронта, предварительно прогнанный на staging с реальными данными prod-копии.
2. Миграция БД не требуется (типы в PG уже `NUMERIC`).
3. Перед мерджем — параллельный запуск unit + E2E тестов на CI.

**Оценка.** Backend: 32–40 часов. Frontend: 24–32 часа.

**Зависимости.** Самостоятельный, но желательно делать после или параллельно с этапом G (CI), чтобы тесты прогонялись автоматически.

---

### Этап B — Auth tokens & session hardening

**Закрывает:** C-1, C-2 (вместе с F), HIGH-5, H-3, H-4.

**Проблема и риск.** Access + refresh JWT хранятся в `localStorage` через zustand persist, что делает их доступными любому XSS. `access_token` передаётся в query string при подключении к WS OTC — попадает в серверные access-логи. На бэке `BlockRawToken` при logout вызывается с игнорированием ошибки. Фронт не вызывает `check_auth` при mount, что приводит к флешу приватного UI до редиректа. `pending_2fa_token` исторически сохранялся в persist — есть риск регрессии.

**Шаги — backend.**
1. Реализовать httpOnly + Secure + SameSite=Strict cookie для refresh-токена:
   - Endpoint `/auth/refresh` принимает cookie `mx_refresh`, не body.
   - При login/2FA-verify — `Set-Cookie: mx_refresh=...; HttpOnly; Secure; Path=/api/v1/auth; SameSite=Strict; Max-Age=...`.
   - Файлы: `internal/api/client/auth_handler.go`, `internal/service/auth_service.go`.
2. `internal/api/client/auth_handler.go:121` — заменить `_ = h.authMiddleware.BlockRawToken(...)` на проверку и логирование ошибки + возврат 500, если blocklist не сработал (HIGH-5):
   ```go
   if err := h.authMiddleware.BlockRawToken(r.Context(), raw); err != nil {
       h.log.Error("logout: blocklist failed", "error", err)
       http.Error(w, "logout failed", http.StatusInternalServerError); return
   }
   ```
3. Новый endpoint `POST /api/v1/auth/ws-ticket` (требует валидный JWT): возвращает короткоживущий (TTL 30 секунд) одноразовый ticket в Redis (`ws:ticket:{uuid}` → `userID`). Используется WS-апгрейдером в этапе F.
4. Логи (zap/logger) маскируют значение `Authorization`/`Cookie` headers глобально.

**Шаги — frontend.**
1. `src/store/authStore.ts:225-234` — убрать `access_token` и `refresh_token` из `partialize`. Хранить access in-memory (только в стейте zustand без persist), refresh — в httpOnly cookie (фронт его не видит).
2. `src/api/client.ts` axios interceptor:
   - Запросы делаются с `withCredentials: true`.
   - На 401 — POST `/auth/refresh` без body (cookie уйдёт автоматически), при успехе — обновить in-memory access.
3. `src/pages/OTCOrderDetail.tsx:229` и `src/pages/AdminOTCOrderDetail.tsx:233` — перед открытием WS вызывать `authService.get_ws_ticket()` и подключаться через `wss://.../ws/otc/{uid}?ticket=...`. Тикет одноразовый.
4. `src/components/auth/ProtectedRoute.tsx:8-18` — в `useEffect` при mount вызывать `authStore.check_auth()`. До завершения проверки — рендерить spinner, не приватный UI (фикс H-3).
5. Тест-регрессия H-4: добавить unit-тест на `partialize`, проверяющий, что объект не содержит `pending_2fa_token`, `access_token`, `refresh_token`.
6. После logout — POST `/auth/logout` с `withCredentials`, бэк очищает cookie через `Set-Cookie: mx_refresh=; Max-Age=0`.

**Acceptance criteria.**
- [ ] `localStorage` после login не содержит JWT (проверка через DevTools или unit-тест на `partialize`).
- [ ] WS OTC подключается только через `?ticket=...`, повторное использование тикета даёт 401.
- [ ] Logout: после успешного запроса повторный запрос с тем же access токеном даёт 401 (проверка через интеграционный тест на бэке).
- [ ] При входе на `/exchange` без сессии нет вспышки приватного UI (Cypress-тест).
- [ ] `pending_2fa_token` ни при каких условиях не попадает в `localStorage`.

**Риски миграции.**
- Cookie + CORS: бэк должен явно разрешить `Access-Control-Allow-Credentials: true` и точный origin.
- Существующие сессии — `localStorage` токены — после деплоя инвалидируются → принудительный re-login. Допустимо, но коммуникация в Telegram-уведомлении в день деплоя.
- Откат: при деплое-проблеме можно временно вернуть старый flow через флаг `MIDAS_AUTH_COOKIE=false` (бэк будет принимать refresh из body), но прод-выкатку делать сразу на cookie.

**Стратегия выкатки.**
1. Деплой бэка с поддержкой обоих flow (cookie + старый body) под флагом — 1 день в staging.
2. Деплой фронта на cookie-flow.
3. Через 24 часа после прод-деплоя — отключение body-flow на бэке.

**Оценка.** Backend: 16–20 часов. Frontend: 12–16 часов.

**Зависимости.** Этап F (WS Origin & ticket) зависит от шага 3 этапа B (новый ws-ticket endpoint).

---

### Этап C — OTC race conditions

**Закрывает:** CRIT-2, HIGH-1, HIGH-2, C-4.

**Проблема и риск.** `WalletGetForUpdateQuery` (`const/queries/wallet_repo.go:29`) не содержит `FOR UPDATE`, при этом `internal/service/otc_service.go:114-125` проверяет `Balance < fromAmount` по кэшу и **затем** дёргает `LockAmount`. Между чтением и записью — окно гонки. При двух параллельных OTC-офферах одного пользователя возможен двойной resv одного и того же баланса. `OTCOrderAgreeQuery` (`const/queries/otc_repo.go:55-61`) выполняет `UPDATE` без проверки `operator_id` и текущего `status` в WHERE, что позволяет «принять оффер» в неподходящем состоянии. На фронте WS-эффект `OTCOrderDetail.tsx:199,223-237` имеет stale closure: при смене статуса хэндлеры держат прежние значения.

**Шаги — backend.**
1. `const/queries/wallet_repo.go:29` — добавить `FOR UPDATE`:
   ```sql
   SELECT id, user_id, currency_id, balance, locked_balance
     FROM wallets
    WHERE user_id = $1 AND currency_id = $2
    FOR UPDATE
   ```
2. `internal/service/otc_service.go:114-125`:
   - Открыть транзакцию (`db.BeginTxx`) перед `WalletGetForUpdate`.
   - Удалить pre-check `if wallet.Balance.LessThan(fromAmount) { return ErrInsufficient }`.
   - Доверять SQL `WHERE balance >= $1` в `LockAmount` — `0 rows affected` → ошибка `ErrInsufficient`.
   - Коммит транзакции после `LockAmount`.
3. `const/queries/otc_repo.go:55-61` (`OTCOrderAgreeQuery`) — расширить WHERE:
   ```sql
   UPDATE otc_orders SET status = 'awaiting_payment', updated_at = NOW()
    WHERE id = $1 AND operator_id = $2 AND status = 'negotiating'
   ```
   Проверять `RowsAffected() == 1`, иначе — `409 Conflict`.
4. Аналогично — `AcceptOffer`, `RejectOffer`, `ConfirmPayment`, `Complete`, `Cancel`: атомарный WHERE на ожидаемом статусе и (где применимо) `operator_id`.
5. Тесты (`internal/service/otc_service_test.go`): два goroutine, параллельный `LockAmount` 100 раз → ровно один успех при балансе на 1 операцию.

**Шаги — frontend.**
1. `src/pages/OTCOrderDetail.tsx`:
   - Вынести WS-хэндлеры в `useCallback` с явными зависимостями.
   - Хранить «свежее» значение `order` в `useRef` (`orderRef = useRef(order); orderRef.current = order`); внутри хэндлеров читать `orderRef.current`.
   - `useEffect` подключения к WS — зависимости `[uid]`, без `order`.
2. Аналогично — `src/pages/AdminOTCOrderDetail.tsx:199,223-237`.
3. На стороне UI — оптимистичные апдейты статусов отключены при ошибке 409 от бэка (toast «Order state changed, refreshing»).

**Acceptance criteria.**
- [ ] Backend: 100 параллельных `LockAmount` на одном wallet → только один успех (race-test).
- [ ] `OTCOrderAgreeQuery` при двойном клике operator → второй вызов получает 409.
- [ ] Frontend: при ручном изменении статуса в одном табе UI второго таба обновляется без stale closure (Cypress-сценарий).

**Риски миграции.**
- `FOR UPDATE` повышает локвейты при горячих кошельках. Митигация: транзакция короткая (только `SELECT...FOR UPDATE` + `UPDATE`), `statement_timeout=2s` на pgxpool.
- При наличии legacy-вызовов `WalletGetForUpdate` вне транзакции — `FOR UPDATE` бросит ошибку. Перед мерджем — grep по `WalletGetForUpdate` в `internal/service/**` и убедиться, что все вызовы внутри `db.BeginTxx`.

**Стратегия выкатки.** Один атомарный деплой бэка + фронта; обратной совместимости не требуется. Откат — revert.

**Оценка.** Backend: 12–16 часов. Frontend: 4–6 часов.

**Зависимости.** Желательно после A (Money precision), чтобы `Balance.LessThan` уже был на `decimal`. Можно выполнять параллельно, мёрджить — после A.

---

### Этап D — Rate limiting & security alerts

**Закрывает:** HIGH-3, HIGH-4.

**Проблема и риск.** Конфиг `RateLimitRequests/Window` объявлен (`pkg/config/config.go:176`), но middleware не реализовано и не подключено в `cmd/server/routes.go:89-108`. `SendSecurityAlert` (`internal/service/twofa_service.go:260-264`) пишет только в log — пользователь не получает уведомления о подозрительной активности.

**Шаги — backend.**
1. Создать `internal/api/middleware/ratelimit.go` на основе `golang.org/x/time/rate`:
   - Per-IP `*rate.Limiter` хранится в LRU-кэше (`hashicorp/golang-lru/v2`).
   - 429 + `Retry-After` header при превышении.
   - Учёт `X-Forwarded-For` через trusted proxy list.
2. Подключить в `cmd/server/routes.go:89-108` к маршрутам:
   - `/auth/login` — 10 req / 15 min.
   - `/auth/register` — 5 req / 15 min.
   - `/auth/2fa/telegram/verify`, `/auth/2fa/verify-telegram`, `/auth/2fa/passkey/finish` — 10 req / 5 min.
   - `/auth/forgot-password/send`, `/auth/forgot-password/verify` — 5 req / 15 min.
3. Реализовать `SendSecurityAlert` через Telegram Bot API:
   - Использовать существующий `pkg/telegram` клиент (если есть) либо инжектить `*tgbotapi.BotAPI` в `TwoFactorService`.
   - Сообщение: «Новый вход с устройства {fingerprint}, IP {ip}, время {ts}».
   - Файл: `internal/service/twofa_service.go:260-264`.

**Шаги — frontend.** Не требуются. На UI отобразить toast «Слишком много попыток, попробуйте через {N} секунд» — уже работает через axios-ошибку 429 (проверить).

**Acceptance criteria.**
- [ ] 11-й POST на `/auth/login` за 15 минут с одного IP → 429 + `Retry-After`.
- [ ] При входе с нового fingerprint в Telegram прилетает сообщение в течение 10 секунд.
- [ ] Логи backend содержат запись о выданном rate-limit hit'е (для алертинга).

**Риски миграции.**
- За NAT'ом несколько пользователей могут разделять IP — лимит 5 регистраций / 15 мин может быть жёстким для офисных сетей. Митигация: оставить на 15 минут для прода, в staging — 60 секунд.
- Telegram Bot API rate limits — добавить ретрай с экспоненциальным backoff.

**Стратегия выкатки.** Деплой бэка, флаг `MIDAS_RATELIMIT_ENABLED=false` для отката. Алерты — без флага, т. к. чисто аддитивная функциональность.

**Оценка.** Backend: 10–14 часов. Frontend: 0–2 часа (только проверка toast'ов).

**Зависимости.** Нет.

---

### Этап E — Webhook hardening

**Закрывает:** CRIT-1.

**Проблема и риск.** `internal/api/webhook/cryptogate_handler.go:30` сравнивает секрет через `==` (timing-атака) и `r.Header.Get("X-TOKEN")` плюс fail-open при пустом секрете (`webhookSecret == "" → allow all`, строки 27–29). Любая мисконфигурация в проде = открытый webhook, через который можно вкачивать поддельные депозиты.

**Шаги — backend.**
1. `internal/api/webhook/cryptogate_handler.go:26-31` заменить на:
   ```go
   func (h *CryptoGateHandler) verifySecret(r *http.Request) bool {
       if h.webhookSecret == "" { return false } // fail-closed
       got := []byte(r.Header.Get("X-TOKEN"))
       want := []byte(h.webhookSecret)
       return subtle.ConstantTimeCompare(got, want) == 1
   }
   ```
2. В `cmd/server/main.go` (или там, где конструируется `CryptoGateHandler`) — фейлить старт сервиса при пустом `webhookSecret` в prod-окружении (`config.Env == "production"`).
3. Тест: unit на `verifySecret` — пустой секрет → false; верный секрет → true; разная длина → false без panic.

**Шаги — frontend.** Не требуются.

**Acceptance criteria.**
- [ ] Запуск бэка с `CRYPTOGATE_WEBHOOK_SECRET=""` в production-режиме завершается с ошибкой.
- [ ] Запрос на `/cg/deposit` без правильного X-TOKEN → 401 (даже если секрет на сервере «случайно» пустой).
- [ ] Constant-time проверка подтверждена тестом на двух разных по длине входах.

**Риски миграции.** Минимальные. Перед деплоем убедиться, что `CRYPTOGATE_WEBHOOK_SECRET` выставлен на проде (verify в Vault/secrets).

**Стратегия выкатки.** Один деплой. Откат — revert (риск низкий).

**Оценка.** Backend: 2–3 часа. Frontend: 0.

**Зависимости.** Нет. Делать как можно раньше (быстрый низкорисковый фикс).

---

### Этап F — WebSocket origin & ticket

**Закрывает:** CRIT-3, C-2 (совместно с B).

**Проблема и риск.** `cmd/server/ws.go:36` и `cmd/server/otc_ws.go:32` имеют `CheckOrigin: func(r *http.Request) bool { return true }` — Cross-Site WebSocket Hijacking. `allowedOrigins` есть в config, но игнорируется. На фронте WS-подключение передаёт `access_token` в URL → попадает в access-логи прокси.

**Шаги — backend.**
1. В `cmd/server/ws.go:36` и `cmd/server/otc_ws.go:32` подменить `CheckOrigin`:
   ```go
   upgrader := websocket.Upgrader{
       CheckOrigin: func(r *http.Request) bool {
           origin := r.Header.Get("Origin")
           for _, a := range cfg.AllowedOrigins { if a == origin { return true } }
           return false
       },
   }
   ```
2. Аутентификация WS — через `?ticket=...` (см. этап B, шаг 3). В апгрейдере:
   - Прочитать `ticket` из query.
   - `redis.GETDEL("ws:ticket:" + ticket)` → если возвращает userID → ок, иначе 401.
   - Закрыть соединение, если userID не совпадает с владельцем `:uid` ордера.
3. Старый параметр `access_token` в query на бэке более не принимается (удалить хэндлер).

**Шаги — frontend.**
1. `src/pages/OTCOrderDetail.tsx:229` и `src/pages/AdminOTCOrderDetail.tsx:233` — заменить:
   ```ts
   const { ticket } = await authService.get_ws_ticket();
   const ws = new WebSocket(`${WS_URL}/otc/${uid}?ticket=${ticket}`);
   ```
2. На переподключении — каждый раз новый тикет.

**Acceptance criteria.**
- [ ] WS-апгрейд с Origin `https://attacker.example` → 403.
- [ ] WS-апгрейд с правильным Origin, но без ticket → 401.
- [ ] Тикет одноразовый: повторное использование → 401.
- [ ] Access-логи не содержат `access_token=` ни в URL, ни в headers.

**Риски миграции.** WS-клиенты должны знать новый flow до деплоя бэка. Деплоить **сначала фронт** (двойная поддержка не нужна, т. к. фронт — единственный клиент). Откат — revert.

**Стратегия выкатки.** Атомарный деплой фронта и бэка после этапа B. Прогон staging минимум 24 часа.

**Оценка.** Backend: 6–8 часов. Frontend: 4–6 часов.

**Зависимости.** Этап B (нужен endpoint `/auth/ws-ticket`).

---

### Этап G — Tooling & CI hygiene

**Закрывает:** H-1 + общая гигиена.

**Проблема и риск.** Фронт-ESLint сломан (Node 16 не поддерживает `structuredClone`, требуется 18+). На бэке отсутствуют `golangci-lint`, `staticcheck`, `govulncheck` в CI. Без них следующие этапы могут провозить регрессии.

**Шаги — frontend.**
1. Добавить `.nvmrc` с `20.11.0`.
2. В `package.json` — `"engines": { "node": ">=20.0.0" }`.
3. Обновить Docker-образ фронта на `node:20-alpine`.
4. Обновить CI (GitHub Actions / GitLab CI) — `actions/setup-node@v4` с `node-version-file: .nvmrc`.
5. Прогнать `npm ci && npm run lint`, починить вылезшие ошибки (если будут).

**Шаги — backend.**
1. Добавить `tools.go` с импортами `golangci-lint`, `staticcheck`, `govulncheck`.
2. `Makefile` — цели `make lint-strict`, `make vuln`.
3. CI:
   - `golangci-lint run --timeout=5m`.
   - `staticcheck ./...`.
   - `govulncheck ./...`.
4. Закрыть выявленные находки или явно `//nolint` с обоснованием.

**Acceptance criteria.**
- [ ] `npm run lint` на фронте — exit 0.
- [ ] CI пайплайн на бэке прогоняет `golangci-lint`, `staticcheck`, `govulncheck` и зелёный.
- [ ] Невозможно смерджить PR с сломанным линтером (branch protection rule).

**Риски миграции.** Минимальны. Возможный «всплытий» багов из-за более строгих линтеров — закрыть в рамках этапа.

**Стратегия выкатки.** PR с CI-настройками первым, затем — все остальные этапы под защитой.

**Оценка.** Frontend: 4–6 часов. Backend: 6–8 часов.

**Зависимости.** Должен быть **первым** или одним из первых, чтобы остальные этапы шли через CI.

---

### Этап H — Type safety hygiene (frontend)

**Закрывает:** H-5, H-7, H-8.

**Проблема и риск.** Многократные `as any` обходят систему типов и маскируют рассинхронизацию с бэк-контрактом. `parseInt(v, 0)` использует радикс 0 → авто-детект (`"012"` → 10, `"0x10"` → 16) — скрытая бомба в продакшне.

**Шаги — frontend.**
1. `src/api/services/authService.ts` — расширить `BackendUser`:
   - Добавить поля, которые сейчас читаются через `as any` в `Profile.tsx:32-34,228-234` и `AdminUserProfile.tsx:49,90` (ФИО, KYC-уровень, флаги верификации, telegram_username и т. п.).
2. `src/pages/Profile.tsx:32-34,228-234` — убрать `as any`, опираться на расширенный `BackendUser`.
3. `src/pages/AdminUserProfile.tsx:49,90` — то же.
4. `src/components/wallets/WithdrawModal.tsx:87` — `currency: wallet.currency.code as any` → нормальный union-тип в DTO (`CryptoCurrency | FiatCurrency`).
5. `src/components/admin/CreateRateModal.tsx:86-87` — заменить:
   ```ts
   parseInt(v, 0)  // -> parseInt(v, 10)
   ```
6. Включить ESLint-правило `@typescript-eslint/no-explicit-any` уровня `error` (после фиксов).

**Шаги — backend.** Не требуются.

**Acceptance criteria.**
- [ ] `grep -r "as any" src/` возвращает 0 совпадений (за исключением обоснованных, помеченных `// eslint-disable-next-line` с комментарием).
- [ ] `grep "parseInt(.*, 0)" src/` возвращает 0.
- [ ] ESLint красный, если кто-то снова добавит `as any`.

**Риски миграции.** Минимальны.

**Стратегия выкатки.** Один PR, после A (типы денег уже строки) — иначе придётся переделывать `BackendUser` дважды.

**Оценка.** Frontend: 6–8 часов.

**Зависимости.** Этап A (синхронизация типов после money-миграции).

---

### Этап I — Frontend resilience

**Закрывает:** H-2, H-6, H-9.

**Проблема и риск.** Нет ErrorBoundary — необработанная ошибка в любом компоненте обрушивает SPA в белый экран. `PublicRoute.tsx:14-19` редиректит staff (`super_admin/operator/support`) на `/exchange`, хотя клиентский UI им не нужен — провоцирует loop с `ClientRoute`. Мёртвый `src/store/exchangeStore.ts` со старым endpoint `/exchange/execute` (после миграции на `/exchanges`) — мина для случайного импорта.

**Шаги — frontend.**
1. Добавить `src/components/ErrorBoundary.tsx` (классовый компонент с `componentDidCatch`):
   - Логирование в Sentry/posthog (если подключено) либо в `console.error`.
   - UI fallback с кнопкой «Reload».
   - Обернуть `<App />` в `src/main.tsx`.
2. `src/components/auth/PublicRoute.tsx:14-19`:
   - Для staff-ролей редирект на `/admin/exchanges`, не `/exchange`.
   - Логика: `if (is_authenticated && user.role === "client") → /exchange; else if (staffRole) → /admin/exchanges; else children`.
3. Удалить `src/store/exchangeStore.ts`:
   - `grep -r "from.*exchangeStore" src/` — найти использования.
   - Перенести оставшуюся логику в `src/api/services/exchangeService.ts` (если есть полезный код) или удалить целиком.
   - Обновить импорты, прогнать `npm run build`.

**Шаги — backend.** Не требуются.

**Acceptance criteria.**
- [ ] Намеренный throw в любом page-компоненте не приводит к белому экрану (виден fallback ErrorBoundary).
- [ ] `super_admin` логин → редирект сразу на `/admin/exchanges`, без посещения `/exchange`.
- [ ] `grep -r "exchangeStore" src/` возвращает 0 совпадений.

**Риски миграции.** Удаление мёртвого стора может вскрыть скрытые импорты — `tsc -b` поймает.

**Стратегия выкатки.** Один PR, можно параллельно с A/B/H.

**Оценка.** Frontend: 6–8 часов.

**Зависимости.** Нет.

---

### Этап J — WebAuthn / 2FA polishing

**Закрывает:** CRIT-5, H-10.

**Проблема и риск.** `internal/service/twofa_service.go:134-171` (`VerifyOTP`) не проверяет, что `payload.UserID` совпадает с владельцем OTP-сессии. Через flow forgot-password (`internal/service/auth_service.go:332-348`) можно ввести чужой телефон, подобрать OTP и захватить аккаунт. На фронте `src/api/services/twoFactorService.ts:108` отправляет `session_id` и `temp_token` в query string POST-запроса — попадает в access-логи.

**Шаги — backend.**
1. `internal/service/twofa_service.go:134-171` — в `VerifyOTP`:
   - Хранить `userID` в Redis-payload OTP-сессии (`otp:session:{id}` → `{userID, code, attempts, exp}`).
   - В верификации требовать совпадение `payload.UserID` с сохранённым `userID`. Несовпадение → 403, инкремент `attempts`, при `attempts >= 5` — invalidate.
2. `internal/service/auth_service.go:332-348` — flow forgot-password передаёт явный `userID` (выясняемый по телефону) в OTP-сессию; recovery — только для конкретного `userID`.
3. Тесты: попытка `VerifyOTP` с чужим userID при правильном code → 403.

**Шаги — frontend.**
1. `src/api/services/twoFactorService.ts:108` — переместить `session_id` и `temp_token` из query string в JSON body POST-запроса.
   - Бэкенд должен принимать оба варианта временно (один спринт), затем — только body.

**Acceptance criteria.**
- [ ] Атака: пользователь A знает OTP пользователя B (ситуативно) — `VerifyOTP` с `payload.UserID = A` и code от B возвращает 403.
- [ ] Access-логи фронт-прокси не содержат `session_id=` или `temp_token=` в URL.
- [ ] Forgot-password восстанавливает только аккаунт, привязанный к указанному телефону, и не даёт его «перепривязать» к другому userID.

**Риски миграции.**
- Кратковременная двойная совместимость (query + body) на бэке — ок, удалить через спринт.
- Старые активные OTP-сессии после деплоя инвалидируются (формат payload меняется). Митигация: TTL OTP — 5 минут, выкатка в окно низкой нагрузки.

**Стратегия выкатки.** Бэк + фронт согласованно, бэк сначала с обратной совместимостью.

**Оценка.** Backend: 6–8 часов. Frontend: 2–3 часа.

**Зависимости.** Нет (но желательно после Этапа G — линтеры в CI).

---

## 5. Порядок выполнения и зависимости

```
G (Tooling/CI)  ──┐
                  ├─►  E (Webhook)        [быстрая победа, низкий риск]
                  │
                  ├─►  D (Rate-limit)     [независим]
                  │
                  ├─►  A (Money)  ──►  C (OTC race)  ──►  H (Type safety)
                  │                  ──►  I (Resilience)
                  │
                  ├─►  B (Auth tokens)  ──►  F (WS Origin/ticket)
                  │
                  └─►  J (2FA)
```

**Рекомендуемый порядок (последовательно по неделям, при двух разработчиках — параллельно):**

| Неделя | Backend | Frontend |
|---|---|---|
| 1 | G (CI), E (Webhook) | G (Node 20, ESLint), I (ErrorBoundary, PublicRoute, мёртвый store) |
| 2 | A (Money) | A (Money) |
| 3 | C (OTC race), B (Auth) — старт | B (Auth tokens) |
| 4 | B (доделать), F (WS origin/ticket) | F (ws-ticket), H (типы) |
| 5 | D (Rate-limit, alerts), J (2FA) | J (2FA), регрессионное тестирование |

**Параллелизация**: G/E/I — независимы и могут идти первой неделей в три PR. A — самая объёмная, лучше выделить отдельную неделю обоим разработчикам. F не стартовать до завершения B.

**Критический путь**: G → A → C → H. Минимальное календарное время — 4 недели при двух full-time разработчиках.

---

## 6. План тестирования

### 6.1. Обязательные unit/integration тесты (новые)
- `internal/service/wallet_service_test.go` — race-test на 100 параллельных `LockAmount` (этап C).
- `internal/service/otc_service_test.go` — атомарность `AcceptOffer`, `ConfirmPayment` (этап C).
- `internal/service/money_test.go` — золотые тесты на BTC 8-знаков, USDT 2-знака, KZT 2-знака (этап A).
- `internal/api/webhook/cryptogate_handler_test.go` — `verifySecret` constant-time + fail-closed (этап E).
- `internal/api/middleware/ratelimit_test.go` — лимит 10/15min работает, `Retry-After` корректен (этап D).
- `internal/service/twofa_service_test.go` — `VerifyOTP` чужого userID → 403 (этап J).
- Frontend: `src/store/__tests__/authStore.partialize.test.ts` — `pending_2fa_token`, токены не попадают в persist (этап B).
- Frontend: `src/lib/__tests__/money.test.ts` — `Decimal` round-trip без потерь (этап A).

### 6.2. Обязательные E2E (Playwright/Cypress)
- Сценарий: депозит → обмен → withdrawal с проверкой балансов до 8-го знака (этап A).
- Сценарий: вход → logout → попытка использовать старый access токен → 401 (этап B).
- Сценарий: WS подключение к OTC через ticket; повторное использование ticket → 401 (этапы B, F).
- Сценарий: super_admin логин → редирект на `/admin/exchanges` (этап I).
- Сценарий: 11 попыток login с одного IP за 15 минут → 429 (этап D).

### 6.3. Ручные проверки
- DevTools → Application → Local Storage после login — токенов нет (этап B).
- Network tab при WS-подключении — `?ticket=` есть, `?access_token=` нет (этапы B, F).
- Telegram Bot — приходит alert при входе с нового устройства (этап D).
- Production-симуляция: `CRYPTOGATE_WEBHOOK_SECRET=""` → бэк не стартует (этап E).

### 6.4. Регрессия
- Полный smoke-тест всех существующих флоу (login, register, 2FA telegram/passkey, exchange, OTC create/accept/cancel, withdraw, admin-flows).
- Visual regression на ключевых страницах (Exchange, OTCOrderDetail, AdminOTCOrders).

---

## 7. Definition of Done

Глобальный чеклист, при выполнении которого план считается завершённым:

- [ ] Все CRITICAL (CRIT-1..5, C-1..4) закрыты, снапшоты PR'ов прилинкованы.
- [ ] Все HIGH (HIGH-1..6, H-1..10) закрыты.
- [ ] Backend: `go test ./...` зелёный, `golangci-lint`, `staticcheck`, `govulncheck` без находок.
- [ ] Frontend: `npm run build`, `npm run lint`, `npm run test` зелёные на Node 20.
- [ ] CI на main — зелёный 7 дней подряд.
- [ ] E2E-тесты из раздела 6.2 пройдены на staging.
- [ ] Security review подписан тех. лидом и владельцем продукта.
- [ ] Runbook на инциденты обновлён (rate-limit override, ws-ticket revoke, refresh cookie reset).
- [ ] Документация (CLAUDE.md обоих репозиториев, OpenAPI) обновлена.
- [ ] `localStorage` после login не содержит JWT (ручная проверка).
- [ ] Webhook прод-конфиг подтверждён (секрет не пуст, bound к Vault).
- [ ] Telegram-алерт о новом устройстве реально доставляется.
- [ ] Денежные суммы передаются как `string`, точность сохраняется до 8-го знака на всех флоу.
- [ ] Принято решение по релиз-окну, объявлен mandatory re-login для всех пользователей.

---

## 8. Что не входит в этот план

Следующие пункты — MEDIUM или ниже, либо относятся к улучшениям после прод-запуска:

- MEDIUM-улучшения: рефакторинг `cache` слоя на Redis-only (сейчас go-cache + Redis), обновление `chi` до v6 при выходе.
- MEDIUM: миграция на pgx/v5 native (с sqlx).
- MEDIUM: Sentry/observability stack (sentry-go, opentelemetry traces, prometheus metrics) — отдельный proj.
- MEDIUM: добавление CSP headers (`Content-Security-Policy`, `X-Frame-Options`) на фронте — отдельный мини-план.
- MEDIUM: переход с zustand persist на IndexedDB для не-секретных данных.
- MEDIUM: обновление i18next на v24, унификация локалей en/ru/kk (en — canonical, см. CLAUDE.md фронта).
- MEDIUM: feature-flag сервис (LaunchDarkly/Unleash) — для будущих миграций.
- MEDIUM: usability-улучшения админ-панели (сортировка, экспорт фильтров).
- LOW: mocking strategy для unit-тестов сервисов (in-memory fakes уже частично есть).
- LOW: оптимизация bundle size фронта (`framer-motion`, `recharts`).
- Архитектурные улучшения OTC — масштабирование, шардирование, оффлайн-операторы — выходят за рамки security/reliability.

---

## 9. Approval

План считается утверждённым после подписи всех сторон ниже:

- [ ] Backend Lead (`midas_exchange_back`): _<TBD имя>_, дата: ____________
- [ ] Frontend Lead (`midas-exchange-frontend`): _<TBD имя>_, дата: ____________
- [ ] Tech Lead / CTO: _<TBD имя>_, дата: ____________
- [ ] Product Owner: _<TBD имя>_, дата: ____________
- [ ] Security Reviewer: _<TBD имя>_, дата: ____________

> После подписи — план переводится в статус `Approved`, создаются Jira/Linear эпики A–J с упомянутыми оценками, владельцами назначаются подписавшиеся лиды.
