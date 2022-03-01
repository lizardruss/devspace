package types

import (
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	devspacecontext "github.com/loft-sh/devspace/pkg/devspace/context"
	"github.com/loft-sh/devspace/pkg/devspace/dependency/registry"
	"github.com/loft-sh/devspace/pkg/devspace/devpod"
)

type Pipeline interface {
	// Run runs the main pipeline
	Run(ctx *devspacecontext.Context) error

	// DevPodManager retrieves the used dev pod manager
	DevPodManager() devpod.Manager

	// DependencyRegistry retrieves the dependency registry
	DependencyRegistry() registry.DependencyRegistry

	// Dependencies retrieves the currently created dependencies
	Dependencies() []Pipeline

	// Name retrieves the name of the pipeline
	Name() string

	// WaitDev waits for the dependency dev managers as well current
	// dev pod manager to be finished
	WaitDev()

	// StartNewPipelines starts sub pipelines in this pipeline. It is ensured
	// that each pipeline can only be run once at the same time and otherwise
	// will fail to start.
	StartNewPipelines(ctx *devspacecontext.Context, pipelines []*latest.Pipeline, sequentially bool) error
}