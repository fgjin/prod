package processing_image_tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"images_compare_update/global"
	"images_compare_update/internal/create_clients"

	cr "github.com/alibabacloud-go/cr-20181201/v2/client"
	"github.com/alibabacloud-go/tea/tea"

	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// 执行所有镜像任务
func ExecImageTasks(semaphoreCap int, acrRegistry, harbor, instanceId, username string, imageSli []string) {
	clientManager := create_clients.NewClientManager(&create_clients.DefaultClientFactory{})
	aliyunClient := clientManager.GetAliyunClient()
	acrClient := NewAliyunACR(aliyunClient, &DefaultCommandRunner{})
	imageOperator := NewImageOperatorImpl(&DefaultCommandRunner{})

	// 登录 ACR
	if err := acrClient.Login(acrRegistry, username, instanceId); err != nil {
		global.Logger.Fatal("Failed to login to ACR", zap.Error(err))
	}

	// 创建信号量，用于控制并发数量
	sem := semaphore.NewWeighted(int64(semaphoreCap))
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, image := range imageSli {
		wg.Add(1)
		go func(image string) {
			defer wg.Done()
			// 尝试获取信号量
			if err := sem.Acquire(ctx, 1); err != nil {
				global.Logger.Error("Failed to acquire semaphore", zap.Error(err))
				return
			}
			defer sem.Release(1)

			nameSpace := ChooseNamespace(image, harbor)
			targetImage := fmt.Sprintf("%s/%s/%s", acrRegistry, nameSpace, filepath.Base(image))
			// 需要执行的命令
			commands := []Command{
				NewPullImageCommand(imageOperator, image),
				NewEnsureNamespaceCommand(acrClient, nameSpace, instanceId),
				NewTagImageCommand(imageOperator, image, targetImage),
				NewPushImageCommand(imageOperator, targetImage),
			}

			task := NewImageTask(commands)
			if err := task.Execute(); err != nil {
				global.Logger.Error("Failed to process image", zap.String("image", image), zap.Error(err))
				cancel()
			}
		}(image)
	}

	wg.Wait()
}

// Command 接口定义了一个通用的执行方法
type Command interface {
	Execute() error
}

// 拉取镜像的命令
type PullImageCommand struct {
	imageOperator ImageOperator
	image         string
}

func NewPullImageCommand(imageOperator ImageOperator, image string) Command {
	return &PullImageCommand{
		imageOperator: imageOperator,
		image:         image,
	}
}

func (p *PullImageCommand) Execute() error {
	return p.imageOperator.PullImage(p.image)
}

// 确保命名空间存在的命令
type EnsureNamespaceCommand struct {
	imageRepoOperator ImageRepoOperator
	nameSpace         string
	instanceId        string
}

func NewEnsureNamespaceCommand(imageRepoOperator ImageRepoOperator, nameSpace, instanceId string) Command {
	return &EnsureNamespaceCommand{
		imageRepoOperator: imageRepoOperator,
		nameSpace:         nameSpace,
		instanceId:        instanceId,
	}
}

func (e *EnsureNamespaceCommand) Execute() error {
	return e.imageRepoOperator.EnsureNamespaceExists(e.nameSpace, e.instanceId)
}

// 标记镜像的命令
type TagImageCommand struct {
	imageOperator  ImageOperator
	source, target string
}

func NewTagImageCommand(imageOperator ImageOperator, source, target string) Command {
	return &TagImageCommand{
		imageOperator: imageOperator,
		source:        source,
		target:        target,
	}
}

func (t *TagImageCommand) Execute() error {
	return t.imageOperator.TagImage(t.source, t.target)
}

// 推送镜像的命令
type PushImageCommand struct {
	imageOperator ImageOperator
	image         string
}

func NewPushImageCommand(imageOperator ImageOperator, image string) Command {
	return &PushImageCommand{
		imageOperator: imageOperator,
		image:         image,
	}
}

func (p *PushImageCommand) Execute() error {
	return p.imageOperator.PushImage(p.image)
}

// 组合任务
type ImageTask struct {
	commands []Command
}

func NewImageTask(commands []Command) *ImageTask {
	return &ImageTask{
		commands: commands,
	}
}

// 执行任务中的所有命令
func (i *ImageTask) Execute() error {
	for _, cmd := range i.commands {
		if err := cmd.Execute(); err != nil {
			return err
		}
	}
	return nil
}

