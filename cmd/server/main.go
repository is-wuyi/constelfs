package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/is-wuyi/constelfs/internal/server"
)

func main() {
	configPath := flag.String("config", "config/server.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建服务器
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("创建服务器失败: %v", err)
	}

	// 启动HTTP服务
	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	log.Printf("HTTP服务启动在 %s", httpAddr)

	go func() {
		if err := http.ListenAndServe(httpAddr, srv.Router()); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务启动失败: %v", err)
		}
	}()

	// 启动gRPC服务
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	log.Printf("gRPC服务启动在 %s", grpcAddr)

	go func() {
		if err := srv.StartGRPC(grpcAddr); err != nil {
			log.Fatalf("gRPC服务启动失败: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")
	srv.Shutdown()
	log.Println("服务器已关闭")
}
