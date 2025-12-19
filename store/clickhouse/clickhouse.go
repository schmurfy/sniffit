package clickhouse

import (
	"context"
	"crypto/tls"
	"database/sql"
	"embed"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/bwmarrin/snowflake"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/pkg/errors"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	_tracer = otel.Tracer("clickhouse_store")
)

const (
	PACKET_INSERT     = "INSERT INTO packets (id, data)"
	PACKET_IPS_INSERT = "INSERT INTO packet_ips (packet_id, timestamp, ip)"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ClickHouseStore implements the StoreInterface using ClickHouse as the backend
type ClickHouseStore struct {
	conn        clickhouse.Conn
	ttl         time.Duration
	idGenerator *snowflake.Node
}

// Options contains configuration for ClickHouse store
type Options struct {
	Addr     []string
	Database string
	Username string
	Password string
	TTL      time.Duration
	TLS      *tls.Config
}

var (
	DefaultOptions = Options{
		Addr:     []string{"127.0.0.1:8123"},
		Database: "sniffit",
		Username: "default",
		Password: "",
		TTL:      24 * time.Hour,
	}
)

// New creates a new ClickHouse store instance
func New(o *Options) (*ClickHouseStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: o.Addr,
		Auth: clickhouse.Auth{
			Database: o.Database,
			Username: o.Username,
			Password: o.Password,
		},
		TLS: o.TLS,
		Settings: clickhouse.Settings{
			"max_execution_time": 300,
		},
		DialTimeout:      30 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	generator, err := snowflake.NewNode(1)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ret := &ClickHouseStore{
		conn:        conn,
		ttl:         o.TTL,
		idGenerator: generator,
	}

	// Initialize schema
	if err := ret.initSchema(o); err != nil {
		conn.Close()
		return nil, err
	}

	return ret, nil
}

// initSchema creates the necessary tables by applying migration files
func (c *ClickHouseStore) initSchema(o *Options) error {
	ctx := context.Background()

	// Read all migration files from embedded filesystem
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return errors.Wrap(err, "failed to read migrations directory")
	}

	// Sort migration files to ensure they run in order
	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Apply each migration file
	for _, filename := range migrationFiles {
		if err := c.applyMigration(ctx, filename, o); err != nil {
			return errors.Wrapf(err, "failed to apply migration %s", filename)
		}
	}

	return nil
}

// applyMigration reads and executes a single migration file
func (c *ClickHouseStore) applyMigration(ctx context.Context, filename string, o *Options) error {
	// Read migration file
	content, err := migrationsFS.ReadFile(filepath.Join("migrations", filename))
	if err != nil {
		return errors.WithStack(err)
	}

	sql := string(content)

	// Split SQL into individual statements
	statements := splitSQLStatements(sql)

	// Execute each statement
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		if err := c.conn.Exec(ctx, stmt); err != nil {
			return errors.Wrapf(err, "failed to execute statement: %s", stmt)
		}
	}

	return nil
}

// splitSQLStatements splits a SQL script into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		// Statement ends with semicolon
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	// Add any remaining statement
	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

// StorePackets stores packets in ClickHouse with their source and destination IPs
func (c *ClickHouseStore) StorePackets(ctx context.Context, pkts []*models.Packet) (err error) {
	ctx, span := _tracer.Start(ctx, "StorePackets",
		trace.WithAttributes(
			attribute.Int("request.packets_count", len(pkts)),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	if len(pkts) == 0 {
		return nil
	}

	// packetsBatch, err := c.conn.PrepareBatch(ctx, "INSERT INTO packets (id, data)")
	// if err != nil {
	// 	return errors.WithStack(err)
	// }

	// packetsIpsBatch, err := c.conn.PrepareBatch(ctx, "INSERT INTO packet_ips (packet_id, timestamp, ip)")
	// if err != nil {
	// 	return errors.WithStack(err)
	// }

	ctx = clickhouse.Context(ctx, clickhouse.WithAsync(false))

	for _, pkt := range pkts {
		// expiresAt := pkt.Timestamp.Add(c.ttl)

		packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.Default)
		ipLayer, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		if !ok {
			continue
		}

		// Extract IP addresses from packet data if not already set
		srcIP := ipLayer.SrcIP
		dstIP := ipLayer.DstIP

		numericId := c.idGenerator.Generate()

		err = c.conn.Exec(ctx, PACKET_INSERT, uint64(numericId), pkt.Data)
		if err != nil {
			return errors.WithStack(err)
		}

		// create ip records pointing to the packet

		err = c.conn.Exec(ctx, PACKET_IPS_INSERT, uint64(numericId), pkt.Timestamp, srcIP)
		if err != nil {
			return errors.WithStack(err)
		}

		err = c.conn.Exec(ctx, PACKET_IPS_INSERT, uint64(numericId), pkt.Timestamp, dstIP)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (c *ClickHouseStore) GetPacketsByAddress(ctx context.Context, ip net.IP, q *store.FindQuery) (pkts []*models.Packet, err error) {
	ctx, span := _tracer.Start(ctx, "GetPacketsByAddress",
		trace.WithAttributes(
			attribute.String("request.ip", ip.String()),
		))
	defer func() {
		if err != nil {
			span.RecordError(err)
		}
		span.End()
	}()

	// Build query
	args := []any{ip.String()}
	query := `
		SELECT p.data, pi.timestamp,
		FROM packet_ips pi
		JOIN packets p ON pi.packet_id = p.id
		WHERE pi.ip = ?
	`

	if q != nil {
		if !q.From.IsZero() {
			query += " AND pi.timestamp >= ?"
			args = append(args, q.From)
		}
		if !q.To.IsZero() {
			query += " AND pi.timestamp <= ?"
			args = append(args, q.To)
		}
	}

	query += " ORDER BY timestamp"
	if q != nil && q.MaxCount > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.MaxCount)
		pkts = make([]*models.Packet, 0, q.MaxCount)
	} else {
		pkts = make([]*models.Packet, 0, 1000)
	}

	fmt.Printf("query: %s\nargs: %v\n", query, args)

	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rows.Close()

	fmt.Printf("after query\n")

	for rows.Next() {
		var pkt models.Packet

		if err := rows.Scan(&pkt.Data, &pkt.Timestamp); err != nil {
			return nil, errors.WithStack(err)
		}

		pkts = append(pkts, &pkt)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.WithStack(err)
	}

	return pkts, nil
}

// GetPackets retrieves packets by their IDs with optional query filters
func (c *ClickHouseStore) GetPackets(ctx context.Context, ids []string, q *store.FindQuery) (pkts []*models.Packet, err error) {
	return
}

// DataKeys returns all packet IDs
func (c *ClickHouseStore) DataKeys(ctx context.Context) (ret []string, err error) {
	ret = []string{}

	rows, err := c.conn.Query(ctx, "SELECT DISTINCT id FROM packets ORDER BY id")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.WithStack(err)
		}
		ret = append(ret, id)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.WithStack(err)
	}

	return ret, nil
}