// Docker 配置文件的结构
type DockerConfig struct {
	Auths       map[string]Auth   `json:"auths,omitempty"`
	HttpHeaders map[string]string `json:"HttpHeaders,omitempty"`
}

// 认证信息
type Auth struct {
	Auth string `json:"auth,omitempty"`
}

// 镜像仓库操作接口
type ImageRepoOperator interface {
	Login(acrPrivateAddr, username, instanceId string) error
	EnsureNamespaceExists(nameSpace, instanceId string) error
}

// 阿里云 ACR 客户端
type AliyunACR struct {
	client         *cr.Client
	namespaceCache sync.Map // 缓存
	runner         CommandRunner
}

func NewAliyunACR(client *cr.Client, runner CommandRunner) *AliyunACR {
	return &AliyunACR{
		client: client,
		runner: runner,
	}
}

// 登录 ACR
func (a *AliyunACR) Login(acrPrivateAddr, username, instanceId string) error {
	if loggedIn, err := a.CheckLogin(acrPrivateAddr, instanceId); err != nil {
		return err
	} else if loggedIn {
		global.Logger.Info("Already logged in to ACR")
		return nil
	}

	password, err := a.GetTemporaryPassword(instanceId)
	if err != nil {
		return err
	}

	return a.runner.RunCommand("sudo", "docker", "login", acrPrivateAddr, "-u", username, "-p", password)
}

// 检查是否已登录到 ACR
func (a *AliyunACR) CheckLogin(acrPrivateAddr, instanceId string) (bool, error) {
	dockerConfig := filepath.Join("/root", ".docker", "config.json")
	output, err := a.runner.RunCommandOutput("sudo", "cat", dockerConfig)
	if err != nil {
		global.Logger.Error("Can't read docker config", zap.Error(err))
		return false, err
	}
	var config DockerConfig
	if err := json.Unmarshal(output, &config); err != nil {
		global.Logger.Error("Can't unmarshal config", zap.Error(err))
		return false, err
	}
	auth, loggedIn := config.Auths[acrPrivateAddr]
	if !loggedIn || auth.Auth == "" {
		return false, nil
	}

	// 验证登录信息是否有效
	valid, err := a.ValidateToken(instanceId)
	if err != nil {
		global.Logger.Error("Failed to validate token", zap.Error(err))
		return false, err
	}

	if !valid {
		global.Logger.Info("Docker login expired, need to re-login")
		return false, nil
	}

	return true, nil
}

// 验证 token 是否有效
func (a *AliyunACR) ValidateToken(instanceId string) (bool, error) {
	request := &cr.GetAuthorizationTokenRequest{InstanceId: tea.String(instanceId)}
	response, err := a.client.GetAuthorizationToken(request)
	if err != nil {
		return false, err
	}
	if tea.StringValue(response.Body.Code) != "success" {
		return false, errors.New("failed to validate token")
	}
	return true, nil
}

// 获取 ACR 临时密码
func (a *AliyunACR) GetTemporaryPassword(instanceId string) (string, error) {
	request := &cr.GetAuthorizationTokenRequest{InstanceId: tea.String(instanceId)}
	response, err := a.client.GetAuthorizationToken(request)
	if err != nil {
		return "", err
	}
	if tea.StringValue(response.Body.Code) != "success" {
		return "", errors.New("failed to get ACR temporary password")
	}
	return tea.StringValue(response.Body.AuthorizationToken), nil
}

// 确保命名空间存在
func (a *AliyunACR) EnsureNamespaceExists(nameSpace, instanceId string) error {
	if cached, ok := a.namespaceCache.Load(nameSpace); ok && cached.(bool) {
		global.Logger.Debug("Namespace already exists in cache", zap.String("namespace", nameSpace))
		return nil
	}

	exists, err := a.CheckNamespaceExists(a.client, nameSpace, instanceId)
	if err != nil {
		return err
	}

	if exists {
		a.namespaceCache.Store(nameSpace, true)
		global.Logger.Debug("Namespace already exists", zap.String("namespace", nameSpace))
		return nil
	}

	if err := a.CreateNamespace(a.client, nameSpace); err != nil {
		return err
	}

	a.namespaceCache.Store(nameSpace, true)
	global.Logger.Info("Namespace created", zap.String("namespace", nameSpace))
	return nil
}

