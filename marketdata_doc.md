<img src="https://r2cdn.perplexity.ai/pplx-full-logo-primary-dark%402x.png" style="height:64px;margin-right:32px"/>

# Как лучше организовать таблицы, если я хочу хранить метрики большого количества бумаг? При этом сделать это надо оптимизированно

Для хранения большого количества метрик (например, свечей или тиков для тысяч бумаг) "в лоб" (одна строка = одна минута) не подойдет — это быстро приведет к миллиардам строк, тормозам и огромному потреблению диска.

Вот 3 уровня оптимизации: от "просто хорошо" до "архитектуры для HighLoad".

### Уровень 1. Классика (Partitioning + BRIN)

Если вы хотите использовать "чистый" PostgreSQL без расширений.

Главная проблема временных рядов — индексы становятся огромными и не влезают в RAM.
**Решение:** Дробим таблицу на куски по времени (Partitioning) и используем легковесные индексы (BRIN).

```sql
-- Создаем мастер-таблицу. Сами данные в ней НЕ хранятся.
CREATE TABLE market_candles (
    time TIMESTAMPTZ NOT NULL,
    instrument_uid UUID NOT NULL,
    o NUMERIC(12, 5), -- Open
    h NUMERIC(12, 5), -- High
    l NUMERIC(12, 5), -- Low
    c NUMERIC(12, 5), -- Close
    v BIGINT,         -- Volume
    -- Композитный PK включает время, чтобы работало партиционирование
    PRIMARY KEY (time, instrument_uid)
) PARTITION BY RANGE (time);

-- Создаем партиции (нужно делать заранее скриптом, например, на каждый месяц)
CREATE TABLE market_candles_y2025m01 PARTITION OF market_candles
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE market_candles_y2025m02 PARTITION OF market_candles
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- !! ГЛАВНЫЙ СЕКРЕТ !! --
-- Обычный B-Tree индекс будет весить как сами данные. 
-- Используем BRIN индекс для времени. Он весит копейки и работает, 
-- так как данные мы пишем последовательно по времени.
CREATE INDEX idx_candles_time_brin ON market_candles USING BRIN (time);

-- Для поиска по инструменту внутри месяца нужен обычный индекс
CREATE INDEX idx_candles_uid_btree ON market_candles (instrument_uid);
```


### Уровень 2. Агрегация в массивы (Array Bucketing)

Это **лучший способ** для чистого PostgreSQL сократить количество строк в 1440 раз (для минутных свечей).

**Идея:** Вместо 1440 строк за сутки для одной акции (минутки), мы храним **1 строку** за сутки, а внутри неё — массив цен.
Постгрес умеет сжимать (TOAST) длинные строки, поэтому это займет в 3-5 раз меньше места на диске и будет читаться молниеносно.

```sql
CREATE TABLE market_candles_daily_bucket (
    day_date DATE NOT NULL,
    instrument_uid UUID NOT NULL,
    
    -- Метаданные дневной свечи (для быстрых выборок "по дням")
    day_open NUMERIC(12, 5),
    day_close NUMERIC(12, 5),
    day_volume BIGINT,
    
    -- ВНУТРЕННОСТИ (минутки упакованы в массивы)
    -- prices[^1] = цена в 00:01, prices[^60] = цена в 01:00
    -- Используем REAL[] для экономии (4 байта) или NUMERIC[] для точности
    opens NUMERIC[], 
    highs NUMERIC[],
    lows NUMERIC[],
    closes NUMERIC[],
    volumes BIGINT[],
    
    PRIMARY KEY (instrument_uid, day_date)
);
```

**Плюсы:**

* **Скорость чтения:** Чтобы вытащить график Apple за год, нужно прочитать всего 365 строк (а не 100,000+).
* **Сжатие:** TOAST автоматически сожмет массивы.[^1][^2]

**Минусы:**

* Сложнее обновлять (нужно делать `UPDATE ... SET closes = array_append(...)`).
* Сложнее строить аналитику внутри SQL (нужно разворачивать массив через `unnest`).


### Уровень 3. TimescaleDB (Рекомендую)

