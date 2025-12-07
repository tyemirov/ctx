package ctxcmd

import (
	"fmt"

	"github.com/tyemirov/ctx/internal/cli"
	"github.com/tyemirov/ctx/internal/utils"
)

// Run bootstraps the ctx CLI with logging.
func Run() {
	loggerInstance, loggerInitializationError := utils.NewApplicationLogger()
	if loggerInitializationError != nil {
		panic(fmt.Errorf(utils.LoggerInitializationFailedMessageFormat, loggerInitializationError))
	}
	defer loggerInstance.Sync()
	if applicationExecutionError := cli.Execute(); applicationExecutionError != nil {
		loggerInstance.Fatal(utils.ApplicationExecutionFailedMessage + ": " + applicationExecutionError.Error())
	}
}
