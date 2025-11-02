CREATE TABLE IF NOT EXISTS packets (
    id UInt64,
    data Array(UInt8),                  -- Hex-encoded serialized packet data
)
ENGINE = MergeTree()
ORDER BY (id);


CREATE TABLE IF NOT EXISTS packet_ips (
    packet_id UInt64,
    timestamp DateTime64(9),      -- Nanosecond precision for accurate timing
    ip IPv4,
)
ENGINE = MergeTree()
ORDER BY (timestamp,ip);

-- Optional: Create a materialized view for quick stats
-- CREATE MATERIALIZED VIEW IF NOT EXISTS packet_stats
-- ENGINE = SummingMergeTree()
-- PARTITION BY toYYYYMM(date)
-- ORDER BY (date)
-- AS SELECT
--     toDate(timestamp) AS date,
--     count() AS packet_count,
--     sum(capture_length) AS total_bytes
-- FROM packets
-- GROUP BY date;
