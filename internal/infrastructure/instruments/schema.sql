-- Создание таблицы shares
CREATE TABLE IF NOT EXISTS shares (
    figi VARCHAR(255) PRIMARY KEY NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    instrument_group VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Создание таблицы futures
CREATE TABLE IF NOT EXISTS futures (
    figi VARCHAR(255) PRIMARY KEY NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    instrument_group VARCHAR(50),
    min_price_increment DECIMAL(10, 6),
    min_price_increment_amount DECIMAL(10, 6),
    asset_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Создание таблицы currencies
CREATE TABLE IF NOT EXISTS currencies (
    figi VARCHAR(255) PRIMARY KEY NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    instrument_group VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Создание таблицы bonds
CREATE TABLE IF NOT EXISTS bonds (
    figi VARCHAR(255) PRIMARY KEY NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    instrument_group VARCHAR(50),
    nominal DECIMAL(10, 2),
    aci_value DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Создание таблицы etfs
CREATE TABLE IF NOT EXISTS etfs (
    figi VARCHAR(255) PRIMARY KEY NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    instrument_group VARCHAR(50),
    min_price_increment DECIMAL(10, 6),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Создание индексов для оптимизации
CREATE INDEX IF NOT EXISTS idx_shares_ticker ON shares (ticker);

CREATE INDEX IF NOT EXISTS idx_futures_ticker ON futures (ticker);

CREATE INDEX IF NOT EXISTS idx_currencies_ticker ON currencies (ticker);

CREATE INDEX IF NOT EXISTS idx_bonds_ticker ON bonds (ticker);

CREATE INDEX IF NOT EXISTS idx_etfs_ticker ON etfs (ticker);

CREATE INDEX IF NOT EXISTS idx_shares_class_code ON shares (class_code);

CREATE INDEX IF NOT EXISTS idx_shares_deleted_at ON shares (deleted_at);

CREATE INDEX IF NOT EXISTS idx_futures_deleted_at ON futures (deleted_at);

CREATE INDEX IF NOT EXISTS idx_currencies_deleted_at ON currencies (deleted_at);

CREATE INDEX IF NOT EXISTS idx_bonds_deleted_at ON bonds (deleted_at);

CREATE INDEX IF NOT EXISTS idx_etfs_deleted_at ON etfs (deleted_at);