# Хранение рыночных данных (Trades / Candles / OrderBook) в TimescaleDB

Ниже приведены SQL-скрипты для создания таблиц рыночных данных (сделки, свечи, стаканы) в стиле TimescaleDB, а также краткая документация по параметрам и основным идеям проектирования.

## Ключевые принципы

- **TimescaleDB**: используем hypertables для временных рядов, чтобы обеспечить эффективную вставку, партиционирование по времени и быстрые запросы по диапазонам времени.
- **Нормализация**: все рыночные данные привязываются к инструментам через `instrument_uid` (FK на `instruments.uid`), а входные поля `figi/ticker/class_code` считаются “дублирующими” и при необходимости сохраняются в `metadata` для трассировки расхождений.
- **Время**: все события приходят с `time` в UTC “по времени биржи”; в БД храним как `TIMESTAMPTZ`.
- **Цена и объем**: во входящем стриме `price` — цена **за 1 инструмент**, а не за лот; чтобы получить стоимость лота, умножаем на `instruments.lot` (лотность хранится в `instruments`).

> Предполагается, что таблица `instruments` уже существует (см. документацию по инструментам) и содержит как минимум: `uid` (PK), `figi` (UNIQUE), `ticker`, `lot`, `class_code`.

---

## Формат входящих данных (стрим)

### OrderBook (пакет стаканов)

Поля:

- `figi` (string): FIGI-идентификатор инструмента.
- `depth` (int32): глубина стакана.
- `bids` (Order[]): массив предложений (покупка).
- `asks` (Order[]): массив спроса (продажа).
- `time` (Timestamp): время формирования стакана, UTC (по времени биржи).
- `uid` (string): UID инструмента.
- `ticker` (string): тикер инструмента.
- `class_code` (string): класс-код (секция торгов).

`Order`:

- `price` (float)
- `quantity` (int64)

### Trade (сделка)

Поля:

- `figi` (string): FIGI-идентификатор инструмента.
- `direction` (int): направление сделки (0/1 — продажа/покупка).
- `price` (float): цена за 1 инструмент (для стоимости лота умножать на `instruments.lot`).
- `quantity` (int64): количество лотов.
- `time` (Timestamp): время сделки, UTC (по времени биржи).
- `uid` (string): UID инструмента.
- `ticker` (string): тикер инструмента.
- `class_code` (string): класс-код (секция торгов).

### Candle (пакет свечей)

Поля:

- `figi` (string): FIGI-идентификатор инструмента.
- `interval` (SubscriptionInterval): интервал свечи (см. тайм-уровни ниже).
- `open/high/low/close` (float): цены за 1 инструмент (для стоимости лота умножать на `instruments.lot`).
- `volume` (int64): объем сделок в лотах.
- `time` (Timestamp): время начала интервала свечи, UTC.
- `last_trade_ts` (Timestamp): время последней сделки, вошедшей в свечу, UTC.
- `uid` (string): UID инструмента.
- `ticker` (string): тикер инструмента.
- `class_code` (string): класс-код (секция торгов).
- `volume_buy` (int64): объем торгов на покупку (в лотах).
- `volume_sell` (int64): объем торгов на продажу (в лотах).

---

## Тайм-уровни свечей

Поддерживаемые таймфреймы (минимально необходимые):

- 1м: `interval_seconds = 60`
- 1ч: `interval_seconds = 3600`
- 1д: `interval_seconds = 86400`

Рекомендуется хранить свечи **ровно в тех интервалах, в которых они приходят** (1м/1ч/1д), чтобы не расходиться с источником и упростить аудит.

---

## SQL-схема

### 1) Trades (сделки)

Назначение: хранение исполненных сделок, включая цену, объем, время и направление.

Особенности:

- `quantity_lots` хранит **количество лотов** из входящего `quantity`.
- `side` получается из `direction`: 0 → SELL, 1 → BUY.
- `metadata` можно использовать для сохранения входных полей `figi/ticker/class_code`, если нужно диагностировать несогласованность справочника.

```sql
CREATE TABLE trades (
trade_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
instrument_uid UUID NOT NULL,

    -- 0/1 из стрима маппится в BUY/SELL на уровне ingestion
    side VARCHAR(4) NOT NULL CHECK (side IN ('BUY','SELL')),

    -- price: цена за 1 инструмент
    price NUMERIC(20, 8) NOT NULL,

    -- quantity: количество лотов (int64 во входе)
    quantity_lots BIGINT NOT NULL,

    traded_at TIMESTAMPTZ NOT NULL,

    metadata JSONB
    );

ALTER TABLE trades
ADD CONSTRAINT fk_trades_instruments
FOREIGN KEY (instrument_uid)
REFERENCES instruments(uid)
ON DELETE CASCADE;

SELECT create_hypertable('trades', 'traded_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_trades_instrument_time
ON trades(instrument_uid, traded_at);

CREATE INDEX IF NOT EXISTS idx_trades_time
ON trades(traded_at);
```

