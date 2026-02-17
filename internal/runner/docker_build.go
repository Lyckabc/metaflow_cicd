package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/neunexus/metaflow_cicd/workflow"
)

// DockerBuildPushActivity clones repo, builds Dockerfile, pushes to registry.
// Image: {registry_url}/{service_name}:{tag}, tag = yymmddhhmm
func DockerBuildPushActivity(ctx context.Context, config *workflow.PipelineConfig) (*workflow.DockerBuildResult, error) {
	tmpDir, err := os.MkdirTemp("", "cicd-docker-"+config.ServiceName+"-*")
	if err != nil {
		return &workflow.DockerBuildResult{Stderr: err.Error(), ExitCode: 1, Success: false}, nil
	}
	defer os.RemoveAll(tmpDir)

	// Clone
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	cloneURL := config.RepoURL
	if config.AccessToken != "" {
		cloneURL = injectTokenIntoURL(config.RepoURL, config.AccessToken)
	}
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, cloneURL, tmpDir)
	var cloneOut, cloneErr bytes.Buffer
	cloneCmd.Stdout = &cloneOut
	cloneCmd.Stderr = &cloneErr
	if err := cloneCmd.Run(); err != nil {
		return &workflow.DockerBuildResult{
			Stdout:   cloneOut.String(),
			Stderr:   cloneErr.String() + "\nclone error: " + err.Error(),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	// Tag: yymmddhhmm
	now := time.Now()
	tag := now.Format("0601021504") // yymmddhhmm

	// Registry host (strip scheme)
	registryHost := strings.TrimPrefix(config.RegistryURL, "https://")
	registryHost = strings.TrimPrefix(registryHost, "http://")
	registryHost = strings.TrimSuffix(registryHost, "/")
	if registryHost == "" {
		return &workflow.DockerBuildResult{
			Stderr:   "registry_url is empty in ci_projects",
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	imageRef := fmt.Sprintf("%s/%s:%s-%s", registryHost, config.ServiceName, config.Branch, tag)

	// Docker env: use DOCKER_API_VERSION to avoid "client version too new" when daemon is older
	apiVer := "1.43"
	if v := os.Getenv("DOCKER_API_VERSION"); v != "" {
		apiVer = v
	}
	dockerEnv := append(os.Environ(), "DOCKER_API_VERSION="+apiVer)

	// Docker login
	if config.RegistryID != "" && config.RegistryPassword != "" {
		loginCmd := exec.CommandContext(ctx, "docker", "login", "-u", config.RegistryID, "--password-stdin", registryHost)
		loginCmd.Env = dockerEnv
		loginCmd.Stdin = strings.NewReader(config.RegistryPassword)
		var loginOut, loginErr bytes.Buffer
		loginCmd.Stdout = &loginOut
		loginCmd.Stderr = &loginErr
		if err := loginCmd.Run(); err != nil {
			return &workflow.DockerBuildResult{
				Stdout:   loginOut.String(),
				Stderr:   loginErr.String() + "\ndocker login error: " + err.Error(),
				ExitCode: 1,
				Success:  false,
			}, nil
		}
	}

	// Docker build
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageRef, ".")
	buildCmd.Env = dockerEnv
	buildCmd.Dir = tmpDir
	var buildOut, buildErr bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildErr
	if err := buildCmd.Run(); err != nil {
		return &workflow.DockerBuildResult{
			ImageRef: imageRef,
			Stdout:   buildOut.String(),
			Stderr:   buildErr.String() + "\ndocker build error: " + err.Error(),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	// Docker push
	pushCmd := exec.CommandContext(ctx, "docker", "push", imageRef)
	pushCmd.Env = dockerEnv
	var pushOut, pushErr bytes.Buffer
	pushCmd.Stdout = &pushOut
	pushCmd.Stderr = &pushErr
	if err := pushCmd.Run(); err != nil {
		return &workflow.DockerBuildResult{
			ImageRef: imageRef,
			Stdout:   buildOut.String() + "\n" + pushOut.String(),
			Stderr:   buildErr.String() + "\n" + pushErr.String() + "\ndocker push error: " + err.Error(),
			ExitCode: 1,
			Success:  false,
		}, nil
	}

	return &workflow.DockerBuildResult{
		ImageRef: imageRef,
		Stdout:   buildOut.String() + "\n" + pushOut.String(),
		Stderr:   buildErr.String() + pushErr.String(),
		ExitCode: 0,
		Success:  true,
	}, nil
}
