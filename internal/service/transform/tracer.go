package transform

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("github.com/pure-golang/budva-claude/internal/service/transform")
