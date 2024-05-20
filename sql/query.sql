WITH LatestROI AS (
    SELECT
        root_user_id,
        strategy_id,
        roi,
        pnl,
        ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) as rn
    FROM
        bts.roi
)
   , UserCalculations AS (
    SELECT
        l.root_user_id AS user_id,
        SUM(l.pnl) AS total_pnl,
        SUM(l.pnl / NULLIF(l.roi, 0)) AS total_original_input_money
    FROM
        LatestROI l
    WHERE
        l.rn = 1
    GROUP BY
        l.root_user_id
)
SELECT
    user_id,
    total_pnl,
    total_original_input_money
FROM
    UserCalculations
ORDER BY
    total_pnl DESC
LIMIT 30;


SELECT user_id, COUNT(*) AS strategy_count
FROM bts.strategy
GROUP BY user_id
ORDER BY strategy_count DESC
LIMIT 3;

SELECT
    s.*,
    r.roi as latest_roi,
    r.pnl as latest_pnl,
    r.time as latest_roi_time
FROM
    bts.strategy s
        LEFT JOIN (
        SELECT
            roi.strategy_id,
            roi.roi,
            roi.pnl,
            roi.time,
            ROW_NUMBER() OVER (PARTITION BY roi.strategy_id ORDER BY roi.time DESC) AS rn
        FROM
            bts.roi
    ) r ON s.strategy_id = r.strategy_id AND r.rn = 1
WHERE
    (s.concluded = FALSE OR s.concluded IS NULL) ORDER BY s.time_discovered;

SELECT
    r.time AS latest_roi_time,
    s.rois_fetched_at
FROM
    bts.strategy s
        JOIN
    bts.roi r ON s.strategy_id = r.strategy_id
WHERE
    s.concluded = TRUE
ORDER BY
    r.time DESC
LIMIT 10;


select count(*) FROM strategy WHERE concluded = TRUE AND strategy_type = 2;

INSERT INTO bts.roi (strategy_id, roi, pnl, time) VALUES (391829570, 0.1, 100, '2020-01-01 00:00:00');

SELECT * FROM strategy ORDER BY rois_fetched_at LIMIT 10;



CREATE OR REPLACE VIEW TheChosen AS WITH LatestRoi AS (
    SELECT
        root_user_id,
        strategy_id,
        roi as roi,
        pnl,
        time,
        ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi
), EarliestRoi AS (
    SELECT
        strategy_id,
        time,
        ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time) AS rn
    FROM
        bts.roi
),
     FilteredStrategies AS (
         SELECT
             l.root_user_id,
             l.strategy_id,
             l.roi,
             l.pnl,
             l.pnl / NULLIF(l.roi, 0) as original_input,
             EXTRACT(EPOCH FROM (l.time - e.time)) as runtime,
             s.concluded
         FROM
             LatestRoi l
         JOIN
            EarliestRoi e ON l.strategy_id = e.strategy_id
         JOIN strategy s ON l.strategy_id = s.strategy_id
         WHERE
             l.rn = 1 AND e.rn = 1 AND (l.roi > 0.01 OR l.roi < -0.01) AND s.strategy_type = 2
     ),
     UserOriginalInputs AS (
         SELECT
             f.root_user_id,
             SUM(f.original_input) AS total_original_input,  -- Calculating original input and summing it per user
             SUM(f.pnl) AS total_pnl,
             AVG(f.roi) AS avg_roi,
             MAX(f.roi) AS max_roi,
             MIN(f.roi) AS min_roi,
             SUM(f.pnl) / SUM(f.original_input) AS total_roi,
             AVG(f.runtime) AS avg_runtime,
             MAX(f.runtime) AS max_runtime,
             MIN(f.runtime) AS min_runtime,
             COUNT(*) AS strategy_count,
             COUNT(f.concluded) AS concluded_count
         FROM
             FilteredStrategies f
         WHERE
             f.runtime > 9000 AND f.original_input > 498
         GROUP BY
             f.root_user_id
     )
