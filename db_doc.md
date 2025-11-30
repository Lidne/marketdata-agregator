Я спроектировал схему так, чтобы она демонстрировала **наследование**, **полиморфные связи** и **глубокую вложенность**, свойственную финансовым инструментам.

### Концептуальная модель (ER-диаграмма в тексте)

В центре всего находится различие между **Активом (Asset)** и **Инструментом (Instrument)**.

* **Asset** — это сущность реального мира (компания Apple, Золото, Валюта USD).
* **Instrument** — это контракт, который торгуется на бирже (Акция Apple, Облигация Apple 2025, Фьючерс на золото).
* У одного Актива может быть много Инструментов (1 компания = обычные акции + привилегированные + 10 выпусков облигаций).

***

### 1. Справочники и Базовые сущности

Эти таблицы нормализуют данные и избавляют от дублирования строк.

* **`countries`**: (ISO коды, названия). Используется для `country_of_risk`.
* **`sectors`**: (IT, Energy, Health).
* **`exchanges`**: (MOEX, SPB, NASDAQ). Важно для связи с расписанием торгов.
* **`brands`**: Хранит логотипы, описания брендов. (Связь: `instruments` -> `brands`).


### 2. Ядро: Активы и Инструменты

Здесь используем паттерн "Table-per-Type" (наследование) или просто жесткие связи 1:1, чтобы разделить общие поля и специфику.

#### Таблица `assets` (Активы)

Сущность "что это вообще такое".

* `uid` (PK, UUID): уникальный ID актива (из API `asset_uid`).
* `type`: тип актива (equity, commodity, currency).
* `name`: название (например, "Газпром").
* `country_code` (FK): страна риска.


#### Таблица `instruments` (Реестр инструментов)

Базовая таблица. Все, что торгуется, попадает сюда.

* `uid` (PK, UUID): уникальный ID инструмента.
* `figi`, `ticker`, `isin`, `class_code`: биржевые идентификаторы.
* `asset_uid` (FK -> `assets`): ссылка на базовый актив.
* `name`: торговое название.
* `lot`: размер лота.
* `currency`: валюта расчетов.
* `exchange_id` (FK -> `exchanges`): где торгуется.
* `api_trade_available_flag`: доступность для робота.
* `instrument_type`: дискриминатор (share, bond, future...).


### 3. Специфичные таблицы (Детализация)

Каждая таблица ссылается на `instruments.uid` как на Primary Key и Foreign Key одновременно.

* **`shares`** (Акции):
    * `instrument_uid` (PK/FK).
    * `share_type` (common/preferred).
    * `ipo_date`, `issue_size`.
    * `div_yield_flag`: есть ли дивиденды.
* **`bonds`** (Облигации):
    * `instrument_uid` (PK/FK).
    * `maturity_date`: дата погашения.
    * `nominal`, `initial_nominal`: номинал.
    * `coupon_quantity_per_year`.
    * `amortization_flag`, `floating_coupon_flag`.
* **`derivatives`** (Срочный рынок - Фьючерсы и Опционы):
    * *Интересная реляция:* Дериватив всегда имеет базовый актив, который сам может быть инструментом.
    * `instrument_uid` (PK/FK).
    * `expiration_date`: экспирация.
    * `basic_asset_instrument_uid` (FK -> `instruments`): ссылка на то, чем мы торгуем (например, фьючерс на акцию Сбера).
    * `strike_price` (для опционов).
    * `contract_type` (Put/Call, Future).


### 4. События и "Денежные потоки" (Интересные отношения 1:Many)

Инструменты генерируют денежные потоки. Это идеальное место для хранения исторических данных.

* **`dividends`**:
    * Связь: `share_uid` -> `instruments.uid`.
    * Поля: `record_date`, `payment_date`, `dividend_net` (сумма на 1 бумагу), `yield_value`.
* **`coupons`**:
    * Связь: `bond_uid` -> `instruments.uid`.
    * Поля: `coupon_date`, `coupon_number`, `pay_one_bond`, `coupon_type`.


### 5. Расписания (Сложная связь)

Биржи работают по расписанию. Инструмент привязан к бирже.

* **`trading_schedules`**:
    * `exchange_id` (FK -> `exchanges`).
    * `date`: дата календаря.
    * `start_time`, `end_time`: время работы.
    * `is_trading_day`: булев флаг.
    * *Логика:* Чтобы понять, торгуется ли инструмент сейчас, нужно сделать JOIN `instruments` -> `exchanges` -> `trading_schedules`.

***

### SQL Схема (Пример реализации)