// IndexPackets is now a no-op since indexing is done in StorePackets
func (c *ClickHouseStore) IndexPackets(ctx context.Context, pkts []*models.Packet) (err error) {
	ctx, span := _tracer.Start(ctx, "IndexPackets",
		trace.WithAttributes(
			attribute.Int("request.packets_count", len(pkts)),
		))
	defer span.End()

	// IP addresses are now stored directly in the packets table during StorePackets
	// This function is kept for interface compatibility but does nothing
	return nil
}

// IndexKeys returns all unique IP addresses from the packets table
func (c *ClickHouseStore) IndexKeys(ctx context.Context) (ret []string, err error) {
	ret = []string{}

	// Get all unique source and destination IPs
	rows, err := c.conn.Query(ctx, `
		SELECT DISTINCT ip FROM (
			SELECT DISTINCT src_ip AS ip FROM packets
			UNION DISTINCT
			SELECT DISTINCT dst_ip AS ip FROM packets
		)
		ORDER BY ip
	`)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, errors.WithStack(err)
		}
		ret = append(ret, ip)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.WithStack(err)
	}

	return ret, nil
}

// FindPacketsByAddress finds all packet IDs associated with a specific IP address
func (c *ClickHouseStore) FindPacketsByAddress(ctx context.Context, ip net.IP) (ret []string, err error) {
	return
}

// GetStats returns statistics about the ClickHouse store
func (c *ClickHouseStore) GetStats() (*store.Stats, error) {
	ctx := context.Background()
	ret := &store.Stats{}

	// Get packet count
	var packetCount uint64
	row := c.conn.QueryRow(ctx, "SELECT count() FROM packets")
	if err := row.Scan(&packetCount); err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	(*ret)["packet_count"] = fmt.Sprintf("%d", packetCount)

	// Get unique source IP count
	var uniqueSrcIPs uint64
	row = c.conn.QueryRow(ctx, "SELECT count(DISTINCT src_ip) FROM packets")
	if err := row.Scan(&uniqueSrcIPs); err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	(*ret)["unique_src_ips"] = fmt.Sprintf("%d", uniqueSrcIPs)

	// Get unique destination IP count
	var uniqueDstIPs uint64
	row = c.conn.QueryRow(ctx, "SELECT count(DISTINCT dst_ip) FROM packets")
	if err := row.Scan(&uniqueDstIPs); err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	(*ret)["unique_dst_ips"] = fmt.Sprintf("%d", uniqueDstIPs)

	// Get total unique IPs
	var uniqueIPs uint64
	row = c.conn.QueryRow(ctx, `
		SELECT count(DISTINCT ip) FROM (
			SELECT DISTINCT src_ip AS ip FROM packets
			UNION DISTINCT
			SELECT DISTINCT dst_ip AS ip FROM packets
		)
	`)
	if err := row.Scan(&uniqueIPs); err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	(*ret)["unique_ips"] = fmt.Sprintf("%d", uniqueIPs)

	// Get disk usage for packets table
	var packetsDiskSize uint64
	row = c.conn.QueryRow(ctx, "SELECT sum(bytes_on_disk) FROM system.parts WHERE database = ? AND table = ? AND active", "sniffit", "packets")
	if err := row.Scan(&packetsDiskSize); err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	(*ret)["packets_disk_size"] = fmt.Sprintf("%d", packetsDiskSize)

	return ret, nil
}

// Close closes the ClickHouse connection
func (c *ClickHouseStore) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ping checks if the connection to ClickHouse is alive
func (c *ClickHouseStore) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}
