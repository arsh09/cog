package cli

import (
	"fmt"
	"path/filepath"
	"os"

	"github.com/spf13/cobra"

	"github.com/replicate/cog/pkg/docker"
	"github.com/replicate/cog/pkg/files"
	"github.com/replicate/cog/pkg/global"
	"github.com/replicate/cog/pkg/logger"
	"github.com/replicate/cog/pkg/model"
	"github.com/replicate/cog/pkg/serving"
)

func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test the model locally",
		RunE:  Test,
		Args:  cobra.NoArgs,
	}
	cmd.Flags().StringP("arch", "a", "cpu", "Test architecture")

	return cmd
}

func Test(cmd *cobra.Command, args []string) error {
	arch, err := cmd.Flags().GetString("arch")
	if err != nil {
		return err
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	logWriter := logger.NewConsoleLogger()

	configPath := filepath.Join(projectDir, global.ConfigFilename)
	exists, err := files.Exists(configPath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s does not exist in %s. Are you in the right directory?", global.ConfigFilename, projectDir)
	}
	configRaw, err := os.ReadFile(filepath.Join(projectDir, global.ConfigFilename))
	if err != nil {
		return fmt.Errorf("Failed to read %s: %w", global.ConfigFilename, err)
	}
	config, err := model.ConfigFromYAML(configRaw)
	if err != nil {
		return err
	}
	if err := config.ValidateAndCompleteConfig(); err != nil {
		return err
	}
	archMap := map[string]bool{}
	for _, confArch := range config.Environment.Architectures {
		archMap[confArch] = true
	}
	if _, ok := archMap[arch]; !ok {
		return fmt.Errorf("Architecture %s is not defined for model", arch)
	}
	generator := &docker.DockerfileGenerator{Config: config, Arch: arch}
	dockerfileContents, err := generator.Generate()
	if err != nil {
		return fmt.Errorf("Failed to generate Dockerfile for %s: %w", arch, err)
	}
	dockerImageBuilder := docker.NewLocalImageBuilder("")
	servingPlatform, err := serving.NewLocalDockerPlatform()
	if err != nil {
		return err
	}
	tag, err := dockerImageBuilder.Build(projectDir, dockerfileContents, "", logWriter)
	if err != nil {
		return fmt.Errorf("Failed to build Docker image: %w", err)
	}

	if _, err := serving.TestModel(servingPlatform, tag, config, projectDir, logWriter); err != nil {
		return err
	}

	return nil
}