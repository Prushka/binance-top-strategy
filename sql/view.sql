CREATE MATERIALIZED VIEW TheChosen AS
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
                                   s.concluded
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
                                   SUM(f.original_input) / COUNT(*)   AS avg_original_input
                            FROM FilteredStrategies f
                            WHERE f.runtime >= 10800
                              AND f.original_input > 198
                            GROUP BY f.user_id)
SELECT u.*
FROM UserOriginalInputs u
WHERE u.total_original_input >= 8500
  AND strategy_count >= 12
--   AND min_roi >= 0.001
  AND total_roi >= 0.025
  AND avg_original_input >= 500
ORDER BY total_roi DESC;


CREATE MATERIALIZED VIEW ThePool AS
WITH Pool AS (SELECT strategy.*,
                     TheChosen.total_roi,
                     TheChosen.total_original_input,
                     TheChosen.avg_original_input,
                     TheChosen.strategy_count
              FROM strategy
                       JOIN TheChosen ON strategy.user_id = TheChosen.user_id
              WHERE (concluded IS NULL OR concluded = false)
                AND strategy_type = 2),
     LatestRoi AS (SELECT r.strategy_id,
                          r.roi                                                             as roi,
                          r.pnl,
                          r.time,
                          ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time DESC) AS rn
                   FROM bts.roi r
                            JOIN Pool ON Pool.strategy_id = r.strategy_id),
     EarliestRoi AS (SELECT r.strategy_id,
                            r.time,
                            ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time) AS rn
                     FROM bts.roi r
                              JOIN Pool ON Pool.strategy_id = r.strategy_id),
     FilteredStrategies AS (SELECT l.strategy_id,
                                   l.roi,
                                   l.pnl,
                                   l.pnl / NULLIF(l.roi, 0)              as original_input,
                                   EXTRACT(EPOCH FROM (l.time - e.time)) as runtime
                            FROM LatestRoi l
                                     JOIN
                                 EarliestRoi e ON l.strategy_id = e.strategy_id
                            WHERE l.rn = 1
                              AND e.rn = 1)
SELECT f.roi     as roi,
       f.pnl     as pnl,
       f.original_input,
       f.runtime as running_time,
       p.strategy_count,
       p.total_roi,
       p.total_original_input,
       p.symbol,
       p.copy_count,
       p.strategy_id,
       p.strategy_type,
       p.direction,
       p.time_discovered,
       p.user_id,
       p.rois_fetched_at,
       p.type,
       p.lower_limit,
       p.upper_limit,
       p.grid_count,
       p.trigger_price,
       p.stop_lower_limit,
       p.stop_upper_limit,
       p.base_asset,
       p.quote_asset,
       p.leverage,
       p.trailing_down,
       p.trailing_up,
       p.trailing_type,
       p.latest_matched_count,
       p.matched_count,
       p.min_investment,
       p.concluded
FROM FilteredStrategies f
         JOIN Pool p ON f.strategy_id = p.strategy_id
WHERE f.original_input > 998
  AND f.original_input >= p.avg_original_input * 0.7
ORDER BY p.total_roi DESC, f.original_input DESC;


SELECT COUNT(*)
FROM TheChosen;

DROP MATERIALIZED VIEW ThePool;
DROP MATERIALIZED VIEW TheChosen;