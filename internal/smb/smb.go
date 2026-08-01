package smb

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

// SMBConfig SMB配置
type SMBConfig struct {
	ListenAddr string
	ShareName  string
	SharePath  string
	ReadOnly   bool
	GuestOK    bool
}

// SMBServer SMB服务器
type SMBServer struct {
	config    *SMBConfig
	smbdPath  string
	configPath string
}

// NewSMBServer 创建新的SMB服务器
func NewSMBServer(config *SMBConfig) *SMBServer {
	return &SMBServer{
		config:    config,
		smbdPath:  "/usr/sbin/smbd",
		configPath: "/etc/samba/smb.conf",
	}
}

// GenerateConfig 生成SMB配置文件
func (s *SMBServer) GenerateConfig() error {
	tmpl := `[global]
   workgroup = WORKGROUP
   server string = ConstelFS
   security = user
   map to guest = Bad User
   dns proxy = no

[{{.ShareName}}]
   path = {{.SharePath}}
   browsable = yes
   writable = {{if .ReadOnly}}no{{else}}yes{{end}}
   guest ok = {{if .GuestOK}}yes{{else}}no{{end}}
   create mask = 0644
   directory mask = 0755
   valid users = @constelfs
`

	t, err := template.New("smb").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	configDir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	f, err := os.Create(s.configPath)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	defer f.Close()

	if err := t.Execute(f, s.config); err != nil {
		return fmt.Errorf("生成配置失败: %w", err)
	}

	log.Printf("SMB配置已生成: %s", s.configPath)
	return nil
}

// Start 启动SMB服务器
func (s *SMBServer) Start() error {
	if err := s.GenerateConfig(); err != nil {
		return err
	}

	if err := os.MkdirAll(s.config.SharePath, 0755); err != nil {
		return fmt.Errorf("创建共享目录失败: %w", err)
	}

	cmd := exec.Command(s.smbdPath, "-D", "--no-process-group")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动smbd失败: %w", err)
	}

	log.Printf("SMB服务器已启动，共享: %s (%s)", s.config.ShareName, s.config.SharePath)
	return nil
}

// Stop 停止SMB服务器
func (s *SMBServer) Stop() error {
	cmd := exec.Command("killall", "smbd")
	return cmd.Run()
}

// AddUser 添加SMB用户
func (s *SMBServer) AddUser(username, password string) error {
	cmd := exec.Command("useradd", "-M", "-s", "/sbin/nologin", username)
	if err := cmd.Run(); err != nil {
		// 用户可能已存在，忽略错误
	}

	cmd = exec.Command("smbpasswd", "-a", "-s", username)
	cmd.Stdin = bytes.NewBufferString(fmt.Sprintf("%s\n%s\n", password, password))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("设置SMB密码失败: %w", err)
	}

	log.Printf("SMB用户已添加: %s", username)
	return nil
}

// RemoveUser 移除SMB用户
func (s *SMBServer) RemoveUser(username string) error {
	cmd := exec.Command("smbpasswd", "-x", username)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("移除SMB用户失败: %w", err)
	}

	log.Printf("SMB用户已移除: %s", username)
	return nil
}

// GetSharePath 获取共享路径
func (s *SMBServer) GetSharePath() string {
	return s.config.SharePath
}

// SetSharePath 设置共享路径
func (s *SMBServer) SetSharePath(path string) {
	s.config.SharePath = path
}

// IsRunning 检查SMB服务器是否运行
func (s *SMBServer) IsRunning() bool {
	cmd := exec.Command("pgrep", "smbd")
	return cmd.Run() == nil
}
