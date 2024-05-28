CREATE TABLE b_user
(
    user_id BIGINT PRIMARY KEY
);

CREATE TABLE strategy
(
    symbol               VARCHAR(30),
    copy_count           INTEGER,
    roi                  NUMERIC,
    pnl                  NUMERIC,
    running_time         INTEGER,
    strategy_id          BIGINT PRIMARY KEY,
    strategy_type        INTEGER,
    direction            INTEGER,
    user_id              BIGINT,
    time_discovered      TIMESTAMP WITH TIME ZONE,
    rois_fetched_at      TIMESTAMP WITH TIME ZONE,
    type                 VARCHAR(30),
    lower_limit          NUMERIC,
    upper_limit          NUMERIC,
    grid_count           INTEGER,
    trigger_price        NUMERIC,
    stop_lower_limit     NUMERIC,
    stop_upper_limit     NUMERIC,
    base_asset           VARCHAR(30),
    quote_asset          VARCHAR(30),
    leverage             INTEGER,
    trailing_up          BOOLEAN,
    trailing_down        BOOLEAN,
    trailing_type        VARCHAR(30),
    latest_matched_count INTEGER,
    matched_count        INTEGER,
    min_investment       NUMERIC,
    concluded            BOOLEAN,
    start_price          NUMERIC,
    end_price            NUMERIC,
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
            REFERENCES b_user (user_id)
            ON DELETE CASCADE
            ON UPDATE CASCADE
);


CREATE TABLE roi
(
    strategy_id  BIGINT,
    roi          NUMERIC,
    pnl          NUMERIC,
    time         TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_strategy
        FOREIGN KEY (strategy_id)
            REFERENCES strategy (strategy_id)
            ON DELETE CASCADE
            ON UPDATE CASCADE
);

CREATE TABLE config (
    KEY TEXT primary key not null,
    VALUE TEXT
);

SELECT public.create_hypertable('bts.roi', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS roi_pnl_idx ON bts.roi (time, strategy_id);

ALTER TABLE roi SET (
    timescaledb.compress=false,
    timescaledb.compress_segmentby = 'strategy_id'
    );
SELECT public.decompress_chunk(c, true)
FROM public.show_chunks('bts.roi') c;

SELECT public.add_compression_policy('roi', INTERVAL '2 days', if_not_exists => TRUE);
SELECT public.remove_compression_policy('roi');
SELECT * FROM timescaledb_information.jobs;