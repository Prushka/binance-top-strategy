WITH LatestROI AS (SELECT strategy_id,
                          roi,
                          pnl,
                          ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) as rn
                   FROM bts.roi)
   , UserCalculations AS (SELECT SUM(l.pnl)                    AS total_pnl,
                                 SUM(l.pnl / NULLIF(l.roi, 0)) AS total_original_input_money,
                                 s.user_id
                          FROM LatestROI l
                                   JOIN strategy s ON l.strategy_id = s.strategy_id
                          WHERE l.rn = 1
                          GROUP BY s.user_id)
SELECT user_id,
       total_pnl,
       total_original_input_money
FROM UserCalculations
ORDER BY total_pnl DESC
LIMIT 30;


SELECT user_id, COUNT(*) AS strategy_count
FROM bts.strategy
GROUP BY user_id
ORDER BY strategy_count DESC
LIMIT 3;

SELECT s.*,
       r.roi  as latest_roi,
       r.pnl  as latest_pnl,
       r.time as latest_roi_time
FROM bts.strategy s
         LEFT JOIN (SELECT roi.strategy_id,
                           roi.roi,
                           roi.pnl,
                           roi.time,
                           ROW_NUMBER() OVER (PARTITION BY roi.strategy_id ORDER BY roi.time DESC) AS rn
                    FROM bts.roi) r ON s.strategy_id = r.strategy_id AND r.rn = 1
WHERE (s.concluded = FALSE OR s.concluded IS NULL)
ORDER BY s.time_discovered;

SELECT r.time AS latest_roi_time,
       s.rois_fetched_at
FROM bts.strategy s
         JOIN
     bts.roi r ON s.strategy_id = r.strategy_id
WHERE s.concluded = TRUE
ORDER BY r.time DESC
LIMIT 10;

WITH LatestRoi AS (SELECT strategy_id,
                          roi                                                             as roi,
                          pnl,
                          time,
                          ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) AS rn
                   FROM bts.roi),
     EarliestRoi AS (SELECT strategy_id,
                            time,
                            ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time) AS rn
                     FROM bts.roi),
     FilteredStrategies AS (SELECT l.strategy_id,
                                   l.roi,
                                   l.pnl,
                                   l.time,
                                   EXTRACT(EPOCH FROM (l.time - e.time)) as runtime
                            FROM LatestRoi l
                                     JOIN
                                 EarliestRoi e ON l.strategy_id = e.strategy_id
                            WHERE l.rn = 1
                              AND e.rn = 1)
SELECT f.*,
       f.pnl / NULLIF(f.roi, 0) AS original_input,
       s.symbol,
       s.time_discovered,
       s.strategy_type,
       s.direction,
       s.concluded,
       s.leverage
FROM FilteredStrategies f
         JOIN strategy s ON f.strategy_id = s.strategy_id
WHERE user_id = 174742987
ORDER BY f.time DESC;


SELECT *
FROM roi
WHERE strategy_id = 392645954;


WITH LatestRoi AS (SELECT strategy_id,
                          roi                                                             as roi,
                          pnl,
                          time,
                          ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time DESC) AS rn
                   FROM bts.roi),
     EarliestRoi AS (SELECT strategy_id,
                            time,
                            ROW_NUMBER() OVER (PARTITION BY strategy_id ORDER BY time) AS rn
                     FROM bts.roi),
     FilteredStrategies AS (SELECT s.user_id,
                                   l.strategy_id,
                                   l.roi,
                                   l.pnl,
                                   l.pnl / NULLIF(l.roi, 0)              as original_input,
                                   EXTRACT(EPOCH FROM (l.time - e.time)) as runtime,
                                   s.concluded,
                                   e.time                                as earliest_roi_time
                            FROM LatestRoi l
                                     JOIN
                                 EarliestRoi e ON l.strategy_id = e.strategy_id
                                     JOIN strategy s ON l.strategy_id = s.strategy_id
                            WHERE l.rn = 1
                              AND e.rn = 1
                              AND (l.roi >= 0.001 OR l.roi <= -0.001)
                              AND s.strategy_type = 2),
     UserOriginalInputs AS (SELECT f.user_id,
                                   SUM(f.original_input)              AS total_original_input, -- Calculating original input and summing it per user
                                   SUM(f.pnl)                         AS total_pnl,
                                   AVG(f.roi)                         AS avg_roi,
                                   MAX(f.roi)                         AS max_roi,
                                   MIN(f.roi)                         AS min_roi,
                                   SUM(f.pnl) / SUM(f.original_input) AS total_roi,
                                   AVG(f.runtime)                     AS avg_runtime,
                                   MAX(f.runtime)                     AS max_runtime,
                                   MIN(f.runtime)                     AS min_runtime,
                                   COUNT(*)                           AS strategy_count,
                                   COUNT(f.concluded)                 AS concluded_count,
                                   MIN(f.earliest_roi_time)           AS earliest_roi_time
                            FROM FilteredStrategies f
                            WHERE f.runtime >= 10800
                              AND f.original_input > 198
                            GROUP BY f.user_id)
