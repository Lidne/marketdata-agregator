-- Компании
CREATE TABLE companies (
    uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL
);

-- Секторы
CREATE TABLE sectors (
    uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    volatility INT NOT NULL CHECK (volatility >= 0 AND volatility < 100)
);

-- Страны
CREATE TABLE countries (
    alfa_two CHAR(2) PRIMARY KEY,
    alfa_three CHAR(3) NOT NULL,
    name VARCHAR(255) NOT NULL,
    name_brief VARCHAR(255)
);

-- Бренды
CREATE TABLE brands (
    uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    info TEXT,
    company_uid UUID NOT NULL REFERENCES companies(uid) ON DELETE RESTRICT,
    sector_uid UUID NOT NULL REFERENCES sectors(uid) ON DELETE RESTRICT,
    country_code CHAR(2) NOT NULL REFERENCES countries(alfa_two) ON DELETE RESTRICT
);

CREATE TABLE instruments (
    uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    figi VARCHAR(255) UNIQUE NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    brand_uid UUID NOT NULL REFERENCES brands(uid) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_instruments_ticker ON instruments(ticker);
CREATE INDEX IF NOT EXISTS idx_instruments_figi ON instruments(figi);

-- Акции
CREATE TABLE shares (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE
);

-- Облигации
CREATE TABLE bonds (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    nominal DECIMAL(10, 2),
    aci_value DECIMAL(10, 2)
);

-- Фьючерсы
CREATE TABLE futures (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    min_price_increment DECIMAL(10, 6),
    min_price_increment_amount DECIMAL(10, 6),
    asset_type VARCHAR(20) NOT NULL
);

-- ETF
CREATE TABLE etfs (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    min_price_increment DECIMAL(10, 6)
);

-- Валюты
CREATE TABLE currencies (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE
);

-- Trades

CREATE TABLE trades (
    trade_id UUID DEFAULT gen_random_uuid(),
    instrument_uid UUID NOT NULL,
    side VARCHAR(4) NOT NULL CHECK (side IN ('BUY','SELL')), -- 0/1 = BUY/SELL
    price NUMERIC(20, 8) NOT NULL,
    quantity_lots BIGINT NOT NULL,
    traded_at TIMESTAMPTZ NOT NULL,
    metadata JSONB,

    PRIMARY KEY (trade_id, traded_at)
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

-- Candles

CREATE TABLE candles (
    candle_id UUID DEFAULT gen_random_uuid(),
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

    metadata JSONB,

    PRIMARY KEY (candle_id, period_start)
);

ALTER TABLE candles
ADD CONSTRAINT fk_candles_instruments
FOREIGN KEY (instrument_uid)
REFERENCES instruments(uid)
ON DELETE CASCADE;

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

-- OrderBook

CREATE TABLE order_book_snapshots (
    snapshot_id UUID DEFAULT gen_random_uuid(),
    instrument_uid UUID NOT NULL,

    snapshot_at TIMESTAMPTZ NOT NULL,
    depth INT NOT NULL,

    -- массив уровней: [{"price": 123.45, "quantity": 100}, ...]
    bids JSONB NOT NULL,
    asks JSONB NOT NULL,

    metadata JSONB,

    PRIMARY KEY (snapshot_id, snapshot_at)
);

ALTER TABLE order_book_snapshots
ADD CONSTRAINT fk_obs_instruments
FOREIGN KEY (instrument_uid)
REFERENCES instruments(uid)
ON DELETE CASCADE;

SELECT create_hypertable('order_book_snapshots', 'snapshot_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_obs_instrument_time
ON order_book_snapshots(instrument_uid, snapshot_at);

-- предотвращает дубли одинакового времени/глубины по инструменту
CREATE UNIQUE INDEX IF NOT EXISTS ux_obs_natural
ON order_book_snapshots(instrument_uid, snapshot_at, depth);