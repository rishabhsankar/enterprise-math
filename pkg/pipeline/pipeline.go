// Package pipeline provides a composable computation pipeline that chains
// operations together with automatic type checking, error propagation,
// and intermediate result inspection.
package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// Stage represents a single step in a computation pipeline.
type Stage struct {
	Name      string
	Operation operations.Operation
	Args      []operations.Number
	Transform func(*operations.OperationResult) (*operations.OperationResult, error)
}

// Pipeline chains multiple computation stages together.
type Pipeline struct {
	name   string
	stages []Stage
	logger logging.Logger
	hooks  PipelineHooks
}

// PipelineHooks provides callbacks for pipeline lifecycle events.
type PipelineHooks struct {
	OnStageStart    func(stage Stage, index int)
	OnStageComplete func(stage Stage, index int, result *operations.OperationResult, duration time.Duration)
	OnStageError    func(stage Stage, index int, err error)
	OnPipelineStart func(pipeline *Pipeline)
	OnPipelineEnd   func(pipeline *Pipeline, result *PipelineResult)
}

// PipelineResult holds the final and intermediate results of a pipeline run.
type PipelineResult struct {
	FinalResult        *operations.OperationResult
	IntermediateResults []*StageResult
	TotalDuration      time.Duration
	StagesExecuted     int
	Success            bool
	Error              error
}

// StageResult captures the result of a single pipeline stage.
type StageResult struct {
	StageName string
	StageIndex int
	Result    *operations.OperationResult
	Duration  time.Duration
	Error     error
}

// NewPipeline creates a new computation pipeline.
func NewPipeline(name string, logger logging.Logger) *Pipeline {
	return &Pipeline{
		name:   name,
		stages: make([]Stage, 0),
		logger: logger.WithPrefix("pipeline"),
	}
}

// SetHooks configures lifecycle callbacks for the pipeline.
func (p *Pipeline) SetHooks(hooks PipelineHooks) {
	p.hooks = hooks
}

// AddStage appends a computation stage to the pipeline.
func (p *Pipeline) AddStage(stage Stage) *Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

// AddOperation adds a simple operation stage with fixed arguments.
func (p *Pipeline) AddOperation(name string, op operations.Operation, args ...operations.Number) *Pipeline {
	return p.AddStage(Stage{
		Name:      name,
		Operation: op,
		Args:      args,
	})
}

// AddTransform adds a stage that transforms the previous result.
func (p *Pipeline) AddTransform(name string, transform func(*operations.OperationResult) (*operations.OperationResult, error)) *Pipeline {
	return p.AddStage(Stage{
		Name:      name,
		Transform: transform,
	})
}

// Execute runs all stages in sequence, passing results forward.
func (p *Pipeline) Execute(ctx context.Context, initial operations.Number) (*PipelineResult, error) {
	pipelineStart := time.Now()

	if p.hooks.OnPipelineStart != nil {
		p.hooks.OnPipelineStart(p)
	}

	pipelineResult := &PipelineResult{
		IntermediateResults: make([]*StageResult, 0, len(p.stages)),
	}

	currentResult := &operations.OperationResult{
		Value: initial,
	}

	for i, stage := range p.stages {
		if p.hooks.OnStageStart != nil {
			p.hooks.OnStageStart(stage, i)
		}

		stageStart := time.Now()
		var stageResult *operations.OperationResult
		var err error

		if stage.Transform != nil {
			stageResult, err = stage.Transform(currentResult)
		} else if stage.Operation != nil {
			args := make([]operations.Number, 0, len(stage.Args)+1)
			args = append(args, currentResult.Value)
			args = append(args, stage.Args...)
			stageResult, err = stage.Operation.Execute(ctx, args...)
		} else {
			err = fmt.Errorf("stage %q has no operation or transform", stage.Name)
		}

		stageDuration := time.Since(stageStart)

		sr := &StageResult{
			StageName:  stage.Name,
			StageIndex: i,
			Duration:   stageDuration,
		}

		if err != nil {
			sr.Error = err
			pipelineResult.IntermediateResults = append(pipelineResult.IntermediateResults, sr)
			pipelineResult.StagesExecuted = i + 1
			pipelineResult.Error = fmt.Errorf("stage %q (index %d) failed: %w", stage.Name, i, err)

			if p.hooks.OnStageError != nil {
				p.hooks.OnStageError(stage, i, err)
			}

			p.logger.Error("Pipeline stage failed", map[string]interface{}{
				"pipeline": p.name,
				"stage":    stage.Name,
				"index":    i,
				"error":    err.Error(),
			})

			return pipelineResult, pipelineResult.Error
		}

		sr.Result = stageResult
		pipelineResult.IntermediateResults = append(pipelineResult.IntermediateResults, sr)
		currentResult = stageResult

		if p.hooks.OnStageComplete != nil {
			p.hooks.OnStageComplete(stage, i, stageResult, stageDuration)
		}

		p.logger.Debug("Pipeline stage completed", map[string]interface{}{
			"pipeline": p.name,
			"stage":    stage.Name,
			"index":    i,
			"result":   stageResult.Value.Value,
			"duration": stageDuration.String(),
		})
	}

	pipelineResult.FinalResult = currentResult
	pipelineResult.TotalDuration = time.Since(pipelineStart)
	pipelineResult.StagesExecuted = len(p.stages)
	pipelineResult.Success = true

	if p.hooks.OnPipelineEnd != nil {
		p.hooks.OnPipelineEnd(p, pipelineResult)
	}

	return pipelineResult, nil
}

