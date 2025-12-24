package cli

import (
	"github.com/nace/brezno/internal/container"
	"github.com/nace/brezno/internal/system"
	"github.com/nace/brezno/internal/ui"
)

// GlobalContext holds shared resources for all commands
type GlobalContext struct {
	Executor    *system.Executor
	Logger      *ui.Logger
	LoopManager *container.LoopManager
	LUKSManager *container.LUKSManager
	MountMgr    *container.MountManager
	Discovery   *container.Discovery
}

// NewGlobalContext creates a new global context
func NewGlobalContext(verbose, quiet, noColor, debug bool) *GlobalContext {
	executor := system.NewExecutor(debug)
	logger := ui.NewLogger(verbose, quiet, noColor)

	return &GlobalContext{
		Executor:    executor,
		Logger:      logger,
		LoopManager: container.NewLoopManager(executor),
		LUKSManager: container.NewLUKSManager(executor),
		MountMgr:    container.NewMountManager(executor),
		Discovery:   container.NewDiscovery(executor),
	}
}

// CheckDependencies checks for required system commands
func (ctx *GlobalContext) CheckDependencies() error {
	deps := []string{
		"cryptsetup",
		"losetup",
		"mount",
		"umount",
		"dmsetup",
		"df",
	}
	return ctx.Executor.CheckDependencies(deps)
}