SELECT u.*
FROM UserOriginalInputs u
WHERE u.user_id = 507526257
ORDER BY total_roi DESC;


SELECT COUNT(s.strategy_id)
FROM bts.strategy s
WHERE (s.concluded = FALSE OR s.concluded IS NULL)
  AND strategy_type = 2
  AND rois_fetched_at <= NOW() - INTERVAL '45 minutes';


DROP VIEW ToPopulate;

CREATE OR REPLACE VIEW ToPopulate AS
WITH ACTIVE AS (SELECT *
                FROM bts.strategy s
                WHERE (s.concluded = FALSE OR s.concluded IS NULL)
                  AND strategy_type = 2
                  AND NOT (
                    EXTRACT(HOUR FROM rois_fetched_at) = EXTRACT(HOUR FROM NOW())
                        AND EXTRACT(MINUTE FROM rois_fetched_at) > 30
                        AND rois_fetched_at::date = NOW()::date
                    )
                  AND rois_fetched_at <= NOW() - INTERVAL '5 minutes'),
     LatestRoi AS (SELECT l.strategy_id,
                          l.roi                                                             as roi,
                          l.pnl,
                          l.time,
                          ROW_NUMBER() OVER (PARTITION BY l.strategy_id ORDER BY time DESC) AS rn
                   FROM bts.roi l)
SELECT a.*
FROM ACTIVE a
         LEFT JOIN
     LatestRoi l ON l.strategy_id = a.strategy_id
WHERE ((l.rn = 1 AND NOW() > l.time + interval '70m')
    OR l.rn IS NULL)
ORDER BY l.pnl / NULLIF(l.roi, 0) desc;

SELECT COUNT(*)
FROM strategy WHERE concluded = TRUE and high_price IS NULL and strategy_type=2;


SELECT COUNT(distinct  user_id) FROM TheChosen;

SELECT * FROM ThePool;

SELECT * FROM strategy WHERE user_id=856563456 and concluded is null;

SELECT COUNT(strategy_id), user_id FROM bts.strategy GROUP BY user_id ORDER BY COUNT(strategy_id) DESC LIMIT 15;

SELECT COUNT(*) as negative_changes,l.strategy_id FROM bts.roi l JOIN bts.roi r ON l.strategy_id = r.strategy_id
                                    WHERE l.roi < r.roi AND l.time = r.time + INTERVAL '1 hour' GROUP BY l.strategy_id;


SELECT * FROM strategy ORDER BY time_discovered LIMIT 10;

WITH Pool AS (
    SELECT * FROM bts.strategy WHERE user_id = 900416725 AND concluded=true AND high_price IS NOT NULL AND strategy_type = 2
), LatestRoi AS (
    SELECT
        r.strategy_id,
        r.roi as roi,
        r.pnl,
        r.time,
        ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi r
            JOIN Pool ON Pool.strategy_id = r.strategy_id
), NegativeChange AS (
    SELECT COUNT(*) as negative_changes, l.strategy_id FROM bts.roi l JOIN bts.roi r ON l.strategy_id = r.strategy_id JOIN bts.strategy s on l.strategy_id = s.strategy_id
    WHERE s.user_id = 900416725 AND l.roi < r.roi AND l.time = r.time + INTERVAL '1 hour' GROUP BY l.strategy_id
),
     FilteredStrategies AS (
         SELECT
             l.strategy_id,
             l.roi,
             l.pnl,
             l.pnl / NULLIF(l.roi, 0) as original_input
         FROM
             LatestRoi l
         WHERE
             l.rn = 1
     )SELECT
          f.roi as roi, f.pnl as pnl, f.original_input, COALESCE(n.negative_changes, 0) as negative_changes,
          p.start_time, p.end_time, p.start_price, p.end_price,
          p.high_price, p.low_price,
          p.symbol, p.copy_count, p.strategy_id, p.strategy_type, p.direction, p.time_discovered,
          p.user_id, p.rois_fetched_at, p.type, p.lower_limit, p.upper_limit,
          p.grid_count, p.trigger_price, p.stop_lower_limit, p.stop_upper_limit, p.base_asset, p.quote_asset,
          p.leverage, p.trailing_down, p.trailing_up, p.trailing_type, p.latest_matched_count, p.matched_count, p.min_investment,
          p.concluded
FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id LEFT JOIN NegativeChange n ON n.strategy_id = f.strategy_id
WHERE f.original_input > 349;


SELECT * FROM strategy WHERE user_id=507526257;


SELECT * FROM bts.strategy WHERE user_id=26053825 and concluded is null;

SELECT * FROM thepool WHERE user_id=26053825 and concluded is null ORDER BY time_discovered DESC;