package transform

import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("github.com/pure-golang/budva/internal/service/transform")
