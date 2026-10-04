// Package codegen defines the pluggable interface for turning a Figma
// frame (an image + its structured data) into code, mirroring the
// Summarizer pattern from commit-notification-app's internal/claude
// package - same idea, different provider pool (here: vision-capable
// models only, since this task needs to actually see the design).
package codegen

import "context"

// CodeGenerator turns one Figma frame into source code for the requested
// framework (e.g. "react", "html"). image is a PNG render of the frame;
// nodeJSON is Figma's structured document for that node (layout, text,
// styles) included so the model isn't guessing everything from pixels alone.
type CodeGenerator interface {
	GenerateCode(ctx context.Context, image []byte, nodeJSON, framework string) (string, error)
}
