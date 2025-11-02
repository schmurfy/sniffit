package clickhouse

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/schmurfy/sniffit/models"
	"github.com/schmurfy/sniffit/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClickHouseStore_Integration(t *testing.T) {

	// Setup
	opts := &Options{
		Addr:     []string{"127.0.0.1:9000"},
		Database: "sniffit_test",
		Username: "default",
		Password: "",
		TTL:      24 * time.Hour,
	}

	// Create test database before initializing store
	if err := createTestDatabase(opts); err != nil {
		t.Skipf("Skipping test, cannot create database: %v", err)
		return
	}
	defer dropTestDatabase(opts)

	chStore, err := New(opts)
	if err != nil {
		t.Skipf("Skipping test, cannot connect to ClickHouse: %v", err)
		return
	}
	defer chStore.Close()

	ctx := context.Background()

	// Verify connection
	err = chStore.Ping(ctx)
	require.NoError(t, err, "Should be able to ping ClickHouse")

	t.Run("StoreAndRetrievePackets", func(t *testing.T) {
		// Create test packets with unique IDs per test
		packets := make([]*models.Packet, 3)
		baseID := time.Now().UnixNano()
		for i := range 3 {
			packets[i] = &models.Packet{
				Id:            fmt.Sprintf("store_retrieve_%d_%d", baseID, i),
				Data:          createSimplePacketData(),
				Timestamp:     time.Now().Add(time.Duration(-i) * time.Minute),
				CaptureLength: 64,
				DataLength:    64,
			}
		}

		// Store packets
		err := chStore.StorePackets(ctx, packets)
		require.NoError(t, err, "Should store packets without error")

		// Retrieve packets
		ids := []string{packets[0].Id, packets[1].Id, packets[2].Id}
		retrieved, err := chStore.GetPackets(ctx, ids, nil)
		require.NoError(t, err, "Should retrieve packets without error")
		assert.Len(t, retrieved, 3, "Should retrieve all 3 packets")

		// Verify packet data - results may not be in order
		retrievedIds := make(map[string]*models.Packet)
		for _, pkt := range retrieved {
			retrievedIds[pkt.Id] = pkt
		}
		for _, original := range packets {
			assert.Contains(t, retrievedIds, original.Id)
			pkt := retrievedIds[original.Id]
			assert.Equal(t, original.CaptureLength, pkt.CaptureLength)
			assert.Equal(t, original.DataLength, pkt.DataLength)
			// Verify IPs were extracted and stored
			assert.NotEmpty(t, pkt.SrcIP, "SrcIP should be populated")
			assert.NotEmpty(t, pkt.DstIP, "DstIP should be populated")
		}
	})

	t.Run("GetPacketsWithQuery", func(t *testing.T) {
		// Create test packets with unique IDs per test
		packets := make([]*models.Packet, 3)
		baseID := time.Now().UnixNano()
		now := time.Now()
		packets[0] = &models.Packet{
			Id:            fmt.Sprintf("query_test_%d_0", baseID),
			Data:          createSimplePacketData(),
			Timestamp:     now.Add(-2 * time.Hour),
			CaptureLength: 64,
			DataLength:    64,
		}
		packets[1] = &models.Packet{
			Id:            fmt.Sprintf("query_test_%d_1", baseID),
			Data:          createSimplePacketData(),
			Timestamp:     now.Add(-30 * time.Minute),
			CaptureLength: 64,
			DataLength:    64,
		}
		packets[2] = &models.Packet{
			Id:            fmt.Sprintf("query_test_%d_2", baseID),
			Data:          createSimplePacketData(),
			Timestamp:     now.Add(1 * time.Minute),
			CaptureLength: 64,
			DataLength:    64,
		}

		err := chStore.StorePackets(ctx, packets)
		require.NoError(t, err)

		// Query with time range that should only include packets 1 and 2
		query := store.FindQuery{
			From:     now.Add(-45 * time.Minute),
			To:       now.Add(5 * time.Minute),
			MaxCount: 10,
		}

		ids := []string{packets[0].Id, packets[1].Id, packets[2].Id}
		retrieved, err := chStore.GetPackets(ctx, ids, &query)
		require.NoError(t, err)

		// Should only get packets within the time range (packets 1 and 2)
		assert.Equal(t, 2, len(retrieved), "Should filter by time range and return 2 packets")
	})

	t.Run("FindByAddress", func(t *testing.T) {
		// Create packets with known IP addresses and unique IDs
		srcIP := net.ParseIP("192.168.1.100")
		dstIP := net.ParseIP("192.168.1.200")
		baseID := time.Now().UnixNano()

		packets := make([]*models.Packet, 1)
		data := createPacketDataWithIPs(srcIP, dstIP)
		packets[0] = &models.Packet{
			Id:            fmt.Sprintf("index_test_%d", baseID),
			Data:          data,
			Timestamp:     time.Now(),
			CaptureLength: int64(len(data)),
			DataLength:    int64(len(data)),
		}

		// Store packets
		err := chStore.StorePackets(ctx, packets)
		require.NoError(t, err, "Should store packets without error")

		// Find packets by source IP
		foundIds, err := chStore.FindPacketsByAddress(ctx, srcIP)
		require.NoError(t, err, "Should find packets by address")
		assert.GreaterOrEqual(t, len(foundIds), 1, "Should find at least one packet")

		// Find packets by destination IP
		foundIds, err = chStore.FindPacketsByAddress(ctx, dstIP)
		require.NoError(t, err, "Should find packets by destination address")
		assert.GreaterOrEqual(t, len(foundIds), 1, "Should find at least one packet by destination IP")
	})

	t.Run("IndexKeys", func(t *testing.T) {
		// Create and store some packets with unique IDs
		srcIP := net.ParseIP("10.0.0.1")
		dstIP := net.ParseIP("10.0.0.2")
		baseID := time.Now().UnixNano()

		packets := make([]*models.Packet, 2)
		for i := range 2 {
			data := createPacketDataWithIPs(srcIP, dstIP)
			packets[i] = &models.Packet{
				Id:            fmt.Sprintf("indexkeys_test_%d_%d", baseID, i),
				Data:          data,
				Timestamp:     time.Now(),
				CaptureLength: int64(len(data)),
				DataLength:    int64(len(data)),
			}
		}

		err := chStore.StorePackets(ctx, packets)
		require.NoError(t, err)

		// Get index keys
		keys, err := chStore.IndexKeys(ctx)
		require.NoError(t, err, "Should get index keys without error")
		assert.Greater(t, len(keys), 0, "Should have at least one index key")
	})

	t.Run("DataKeys", func(t *testing.T) {
		// Store some new packets with unique IDs
		packets := make([]*models.Packet, 2)
		baseID := time.Now().UnixNano()
		for i := range 2 {
			packets[i] = &models.Packet{
				Id:            fmt.Sprintf("datakeys_test_%d_%d", baseID, i),
				Data:          createSimplePacketData(),
				Timestamp:     time.Now(),
				CaptureLength: 64,
				DataLength:    64,
			}
		}
		err := chStore.StorePackets(ctx, packets)
		require.NoError(t, err)

		// Get data keys
		keys, err := chStore.DataKeys(ctx)
		require.NoError(t, err, "Should get data keys without error")
		assert.Greater(t, len(keys), 0, "Should have at least one data key")

		// Verify our stored packets are in the keys
		keySet := make(map[string]bool)
		for _, key := range keys {
			keySet[key] = true
		}
		for _, pkt := range packets {
			assert.True(t, keySet[pkt.Id], fmt.Sprintf("Packet %s should be in data keys", pkt.Id))
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		stats, err := chStore.GetStats()
		require.NoError(t, err, "Should get stats without error")
		assert.NotNil(t, stats, "Stats should not be nil")

		// Check for expected stat keys
		assert.Contains(t, *stats, "packet_count")
		assert.Contains(t, *stats, "unique_ips")
		assert.Contains(t, *stats, "unique_src_ips")
		assert.Contains(t, *stats, "unique_dst_ips")
	})
}

