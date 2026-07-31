package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/is-wuyi/constelfs/internal/node"
)

func main() {
	configPath := flag.String("config", "config/node.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := node.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建节点代理
	agent, err := node.New(cfg)
	if err != nil {
		log.Fatalf("创建节点代理失败: %v", err)
	}

	// 启动节点代理
	log.Printf("存储节点启动: %s", cfg.NodeID)
	if err := agent.Start(); err != nil {
		log.Fatalf("启动节点代理失败: %v", err)
	}

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭节点代理...")
	agent.Stop()
	log.Println("节点代理已关闭")
}