// 检查命名空间是否存在
func (a *AliyunACR) CheckNamespaceExists(client *cr.Client, nameSpace, instanceId string) (bool, error) {
	request := &cr.GetNamespaceRequest{
		NamespaceName: tea.String(nameSpace),
		InstanceId:    tea.String(instanceId),
	}
	response, err := client.GetNamespace(request)
	if err != nil {
		global.Logger.Error("Failed to check namespace", zap.String("namespace", nameSpace), zap.Error(err))
		return false, err
	}
	if response == nil || tea.StringValue(response.Body.Code) == "NAMESPACE_NOT_EXIST" {
		return false, nil
	}
	if tea.StringValue(response.Body.Code) != "success" {
		return false, errors.New("failed to check namespace with error code: " + tea.StringValue(response.Body.Code))
	}
	return true, nil
}

// 创建命名空间
func (a *AliyunACR) CreateNamespace(client *cr.Client, nameSpace string) error {
	request := &cr.CreateNamespaceRequest{
		AutoCreateRepo:  tea.Bool(true),
		DefaultRepoType: tea.String("PRIVATE"),
		NamespaceName:   tea.String(nameSpace),
		InstanceId:      tea.String(global.ConfigStruct.GetInstanceId()),
	}
	_, err := client.CreateNamespace(request)
	return err
}

// 镜像操作接口
type ImageOperator interface {
	PullImage(image string) error
	TagImage(source, target string) error
	PushImage(image string) error
}

// ImageOperatorImpl 实现了 ImageOperator 接口
type ImageOperatorImpl struct {
	runner CommandRunner
}

func NewImageOperatorImpl(runner CommandRunner) *ImageOperatorImpl {
	return &ImageOperatorImpl{runner: runner}
}

// 拉取镜像
func (i *ImageOperatorImpl) PullImage(image string) error {
	return i.runner.RunCommand("sudo", "docker", "pull", image)
}

// 标记镜像
func (i *ImageOperatorImpl) TagImage(source, target string) error {
	return i.runner.RunCommand("sudo", "docker", "tag", source, target)
}

// 推送镜像
func (i *ImageOperatorImpl) PushImage(image string) error {
	return i.runner.RunCommand("sudo", "docker", "push", image)
}

// CommandRunner 是一个接口，用于运行命令
type CommandRunner interface {
	RunCommand(command ...string) error
	RunCommandOutput(command ...string) ([]byte, error)
}

// DefaultCommandRunner 是 CommandRunner 的默认实现
type DefaultCommandRunner struct{}

// 执行命令并返回错误
func (d *DefaultCommandRunner) RunCommand(command ...string) error {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %q failed: %w", strings.Join(command, " "), err)
	}
	return nil
}

// 执行命令并返回输出结果
func (d *DefaultCommandRunner) RunCommandOutput(command ...string) ([]byte, error) {
	cmd := exec.Command(command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command %q failed: %w\nOutput: %s", strings.Join(command, " "), err, string(output))
	}
	return output, nil
}

// 命名空间选择器接口
type NamespaceSelector interface {
	Select(nameSpace string) string
}

// 默认命名空间选择器
type DefaultNamespaceSelector struct {
	namespaceRules map[string]string
}

func NewDefaultNamespaceSelector(rules map[string]string) *DefaultNamespaceSelector {
	return &DefaultNamespaceSelector{namespaceRules: rules}
}

// 根据命名空间选择规则返回匹配的命名空间
func (ns *DefaultNamespaceSelector) Select(nameSpace string) string {
	for key, value := range ns.namespaceRules {
		if strings.Contains(nameSpace, key) {
			return value
		}
	}
	return nameSpace
}

// 根据镜像选择适当的命名空间
func ChooseNamespace(image, harbor string) string {
	if !strings.HasPrefix(image, harbor) {
		return "public"
	}

	splitImageToSli := strings.Split(image, "/")
	nameSpace := splitImageToSli[1]

	rules := map[string]string{
		"idc-h3-core":           "idc",
		"idc-h3-frontend":       "idc",
		"idc-h3-expansion":      "idc",
		"idc-h3-infra":          "idc",
		"idc-h3-public":         "idc",
		"idc-h3-scale":          "idc",
		"idc-h3-shennong":       "idc",
		"idc-h3yun":             "idc",
		"idc-platform-assisted": "idc",
		"idc-h3yun-deploy":      "h3sre",
		"idc-sre-scrapers":      "monitor",
		"monitoring":            "monitor",
		"prometheus-operator":   "monitor",
		"basic":                 "basic",
		"base":                  "basic",
		"elastic-operator":      "basic",
	}

	selector := NewDefaultNamespaceSelector(rules)
	return selector.Select(nameSpace)
}
