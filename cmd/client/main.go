package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/is-wuyi/constelfs/internal/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	configPath := flag.String("config", "config/client.yaml", "配置文件路径")
	replicas := flag.Int("replicas", 3, "副本数量")
	flag.Parse()

	// 加载配置
	cfg, err := client.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建客户端
	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}

	// 解析命令
	command := os.Args[1]
	switch command {
	case "upload":
		if len(os.Args) < 3 {
			fmt.Println("用法: constelfs-client upload <文件路径> [--replicas=3]")
			os.Exit(1)
		}
		result, err := c.Upload(os.Args[2], *replicas)
		if err != nil {
			log.Fatalf("上传失败: %v", err)
		}
		fmt.Printf("上传成功! 文件ID: %s\n", result.FileID)

	case "download":
		if len(os.Args) < 4 {
			fmt.Println("用法: constelfs-client download <文件ID> <本地路径>")
			os.Exit(1)
		}
		if err := c.Download(os.Args[2], os.Args[3]); err != nil {
			log.Fatalf("下载失败: %v", err)
		}
		fmt.Println("下载成功")

	case "list":
		path := "/"
		if len(os.Args) > 2 {
			path = os.Args[2]
		}
		files, err := c.List(path)
		if err != nil {
			log.Fatalf("列出文件失败: %v", err)
		}
		for _, f := range files {
			fmt.Printf("%s\t%d\t%s\n", f.FileID, f.FileSize, f.FileName)
		}

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("用法: constelfs-client delete <文件ID>")
			os.Exit(1)
		}
		if err := c.Delete(os.Args[2]); err != nil {
			log.Fatalf("删除失败: %v", err)
		}
		fmt.Println("删除成功")

	case "nodes":
		nodes, err := c.ListNodes()
		if err != nil {
			log.Fatalf("获取节点列表失败: %v", err)
		}
		for _, n := range nodes {
			fmt.Printf("%s\t%s\t%s\n", n.NodeID, n.IPAddress, n.Status)
		}

	default:
		fmt.Printf("未知命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("ConstelFS 客户端")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  constelfs-client upload <文件路径> [--replicas=3]")
	fmt.Println("  constelfs-client download <文件ID> <本地路径>")
	fmt.Println("  constelfs-client list [目录路径]")
	fmt.Println("  constelfs-client delete <文件ID>")
	fmt.Println("  constelfs-client nodes")
}
