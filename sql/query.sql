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



WITH LatestRoi AS (
    SELECT
        root_user_id,
        strategy_id,
        roi,
        pnl,
        time,
        ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi
),
     FilteredStrategies AS (
         SELECT
             l.root_user_id,
             l.strategy_id,
             l.roi,
             l.pnl
         FROM
             LatestRoi l
         WHERE
             l.rn = 1 AND
             l.roi > 0.01  -- Ensuring that the latest ROI is positive
     ),
     UserOriginalInputs AS (
         SELECT
             f.root_user_id,
             SUM(f.pnl / NULLIF(f.roi, 0)) AS total_original_input  -- Calculating original input and summing it per user
         FROM
             FilteredStrategies f
         GROUP BY
             f.root_user_id
         HAVING
             SUM(f.pnl / NULLIF(f.roi, 0)) >= 2000  -- Ensuring total original input is at least 2000
     )
SELECT
    u.root_user_id,
    u.total_original_input
FROM
    UserOriginalInputs u
ORDER BY
    u.total_original_input DESC
LIMIT 10;

SELECT * FROM strategy WHERE user_id = 20139305;

SELECT * FROM roi WHERE strategy_id=391821583;