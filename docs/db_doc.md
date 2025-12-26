Я спроектировал схему так, чтобы она демонстрировала **наследование** и **полиморфные связи**, адаптированную под существующие модели Go.

### Концептуальная модель

В центре находится таблица **Инструментов (Instruments)**, содержащая общие поля для всех типов (акции, облигации, фьючерсы и т.д.). Специфичные поля вынесены в отдельные таблицы, связанные с главной таблицей один-к-одному.

### 1. Ядро: Инструменты

Используем паттерн "Table-per-Type" (наследование).

#### Таблица `instruments` (Реестр инструментов)

Базовая таблица (соответствует `BaseModel`). Все, что торгуется, попадает сюда.

- `uid` (PK, UUID): уникальный внутренний ID инструмента (не зависит от внешних источников).
- `figi` (VARCHAR, UNIQUE): идентификатор биржи (Financial Global Identifier).
- `ticker`: тикер.
- `lot`: размер лота.
- `class_code`: код класса.
- `logo_url`: ссылка на логотип.
- `created_at`, `updated_at`, `deleted_at`: временные метки.

### 2. Специфичные таблицы (Детализация)

Каждая таблица ссылается на `instruments.uid` как на Primary Key и Foreign Key одновременно.

- **`shares`** (Акции):
  - `uid` (PK/FK, UUID) -> `instruments.uid`.
  - (В текущей модели специфичные поля отсутствуют, таблица фиксирует тип инструмента).
- **`bonds`** (Облигации):
  - `uid` (PK/FK, UUID) -> `instruments.uid`.
  - `nominal`: номинал.
  - `aci_value`: НКД (Накопленный Купонный Доход).
- **`futures`** (Фьючерсы):
  - `uid` (PK/FK, UUID) -> `instruments.uid`.
  - `min_price_increment`: минимальный шаг цены.
  - `min_price_increment_amount`: стоимость шага цены.
  - `asset_type`: тип базового актива.
- **`etfs`** (ETF):
  - `uid` (PK/FK, UUID) -> `instruments.uid`.
  - `min_price_increment`: минимальный шаг цены.
- **`currencies`** (Валюты):
  - `uid` (PK/FK, UUID) -> `instruments.uid`.

### SQL Схема (Реализация наследования)

```sql
-- 1. Общая таблица инструментов (BaseModel)
CREATE TABLE instruments (
    uid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    figi VARCHAR(255) UNIQUE NOT NULL,
    ticker VARCHAR(50) NOT NULL,
    lot INTEGER NOT NULL,
    class_code VARCHAR(50),
    logo_url VARCHAR,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Индексы для оптимизации
CREATE INDEX idx_instruments_ticker ON instruments(ticker);
CREATE INDEX idx_instruments_figi ON instruments(figi);

-- 2. Специфика: Акции
CREATE TABLE shares (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE
);

-- 3. Специфика: Облигации
CREATE TABLE bonds (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    nominal DECIMAL(10, 2),
    aci_value DECIMAL(10, 2)
);

-- 4. Специфика: Фьючерсы
CREATE TABLE futures (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    min_price_increment DECIMAL(10, 6),
    min_price_increment_amount DECIMAL(10, 6),
    asset_type VARCHAR(20) NOT NULL
);

-- 5. Специфика: ETF
CREATE TABLE etfs (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE,
    min_price_increment DECIMAL(10, 6)
);

-- 6. Специфика: Валюты
CREATE TABLE currencies (
    uid UUID PRIMARY KEY REFERENCES instruments(uid) ON DELETE CASCADE
);
```
