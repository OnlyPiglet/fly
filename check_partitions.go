package main

import (
	"context"
	"fmt"
	"time"

	"github.com/OnlyPiglet/fly/clickhousetools"
)

func main() {
	fmt.Println("=== ClickHouse Partition Check ===")

	config := &clickhousetools.ClickHouseConfig{
		Addresses: []string{"127.0.0.1:9000"},
		Username:  "default",
		Password:  "",
		Database:  "test_clickhouse_db",
	}

	client, err := clickhousetools.NewClickHouseClient(config)
	if err != nil {
		fmt.Printf("❌ Failed to create ClickHouse client: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("✅ Connected to ClickHouse")

	// 检查所有表
	fmt.Println("\n=== All Tables ===")
	rows, err := client.Query(context.Background(), "SHOW TABLES")
	if err != nil {
		fmt.Printf("❌ Failed to show tables: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			fmt.Printf("❌ Failed to scan table name: %v\n", err)
			continue
		}
		fmt.Printf("📋 Table: %s\n", tableName)
	}

	// 检查分区信息
	fmt.Println("\n=== Partition Information ===")
	
	tables := []string{
		"test_partition_events_by_date",
		"test_partition_events_by_type", 
		"test_partition_large_events",
	}

	for _, table := range tables {
		fmt.Printf("\n🔍 Checking partitions for table: %s\n", table)
		
		// 查询分区信息
		query := fmt.Sprintf(`
			SELECT 
				partition,
				name,
				rows,
				bytes_on_disk,
				modification_time
			FROM system.parts 
			WHERE table = '%s' AND database = 'test_clickhouse_db'
			ORDER BY partition
		`, table)

		partRows, err := client.Query(context.Background(), query)
		if err != nil {
			fmt.Printf("❌ Failed to query partitions for %s: %v\n", table, err)
			continue
		}

		partitionCount := 0
		totalRows := uint64(0)
		
		for partRows.Next() {
			var partition, name string
			var rows, bytesOnDisk uint64
			var modTime time.Time
			
			if err := partRows.Scan(&partition, &name, &rows, &bytesOnDisk, &modTime); err != nil {
				fmt.Printf("❌ Failed to scan partition info: %v\n", err)
				continue
			}
			
			partitionCount++
			totalRows += rows
			fmt.Printf("  📁 Partition: %s, Rows: %d, Size: %d bytes\n", partition, rows, bytesOnDisk)
		}
		partRows.Close()

		if partitionCount == 0 {
			fmt.Printf("  ⚠️  No partitions found for table %s\n", table)
		} else {
			fmt.Printf("  ✅ Found %d partitions with total %d rows\n", partitionCount, totalRows)
		}
	}

	// 检查表结构
	fmt.Println("\n=== Table Structures ===")
	for _, table := range tables {
		fmt.Printf("\n🏗️  Structure for table: %s\n", table)
		
		query := fmt.Sprintf("DESCRIBE TABLE %s", table)
		descRows, err := client.Query(context.Background(), query)
		if err != nil {
			fmt.Printf("❌ Failed to describe table %s: %v\n", table, err)
			continue
		}

		for descRows.Next() {
			var name, type_, defaultType, defaultExpr, comment, codecExpr, ttlExpr string
			
			if err := descRows.Scan(&name, &type_, &defaultType, &defaultExpr, &comment, &codecExpr, &ttlExpr); err != nil {
				fmt.Printf("❌ Failed to scan table structure: %v\n", err)
				continue
			}
			
			fmt.Printf("  📝 %s: %s\n", name, type_)
		}
		descRows.Close()

		// 检查表的创建语句
		fmt.Printf("\n📜 CREATE statement for %s:\n", table)
		createQuery := fmt.Sprintf("SHOW CREATE TABLE %s", table)
		createRows, err := client.Query(context.Background(), createQuery)
		if err != nil {
			fmt.Printf("❌ Failed to show create table %s: %v\n", table, err)
			continue
		}

		for createRows.Next() {
			var createStmt string
			if err := createRows.Scan(&createStmt); err != nil {
				fmt.Printf("❌ Failed to scan create statement: %v\n", err)
				continue
			}
			fmt.Printf("  %s\n", createStmt)
		}
		createRows.Close()
	}

	fmt.Println("\n🎉 Partition check completed!")
}
