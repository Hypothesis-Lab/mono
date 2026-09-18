package harness

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-resty/resty/v2"
	"github.com/moby/moby/api/types/container"
	dockernetwork "github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testJWTSecret      = "test-jwt-secret"
	postgresUser       = "postgres"
	postgresPassword   = "postgres"
	postgresDB         = "app"
	appPort            = "8000"
	postgresAlias      = "postgres"
	appAlias           = "app"
	s3Alias            = "seaweedfs"
	s3Bucket           = "my-bucket"
	s3AccessKeyID      = "admin"
	s3SecretKey        = "secret"
	s3Region           = "us-east-1"
	yandexClientID     = "test-yandex-client-id"
	yandexClientSecret = "test-yandex-client-secret"
	yandexToken        = "test-yandex-token"
	generatorURL       = "http://generator:8000"
)

type Env struct {
	Client *resty.Client
	S3     *s3.Client

	baseURL     string
	databaseURL string

	cleanup     func(context.Context)
	cleanupOnce sync.Once
	setupDone   chan struct{}
}

func NewEnv() *Env {
	return &Env{setupDone: make(chan struct{})}
}

func (e *Env) SetupEnvironment(ctx context.Context) error {
	defer close(e.setupDone)

	cleanupFns := []func(context.Context){}
	e.setCleanup(&cleanupFns)

	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	testNetwork, err := network.New(ctx)
	if err != nil {
		return fmt.Errorf("create test network: %w", err)
	}
	cleanupFns = append(cleanupFns, func(cleanupCtx context.Context) { _ = testNetwork.Remove(cleanupCtx) })

	postgresContainer, err := startContainer(ctx, testNetwork.Name, &cleanupFns, "postgres", testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     postgresUser,
			"POSTGRES_PASSWORD": postgresPassword,
			"POSTGRES_DB":       postgresDB,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(2 * time.Minute),
	})
	if err != nil {
		return err
	}

	postgresHost, err := postgresContainer.Host(ctx)
	if err != nil {
		return fmt.Errorf("get postgres host: %w", err)
	}

	postgresMappedPort, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return fmt.Errorf("get postgres mapped port: %w", err)
	}

	e.databaseURL = fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		postgresUser,
		postgresPassword,
		postgresHost,
		postgresMappedPort.Port(),
		postgresDB,
	)

	s3Container, err := startContainer(ctx, testNetwork.Name, &cleanupFns, "seaweedfs", testcontainers.ContainerRequest{
		Image:        "chrislusf/seaweedfs:4.40",
		ExposedPorts: []string{"8333/tcp"},
		Env: map[string]string{
			"AWS_ACCESS_KEY_ID":     s3AccessKeyID,
			"AWS_SECRET_ACCESS_KEY": s3SecretKey,
			"S3_BUCKET":             s3Bucket,
		},
		Cmd:        []string{"server", "-s3"},
		WaitingFor: wait.ForLog("Start Seaweed S3 API Server").WithStartupTimeout(40 * time.Second),
	})
	if err != nil {
		return err
	}

	s3Host, err := s3Container.Host(ctx)
	if err != nil {
		return fmt.Errorf("get s3 host: %w", err)
	}
	if s3Host == "localhost" {
		s3Host = "127.0.0.1"
	}

	s3MappedPort, err := s3Container.MappedPort(ctx, "8333/tcp")
	if err != nil {
		return fmt.Errorf("get s3 mapped port: %w", err)
	}
	s3Endpoint := fmt.Sprintf("http://%s:%s", s3Host, s3MappedPort.Port())
	if err := waitForSeaweedFS(ctx, s3Endpoint); err != nil {
		return err
	}

	e.S3 = s3.New(s3.Options{
		Region:       s3Region,
		BaseEndpoint: aws.String(s3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(s3AccessKeyID, s3SecretKey, ""),
		UsePathStyle: true,
	})

	appContainer, err := startContainer(ctx, testNetwork.Name, &cleanupFns, "app", testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    projectRoot,
			Dockerfile: "Dockerfile",
			KeepImage:  true,
		},
		ExposedPorts: []string{appPort + "/tcp"},
		Env: map[string]string{
			"PORT":                        appPort,
			"DATABASE_URL":                fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", postgresUser, postgresPassword, postgresAlias, postgresDB),
			"JWT_SECRET":                  testJWTSecret,
			"S3_ENDPOINT":                 fmt.Sprintf("http://%s:8333", s3Alias),
			"S3_REGION":                   s3Region,
			"S3_BUCKET":                   s3Bucket,
			"S3_ACCESS_KEY_ID":            s3AccessKeyID,
			"S3_SECRET_ACCESS_KEY":        s3SecretKey,
			"YANDEX_DIRECT_CLIENT_ID":     yandexClientID,
			"YANDEX_DIRECT_CLIENT_SECRET": yandexClientSecret,
			"YANDEX_DIRECT_TOKEN":         yandexToken,
			"GENERATOR_URL":               generatorURL,
		},
		WaitingFor: wait.ForHTTP("/healthz").WithPort(appPort + "/tcp").WithStartupTimeout(10 * time.Second),
	})
	if err != nil {
		return err
	}

	appHost, err := appContainer.Host(ctx)
	if err != nil {
		return fmt.Errorf("get app host: %w", err)
	}

	appMappedPort, err := appContainer.MappedPort(ctx, appPort+"/tcp")
	if err != nil {
		return fmt.Errorf("get app mapped port: %w", err)
	}

	e.baseURL = fmt.Sprintf("http://%s:%s", appHost, appMappedPort.Port())
	e.Client = resty.New().
		SetBaseURL(e.baseURL).
		SetTimeout(15 * time.Second)

	return nil
}

