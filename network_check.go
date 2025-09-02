package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("=== Network Connectivity Check ===")

	// 测试不同的地址组合
	addresses := []string{
		"127.0.0.1:19091",
		"127.0.0.1:19092", 
		"127.0.0.1:19093",
		"localhost:19091",
		"localhost:19092",
		"localhost:19093",
		"172.21.52.178:19091",
		"172.21.52.178:19092",
		"172.21.52.178:19093",
	}

	for _, addr := range addresses {
		fmt.Printf("Testing connection to %s... ", addr)
		
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Printf("✅ Success\n")
			conn.Close()
		}
	}

	fmt.Println("\n=== Port Listening Check ===")
	
	// 检查本地监听的端口
	ports := []string{"19091", "19092", "19093"}
	for _, port := range ports {
		fmt.Printf("Checking if port %s is listening... ", port)
		
		listener, err := net.Listen("tcp", ":"+port)
		if err != nil {
			fmt.Printf("❌ Port %s is in use or blocked: %v\n", port, err)
		} else {
			fmt.Printf("✅ Port %s is available\n", port)
			listener.Close()
		}
	}

	fmt.Println("\n=== Docker Host Check ===")
	
	// 检查 Docker 主机地址
	dockerHosts := []string{
		"host.docker.internal:19091",
		"host.docker.internal:19092",
		"host.docker.internal:19093",
	}
	
	for _, addr := range dockerHosts {
		fmt.Printf("Testing Docker host %s... ", addr)
		
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
		} else {
			fmt.Printf("✅ Success\n")
			conn.Close()
		}
	}
}