// Len returns the number of stages in the pipeline.
func (p *Pipeline) Len() int {
	return len(p.stages)
}

// PipelineBuilder provides a fluent API for constructing pipelines.
type PipelineBuilder struct {
	pipeline *Pipeline
	factory  operations.OperationFactory
}

// NewPipelineBuilder creates a builder for constructing pipelines.
func NewPipelineBuilder(name string, factory operations.OperationFactory, logger logging.Logger) *PipelineBuilder {
	return &PipelineBuilder{
		pipeline: NewPipeline(name, logger),
		factory:  factory,
	}
}

// Then adds an operation by name to the pipeline.
func (b *PipelineBuilder) Then(opName string, args ...operations.Number) *PipelineBuilder {
	op, err := b.factory.Create(opName, nil)
	if err != nil {
		b.pipeline.logger.Error("Failed to create operation for pipeline", map[string]interface{}{
			"operation": opName,
			"error":     err.Error(),
		})
		return b
	}
	b.pipeline.AddOperation(opName, op, args...)
	return b
}

// ThenTransform adds a transformation function to the pipeline.
func (b *PipelineBuilder) ThenTransform(name string, fn func(*operations.OperationResult) (*operations.OperationResult, error)) *PipelineBuilder {
	b.pipeline.AddTransform(name, fn)
	return b
}

// WithHooks sets lifecycle hooks on the pipeline.
func (b *PipelineBuilder) WithHooks(hooks PipelineHooks) *PipelineBuilder {
	b.pipeline.SetHooks(hooks)
	return b
}

// Build finalizes and returns the pipeline.
func (b *PipelineBuilder) Build() *Pipeline {
	return b.pipeline
}

// ForkJoinPipeline runs multiple pipelines in parallel and merges results.
type ForkJoinPipeline struct {
	name      string
	pipelines []*Pipeline
	merger    func([]*PipelineResult) (*operations.OperationResult, error)
	logger    logging.Logger
}

// NewForkJoinPipeline creates a parallel pipeline executor.
func NewForkJoinPipeline(name string, pipelines []*Pipeline, merger func([]*PipelineResult) (*operations.OperationResult, error), logger logging.Logger) *ForkJoinPipeline {
	return &ForkJoinPipeline{
		name:      name,
		pipelines: pipelines,
		merger:    merger,
		logger:    logger,
	}
}

// Execute runs all pipelines concurrently and merges results.
func (fjp *ForkJoinPipeline) Execute(ctx context.Context, initial operations.Number) (*PipelineResult, error) {
	start := time.Now()

	type pipelineOutput struct {
		result *PipelineResult
		err    error
		index  int
	}

	results := make(chan pipelineOutput, len(fjp.pipelines))

	for i, p := range fjp.pipelines {
		go func(idx int, pipe *Pipeline) {
			r, err := pipe.Execute(ctx, initial)
			results <- pipelineOutput{result: r, err: err, index: idx}
		}(i, p)
	}

	pipelineResults := make([]*PipelineResult, len(fjp.pipelines))
	for range fjp.pipelines {
		output := <-results
		if output.err != nil {
			return nil, fmt.Errorf("fork pipeline %d failed: %w", output.index, output.err)
		}
		pipelineResults[output.index] = output.result
	}

	merged, err := fjp.merger(pipelineResults)
	if err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	return &PipelineResult{
		FinalResult:    merged,
		TotalDuration:  time.Since(start),
		StagesExecuted: len(fjp.pipelines),
		Success:        true,
	}, nil
}