### 2) Candles (свечи)

Назначение: хранение OHLCV-данных по фиксированным интервалам.

Особенности:

- `interval_seconds` хранит один из поддерживаемых таймфреймов: 60/3600/86400.
- `period_start` соответствует входному `time` (время начала интервала).
- `volume_lots`, `volume_buy_lots`, `volume_sell_lots` — объемы **в лотах** из стрима.
- `last_trade_at` соответствует `last_trade_ts`.

```sql
CREATE TABLE candles (
candle_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
instrument_uid UUID NOT NULL,

    interval_seconds BIGINT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,

    open NUMERIC(20, 8) NOT NULL,
    high NUMERIC(20, 8) NOT NULL,
    low  NUMERIC(20, 8) NOT NULL,
    close NUMERIC(20, 8) NOT NULL,

    volume_lots BIGINT NOT NULL,
    volume_buy_lots BIGINT,
    volume_sell_lots BIGINT,

    last_trade_at TIMESTAMPTZ,

    metadata JSONB
    );

ALTER TABLE candles
ADD CONSTRAINT fk_candles_instruments
FOREIGN KEY (instrument_uid)
REFERENCES instruments(uid)
ON DELETE CASCADE;

-- hypertable по времени начала периода
SELECT create_hypertable(
'candles',
'period_start',
chunk_time_interval => INTERVAL '1 day',
if_not_exists => TRUE
);

-- Уникальность свечи в рамках инструмента + таймфрейма + начала интервала
CREATE UNIQUE INDEX IF NOT EXISTS ux_candles_natural
ON candles(instrument_uid, interval_seconds, period_start);

CREATE INDEX IF NOT EXISTS idx_candles_instrument_time
ON candles(instrument_uid, period_start);
```

### 3) OrderBook snapshots (снимки стакана)

Назначение: хранение снимков стакана ликвидности в момент времени на заданной глубине.

Особенности:

- `depth` хранится явно (из входящего `depth`), чтобы различать стаканы разной глубины.
- `bids`/`asks` — JSONB массив объектов `{price, quantity}`, где `quantity` — int64 из стрима.
- Для простоты и скорости вставки используем snapshots + JSONB; при необходимости аналитики по уровням можно добавить отдельную таблицу уровней позже.

```sql
CREATE TABLE order_book_snapshots (
snapshot_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
instrument_uid UUID NOT NULL,

    snapshot_at TIMESTAMPTZ NOT NULL,
    depth INT NOT NULL,

    -- массив уровней: [{"price": 123.45, "quantity": 100}, ...]
    bids JSONB NOT NULL,
    asks JSONB NOT NULL,

    metadata JSONB
    );

ALTER TABLE order_book_snapshots
ADD CONSTRAINT fk_obs_instruments
FOREIGN KEY (instrument_uid)
REFERENCES instruments(uid)
ON DELETE CASCADE;

SELECT create_hypertable('order_book_snapshots', 'snapshot_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_obs_instrument_time
ON order_book_snapshots(instrument_uid, snapshot_at);

-- Часто полезно предотвращать дубли одинакового времени/глубины по инструменту
CREATE UNIQUE INDEX IF NOT EXISTS ux_obs_natural
ON order_book_snapshots(instrument_uid, snapshot_at, depth);
```

---

## Маппинг полей стрима в таблицы (кратко)

### Trade → trades

- `uid` → `instrument_uid`
- `time` → `traded_at`
- `direction` (0/1) → `side` (SELL/BUY)
- `price` → `price` (за 1 инструмент)
- `quantity` → `quantity_lots`
- `figi/ticker/class_code` → опционально в `metadata`

### Candle → candles

- `uid` → `instrument_uid`
- `time` → `period_start`
- `interval` → `interval_seconds` (60/3600/86400)
- `open/high/low/close` → `open/high/low/close`
- `volume` → `volume_lots`
- `volume_buy/volume_sell` → `volume_buy_lots` / `volume_sell_lots`
- `last_trade_ts` → `last_trade_at`
- `figi/ticker/class_code` → опционально в `metadata`

### OrderBook → order_book_snapshots

- `uid` → `instrument_uid`
- `time` → `snapshot_at`
- `depth` → `depth`
- `bids/asks` → `bids/asks` (JSONB массив объектов `{price, quantity}`)
- `figi/ticker/class_code` → опционально в `metadata`

---

## Примечания по производительности и дизайну

- Hypertable для каждой временной сущности обеспечивает быстрые вставки и запросы по времени.
- Для свечей ключ `(instrument_uid, interval_seconds, period_start)` позволяет безопасно делать upsert и исключать дубли.
- Стаканы в JSONB — быстрый ingestion и гибкость по глубине; если потребуется аналитика по уровням (например, cumulative depth), можно добавить отдельную таблицу уровней (нормализованный “level store”).
