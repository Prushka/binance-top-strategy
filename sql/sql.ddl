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
    price_difference     NUMERIC,
    time_discovered      TIMESTAMP WITH TIME ZONE,
    time_not_found       TIMESTAMP WITH TIME ZONE,
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
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
            REFERENCES b_user (user_id)
            ON DELETE CASCADE
            ON UPDATE CASCADE
);


CREATE TABLE roi
(
    root_user_id BIGINT,
    strategy_id  BIGINT,
    roi          NUMERIC,
    pnl          NUMERIC,
    time         TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_strategy
        FOREIGN KEY (strategy_id)
            REFERENCES strategy (strategy_id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT fk_user FOREIGN KEY (root_user_id)
        REFERENCES b_user (user_id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    CONSTRAINT pk_roi PRIMARY KEY (strategy_id, time)
);

SELECT public.create_hypertable('bts.roi', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS roi_pnl_idx ON bts.roi (time, strategy_id);