SELECT
    u.*
FROM
    UserOriginalInputs u
WHERE u.total_original_input >= 8500 AND strategy_count >= 3 AND min_roi >= 0.015 AND total_roi >= 0.04
ORDER BY
    total_roi DESC;

SELECT * FROM TheChosen;

SELECT * FROm strategy WHERE user_id=215777793;
SELECT * FROM strategy WHERE user_id = 215777793 AND (concluded IS NULL OR concluded = false);

CREATE OR REPLACE VIEW ThePool AS WITH Pool AS (
    SELECT
        strategy.*, TheChosen.total_roi, TheChosen.total_original_input,
        TheChosen.strategy_count
    FROM strategy
        JOIN TheChosen ON strategy.user_id = TheChosen.root_user_id
    WHERE (concluded IS NULL OR concluded = false) AND strategy_type = 2
), LatestRoi AS (
    SELECT
        r.root_user_id,
        r.strategy_id,
        r.roi as roi,
        r.pnl,
        r.time,
        ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi r
    JOIN Pool ON Pool.strategy_id = r.strategy_id
), EarliestRoi AS (
    SELECT
        r.strategy_id,
        r.time,
        ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time) AS rn
    FROM
        bts.roi r
    JOIN Pool ON Pool.strategy_id = r.strategy_id
),
     FilteredStrategies AS (
         SELECT
             l.root_user_id,
             l.strategy_id,
             l.roi,
             l.pnl,
             l.pnl / NULLIF(l.roi, 0) as original_input,
             EXTRACT(EPOCH FROM (l.time - e.time)) as runtime
         FROM
             LatestRoi l
                 JOIN
             EarliestRoi e ON l.strategy_id = e.strategy_id
         WHERE
             l.rn = 1 AND e.rn = 1
     )
SELECT
    f.roi as roi, f.pnl as pnl, f.original_input, f.runtime as running_time, p.strategy_count,
    p.total_roi, p.total_original_input,
    p.symbol, p.copy_count, p.strategy_id, p.strategy_type, p.direction, p.time_discovered,
    p.user_id, p.price_difference, p.rois_fetched_at, p.type, p.lower_limit, p.upper_limit,
    p.grid_count, p.trigger_price, p.stop_lower_limit, p.stop_upper_limit, p.base_asset, p.quote_asset,
    p.leverage, p.trailing_down, p.trailing_up, p.trailing_type, p.latest_matched_count, p.matched_count, p.min_investment,
    p.concluded
    FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id
        WHERE f.original_input > 498
        ORDER BY p.total_roi DESC;

SELECT * FROM ThePool;

-- TODO: min possible roi in any strategy whose original input is greater than 500

WITH LatestRoi AS (
    SELECT
        root_user_id,
        strategy_id,
        roi as roi,
        pnl,
        time,
        ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi
    WHERE
        root_user_id = 215777793
),
     EarliestRoi AS (
         SELECT
             strategy_id,
             time,
             ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time) AS rn
         FROM
             bts.roi
         WHERE
             root_user_id = 215777793
     ),
     FilteredStrategies AS (
         SELECT
             l.strategy_id,
             l.roi,
             l.pnl,
             l.time,
             EXTRACT(EPOCH FROM (l.time - e.time)) as runtime
         FROM
             LatestRoi l
         JOIN
             EarliestRoi e ON l.strategy_id = e.strategy_id
         WHERE
             l.rn = 1 AND e.rn = 1
     )
SELECT f.*,
       f.pnl / NULLIF(f.roi, 0) AS original_input,
       s.symbol,
       s.time_discovered,
       s.strategy_type,
       s.direction,
       s.concluded,
       s.leverage
       FROM FilteredStrategies f JOIN strategy s ON f.strategy_id = s.strategy_id ORDER BY f.time DESC;


SELECT * FROM roi WHERE strategy_id=392280445;