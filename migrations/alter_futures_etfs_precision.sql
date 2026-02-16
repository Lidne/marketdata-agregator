ALTER TABLE futures
ALTER COLUMN min_price_increment TYPE NUMERIC(20, 8);

ALTER TABLE futures
ALTER COLUMN min_price_increment_amount TYPE NUMERIC(20, 8);

ALTER TABLE etfs
ALTER COLUMN min_price_increment TYPE NUMERIC(20, 8);