```sql
-- 1. Справочники
CREATE TABLE countries (
    code CHAR(2) PRIMARY KEY, -- RU, US
    name_full VARCHAR(255)
);

CREATE TABLE exchanges (
    id VARCHAR(50) PRIMARY KEY, -- MOEX_EVENING, SPB
    name VARCHAR(100)
);

CREATE TABLE brands (
    uid UUID PRIMARY KEY,
    name VARCHAR(100),
    logo_file VARCHAR(255),
    sector VARCHAR(100)
);

-- 2. Активы
CREATE TABLE assets (
    uid UUID PRIMARY KEY,
    type VARCHAR(50), -- 'equity', 'commodity'
    name VARCHAR(255),
    country_code CHAR(2) REFERENCES countries(code)
);

-- 3. Главный реестр инструментов
CREATE TABLE instruments (
    uid UUID PRIMARY KEY,
    figi VARCHAR(20) UNIQUE,
    ticker VARCHAR(20),
    class_code VARCHAR(20),
    isin VARCHAR(20),
    
    asset_uid UUID REFERENCES assets(uid), -- Связь "Инструмент -> Актив"
    brand_uid UUID REFERENCES brands(uid),
    exchange_id VARCHAR(50) REFERENCES exchanges(id),
    
    name VARCHAR(255),
    lot INTEGER DEFAULT 1,
    currency VARCHAR(3),
    
    instrument_type VARCHAR(20) NOT NULL, -- 'share', 'bond', 'future'
    trading_status VARCHAR(50),
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индекс для быстрого поиска по тикерам
CREATE INDEX idx_instruments_ticker ON instruments(ticker);

-- 4. Специфика: Акции
CREATE TABLE shares (
    instrument_uid UUID PRIMARY KEY REFERENCES instruments(uid),
    share_type VARCHAR(20), -- 'common', 'preferred'
    ipo_date DATE,
    issue_size BIGINT,
    div_yield_flag BOOLEAN
);

-- 5. Специфика: Облигации
CREATE TABLE bonds (
    instrument_uid UUID PRIMARY KEY REFERENCES instruments(uid),
    maturity_date DATE, -- Дата погашения
    nominal NUMERIC(18, 4),
    coupon_quantity_per_year INTEGER,
    floating_coupon_flag BOOLEAN,
    amortization_flag BOOLEAN
);

-- 6. Специфика: Фьючерсы (Рекурсивная связь!)
CREATE TABLE futures (
    instrument_uid UUID PRIMARY KEY REFERENCES instruments(uid),
    expiration_date DATE,
    -- Ссылка на БАЗОВЫЙ инструмент (например, фьючерс на Акцию Газпрома)
    -- Это позволяет строить цепочки зависимостей
    basic_asset_instrument_uid UUID REFERENCES instruments(uid), 
    initial_margin_on_buy NUMERIC(18, 2) -- Гарантийное обеспечение
);

-- 7. Дивиденды (One-to-Many)
CREATE TABLE dividends (
    id SERIAL PRIMARY KEY,
    instrument_uid UUID REFERENCES instruments(uid),
    payment_date DATE,
    declared_date DATE,
    record_date DATE,
    amount NUMERIC(18, 4), -- Выплата на 1 бумагу
    yield_percent NUMERIC(5, 2)
);

-- 8. Купоны (One-to-Many)
CREATE TABLE coupons (
    id SERIAL PRIMARY KEY,
    instrument_uid UUID REFERENCES instruments(uid),
    coupon_date DATE,
    coupon_number INTEGER,
    amount NUMERIC(18, 4),
    coupon_type VARCHAR(50)
);

-- 9. Расписание торгов
CREATE TABLE trading_schedules (
    exchange_id VARCHAR(50) REFERENCES exchanges(id),
    trade_date DATE,
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    is_trading_day BOOLEAN,
    PRIMARY KEY (exchange_id, trade_date)
);
```


### Идеи для запросов (Зачем такая структура?)

1. **"Найти все облигации, у которых выплата купона на следующей неделе":**
`JOIN instruments -> bonds -> coupons`.
2. **"Построить дерево деривативов":**
Используя таблицу `futures` и поле `basic_asset_instrument_uid`, можно найти все фьючерсы, которые зависят от конкретной акции (например, Сбербанка).
3. **"Календарь инвестора":**
Объединить (`UNION`) даты из `dividends`, `coupons` и `bonds.maturity_date` для конкретного портфеля пользователя.
4. **"Умный фильтр":**
Найти все акции IT-сектора (`JOIN brands`), торгуемые на SPB бирже (`JOIN instruments`), которые платят дивиденды (`div_yield_flag = true`).