func (e *Env) Cleanup(ctx context.Context) {
	<-e.setupDone

	e.cleanupOnce.Do(func() {
		e.cleanup(ctx)
	})
}

func (e *Env) DatabaseURL() string {
	return e.databaseURL
}

func (e *Env) S3Bucket() string {
	return s3Bucket
}

func (e *Env) setCleanup(cleanupFns *[]func(context.Context)) {
	e.cleanup = func(cleanupCtx context.Context) {
		for i := len(*cleanupFns) - 1; i >= 0; i-- {
			(*cleanupFns)[i](cleanupCtx)
		}
	}
}

func startContainer(ctx context.Context, networkName string, cleanupFns *[]func(context.Context), service string, req testcontainers.ContainerRequest) (testcontainers.Container, error) {
	portBindings, err := loopbackPortBindings(req.ExposedPorts)
	if err != nil {
		return nil, err
	}

	if req.Networks == nil {
		req.Networks = []string{}
	}
	req.Networks = append(req.Networks, networkName)
	if req.NetworkAliases == nil {
		req.NetworkAliases = map[string][]string{}
	}
	req.NetworkAliases[networkName] = append(req.NetworkAliases[networkName], service)
	req.HostConfigModifier = func(hostConfig *container.HostConfig) {
		hostConfig.PortBindings = portBindings
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		if container != nil {
			_ = container.Terminate(ctx)
		}

		return nil, fmt.Errorf("start %s container: %w", service, err)
	}

	*cleanupFns = append(*cleanupFns, func(cleanupCtx context.Context) { _ = container.Terminate(cleanupCtx) })

	return container, nil
}

func loopbackPortBindings(exposedPorts []string) (dockernetwork.PortMap, error) {
	portBindings := make(dockernetwork.PortMap, len(exposedPorts))
	loopback := netip.MustParseAddr("127.0.0.1")
	for _, exposedPort := range exposedPorts {
		portRange, err := dockernetwork.ParsePortRange(exposedPort)
		if err != nil {
			return nil, fmt.Errorf("parse exposed port %q: %w", exposedPort, err)
		}
		for port := range portRange.All() {
			portBindings[port] = []dockernetwork.PortBinding{{HostIP: loopback, HostPort: "0"}}
		}
	}

	return portBindings, nil
}

func waitForSeaweedFS(ctx context.Context, endpoint string) error {
	startedAt := time.Now()
	fmt.Printf("waiting for SeaweedFS at %s\n", startedAt.Format(time.RFC3339Nano))
	defer func() {
		fmt.Printf("finished waiting for SeaweedFS after %s\n", time.Since(startedAt))
	}()

	readyCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	client := http.Client{Timeout: time.Second}
	for {
		request, err := http.NewRequestWithContext(readyCtx, http.MethodGet, endpoint+"/status", nil)
		if err != nil {
			return fmt.Errorf("create seaweedfs status request: %w", err)
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}

		select {
		case <-readyCtx.Done():
			return fmt.Errorf("wait for seaweedfs status: %w", readyCtx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func findProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("find project root from %s: go.mod not found", wd)
		}
	}
}
