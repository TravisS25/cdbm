package app

import (
	"fmt"
	"os/exec"
)

// LogFlagsConfig is config struct used for logging settings for CDBM#Logs function
type LogFlagsConfig struct {
	// LogFile should the the directory
	LogFile string `yaml:"log_file" mapstructure:"log_file"`
}

// Logs function will simply display log information written to log log file
func (cdbm *CDBM) Logs() error {
	catCmd := exec.Command("cat", cdbm.LogFlags.LogFile)
	fileBytes, err := catCmd.Output()

	if err != nil {
		return err
	}

	fmt.Printf(string(fileBytes) + "\n")
	return nil
}
