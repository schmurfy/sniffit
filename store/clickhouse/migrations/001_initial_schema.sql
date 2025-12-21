CREATE TABLE IF NOT EXISTS packets (
  data Array(UInt8) CODEC(ZSTD),
  received_at DateTime64(9),
  expires_at DateTime,
  src_ip IPv4,
  dst_ip IPv4
)
ENGINE = MergeTree()
ORDER BY (src_ip, dst_ip, received_at)
TTL expires_at;


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
