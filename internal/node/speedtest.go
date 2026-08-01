package node

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// SpeedTestResult 测速结果
type SpeedTestResult struct {
	UploadSpeed   float64 `json:"upload_speed"`   // Mbps
	DownloadSpeed float64 `json:"download_speed"` // Mbps
	Latency       int64   `json:"latency"`        // ms
	TestTime      time.Time `json:"test_time"`
}

// SpeedTester 测速器
type SpeedTester struct {
	config *Config
}

// NewSpeedTester 创建测速器
func NewSpeedTester(config *Config) *SpeedTester {
	return &SpeedTester{
		config: config,
	}
}

// RunSpeedTest 运行测速
func (st *SpeedTester) RunSpeedTest() (*SpeedTestResult, error) {
	log.Println("开始测速...")

	// 下载测速脚本
	scriptPath := "/tmp/superbench.sh"
	if err := downloadFile(scriptPath, 
		"https://down.vpsaff.net/linux/speedtest/superbench.sh"); err != nil {
		return nil, fmt.Errorf("下载测速脚本失败: %w", err)
	}

	// 运行测速脚本
	cmd := exec.Command("bash", scriptPath, "--speed")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("运行测速脚本失败: %w", err)
	}

	// 解析测速结果
	result, err := parseSpeedTestOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("解析测速结果失败: %w", err)
	}

	log.Printf("测速完成: 上传=%.2f Mbps, 下载=%.2f Mbps, 延迟=%d ms", 
		result.UploadSpeed, result.DownloadSpeed, result.Latency)

	return result, nil
}

// parseSpeedTestOutput 解析测速输出
func parseSpeedTestOutput(output string) (*SpeedTestResult, error) {
	result := &SpeedTestResult{
		TestTime: time.Now(),
	}

	// 解析下载速度
	downloadRegex := regexp.MustCompile(`Download:\s+([\d.]+)\s+Mbps`)
	if matches := downloadRegex.FindStringSubmatch(output); len(matches) > 1 {
		speed, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			result.DownloadSpeed = speed
		}
	}

	// 解析上传速度
	uploadRegex := regexp.MustCompile(`Upload:\s+([\d.]+)\s+Mbps`)
	if matches := uploadRegex.FindStringSubmatch(output); len(matches) > 1 {
		speed, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			result.UploadSpeed = speed
		}
	}

	// 解析延迟
	latencyRegex := regexp.MustCompile(`Idle Latency:\s+([\d.]+)\s+ms`)
	if matches := latencyRegex.FindStringSubmatch(output); len(matches) > 1 {
		latency, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			result.Latency = int64(latency)
		}
	}

	return result, nil
}

// downloadFile 下载文件
func downloadFile(path, url string) error {
	cmd := exec.Command("curl", "-sL", "-o", path, url)
	return cmd.Run()
}