// Helper function to create simple packet data
func createSimplePacketData() []byte {
	// Create a simple Ethernet frame with IPv4
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x0a, 0x95, 0x9d, 0x68, 0x16},
		DstMAC:       net.HardwareAddr{0x00, 0x0a, 0x95, 0x9d, 0x68, 0x17},
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    net.ParseIP("192.168.1.1").To4(),
		DstIP:    net.ParseIP("192.168.1.2").To4(),
	}

	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
		Seq:     1,
	}
	tcp.SetNetworkLayerForChecksum(ip)

	gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload([]byte("test")))
	return buf.Bytes()
}

// Helper to create packet data with specific IPs
func createPacketDataWithIPs(srcIP, dstIP net.IP) []byte {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{}

	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x0a, 0x95, 0x9d, 0x68, 0x16},
		DstMAC:       net.HardwareAddr{0x00, 0x0a, 0x95, 0x9d, 0x68, 0x17},
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP.To4(),
		DstIP:    dstIP.To4(),
	}

	tcp := &layers.TCP{
		SrcPort: 12345,
		DstPort: 80,
		Seq:     1,
	}
	tcp.SetNetworkLayerForChecksum(ip)

	gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload([]byte("test")))
	return buf.Bytes()
}

// TestClickHouseStore_Interface verifies that ClickHouseStore implements StoreInterface
func TestClickHouseStore_Interface(t *testing.T) {
	var _ store.StoreInterface = (*ClickHouseStore)(nil)
}

// createTestDatabase creates a test database in ClickHouse
func createTestDatabase(opts *Options) error {
	// Connect without specifying database
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: opts.Addr,
		Auth: clickhouse.Auth{
			Database: "default",
			Username: opts.Username,
			Password: opts.Password,
		},
		DialTimeout: 5 * time.Second,
		// Debug:       true,
		// Debugf: func(format string, v ...interface{}) {
		// 	fmt.Printf("[LOG] "+format+"\n", v...)
		// },
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()

	// Create the test database
	err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", opts.Database))
	return err
}

// dropTestDatabase drops the test database
func dropTestDatabase(opts *Options) error {
	// Connect without specifying database
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: opts.Addr,
		Auth: clickhouse.Auth{
			Database: "default",
			Username: opts.Username,
			Password: opts.Password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()

	// Drop the test database
	err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", opts.Database))
	return err
}
