-- for clean

SET GLOBAL event_scheduler = ON;
DROP EVENT IF EXISTS clean;
DELIMITER $$

CREATE EVENT
    clean ON SCHEDULE EVERY 12 HOUR
DO BEGIN
DELETE FROM search_history
WHERE
    searched_at < (
        ROUND(
            UNIX_TIMESTAMP(CURTIME(4)) * 1000
        ) - 2628288000
    )
    AND user_id IS NULL;

END$$

DELIMITER ;




-- for update top 10

DELIMITER //

DROP PROCEDURE UPDATE_STAT // CREATE PROCEDURE UPDATE_STAT() BEGIN



	DELETE FROM global_stat;


INSERT INTO
global_stat (day_bin)
SELECT bin as day_bin
FROM search_history
WHERE
searched_at >= (
    ROUND(
        UNIX_TIMESTAMP(CURTIME(4)) * 1000
    ) - 86400000
)
GROUP BY bin
ORDER BY COUNT(bin) DESC
LIMIT 10;

INSERT INTO
global_stat (week_bin) (
    SELECT bin as week_bin
    FROM search_history
    WHERE
searched_at >= (
    ROUND(
        UNIX_TIMESTAMP(CURTIME(4)) * 1000
    ) - 604800000
)
GROUP BY bin
ORDER BY
    COUNT(bin) DESC
LIMIT 10
);

INSERT INTO
global_stat (month_bin)
SELECT bin as month_bin
FROM search_history
WHERE
searched_at >= (
    ROUND(
        UNIX_TIMESTAMP(CURTIME(4)) * 1000
    ) - 2628288000
)
GROUP BY bin
ORDER BY COUNT(bin) DESC
LIMIT 10;

END //


DELIMITER ;

-- for update trigger
DELIMITER $$

DROP TRIGGER update_stat$$
CREATE TRIGGER
    update_stat AFTER
INSERT
    ON search_history FOR EACH ROW BEGIN
CALL update_stat();

END$$

DELIMITER ;