Если это учебный проект, лучше всего поставить расширение **TimescaleDB**. Это надстройка над Postgres, которая делает всё вышеописанное за вас автоматически.

Она сама создает партиции (они называют их "chunks"), сама сжимает старые данные в колоночный формат (Columnar Compression), что уменьшает диск на 90%.

```sql
-- Обычная таблица
CREATE TABLE candles (
    time TIMESTAMPTZ NOT NULL,
    instrument_uid UUID NOT NULL,
    price NUMERIC,
    volume BIGINT
);

-- Магия TimescaleDB: превращаем в гипертаблицу
SELECT create_hypertable('candles', 'time');

-- Включаем нативное сжатие (старые данные сжимаются сами)
ALTER TABLE candles SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'instrument_uid'
);
```


### Резюме (Что выбрать?)

1. **Если можно ставить расширения:** Ставьте **TimescaleDB**. Это стандарт индустрии, получите опыт работы с "взрослым" инструментом.[^3][^4]
2. **Если чистый Postgres + важна запись (RT):** Вариант 1 (Partitioning + BRIN).
3. **Если чистый Postgres + важно чтение истории:** Вариант 2 (Array Buckets).

**Совет по типам данных:**
Не используйте `NUMERIC` для всего подряд, он тяжелый.

* Цена: если это учебный проект, часто хватает `DOUBLE PRECISION` (быстрее в разы). Если нужны копейки — храните как `INTEGER` (цена * 100) или `BIGINT`.
* Объем: `INTEGER` или `BIGINT`.
<span style="display:none">[^10][^11][^12][^13][^14][^15][^16][^17][^18][^19][^20][^5][^6][^7][^8][^9]</span>

<div align="center">⁂</div>

[^1]: https://www.tigerdata.com/blog/handling-billions-of-rows-in-postgresql

[^2]: https://stackoverflow.com/questions/9815234/how-to-store-7-3-billion-rows-of-market-data-optimized-to-be-read

[^3]: https://dev.to/pawnsapprentice/postgresql-for-time-series-data-harnessing-temporal-insights-4mha

[^4]: https://blog.nilayparikh.com/analysing-the-best-timeseries-databases-for-financial-and-market-analytics-4f5a26175315

[^5]: https://www.reddit.com/r/PostgreSQL/comments/1h3rhuq/best_practices_for_optimizing_postgresql_rds_for/

[^6]: https://www.alibabacloud.com/blog/postgresql-time-series-best-practices-stock-exchange-system-database_594815

[^7]: https://www.tigerdata.com/learn/postgresql-data-analysis-best-practices

[^8]: https://openmetal.io/resources/blog/how-to-build-a-high-performance-time-series-database-on-openmetal/

[^9]: https://chandel.hashnode.dev/boosting-postgresql-performance-with-brin-indexes

[^10]: https://www.tigerdata.com/learn/best-practices-time-series-data-modeling-single-or-multiple-partitioned-tables-aka-hypertables

[^11]: https://www.cybrosys.com/research-and-development/postgres/how-brin-indexes-in-postgresql-offer-memory-efficient-and-fast-indexing-solutions

[^12]: https://www.domo.com/learn/article/postgresql-for-data-analysis-a-complete-guide

[^13]: https://dev.to/jbranchaud/speeding-up-an-expensive-postgresql-query-b-tree-vs-brin-3cpc

[^14]: https://www.cybertec-postgresql.com/en/postgresql-bulk-loading-huge-amounts-of-data/

[^15]: https://www.cybertec-postgresql.com/en/btree-vs-brin-2-options-for-indexing-in-postgresql-data-warehouses/

[^16]: https://www.reddit.com/r/dataengineering/comments/1gmmg61/best_approach_to_handle_billions_of_data/

[^17]: https://www.postgresql.org/docs/current/indexes-types.html

[^18]: https://www.ashnik.com/unlock-the-power-of-postgresql-a-guide-to-managing-large-datasets/

[^19]: https://learnomate.org/postgresql-indexing-strategies-btree-vs-gin-vs-brin/

[^20]: https://dev.to/tim_huang/query-1b-rows-in-postgresql-25x-faster-with-squirrels-4e01

