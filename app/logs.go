package app

import (
	"fmt"
	"os/exec"
)

type LogFlagsConfig struct {
	MigrationsDir string `yaml:"migrations_dir" mapstructure:"migrations_dir"`
}

func (cdbm *CDBM) Logs() error {
	catCmd := exec.Command("cat", cdbm.LogFlags.MigrationsDir+"logs/logs.txt")
	fileBytes, err := catCmd.Output()

	if err != nil {
		return err
	}

	fmt.Printf(string(fileBytes) + "\n")
	return nil
}
